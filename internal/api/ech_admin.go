package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func generateToken(prefix string) (rawToken string, hash string) {
	bytes := make([]byte, 24)
	_, _ = rand.Read(bytes)
	rawToken = prefix + "_" + hex.EncodeToString(bytes)
	hash = transport.HashAgentToken(rawToken)
	return rawToken, hash
}

// ListECHClusters handles GET /api/ech/clusters
func (s *AppState) ListECHClusters(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	clusters, err := s.Repo.ListECHClusters(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, clusters)
}

type CreateClusterRequest struct {
	Name                  string `json:"name"`
	PublicName            string `json:"public_name"`
	CipherSuite           string `json:"cipher_suite"`
	MaxNameLen           int    `json:"max_name_len"`
	RotationIntervalHours int    `json:"rotation_interval_hours"`
	AutoRotate            bool   `json:"auto_rotate"`
}

// CreateECHCluster handles POST /api/ech/clusters
func (s *AppState) CreateECHCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.CipherSuite = engine.NormalizeCipherSuite(req.CipherSuite)

	pubKey, privKey, err := engine.GenerateSigningKeyPair()
	if err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate cluster signing key: %v", err))
		return
	}

	cluster, err := s.Repo.CreateECHCluster(r.Context(), user.ID, req.Name, req.PublicName, req.CipherSuite,
		req.MaxNameLen, req.RotationIntervalHours, req.AutoRotate, pubKey, privKey)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, cluster)
}

// GetECHCluster handles GET /api/ech/clusters/{id}
func (s *AppState) GetECHCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	RespondJSON(w, http.StatusOK, cluster)
}

// UpdateECHCluster handles PUT /api/ech/clusters/{id}
func (s *AppState) UpdateECHCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.CipherSuite = engine.NormalizeCipherSuite(req.CipherSuite)

	cluster, err := s.Repo.UpdateECHCluster(r.Context(), user.ID, id, req.Name, req.PublicName,
		req.CipherSuite, req.MaxNameLen, req.RotationIntervalHours, req.AutoRotate)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, cluster)
}

// DeleteECHCluster handles DELETE /api/ech/clusters/{id}
func (s *AppState) DeleteECHCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	if err := s.Repo.DeleteECHCluster(r.Context(), user.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// TriggerClusterRotation handles POST /api/ech/clusters/{id}/rotate
func (s *AppState) TriggerClusterRotation(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, id)
	if err != nil || cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Generate new ECH Key using engine
	eng := engine.NewEngine("", "auto", s.OpenSSLPath)
	key, err := eng.GenerateECHKeyPair(r.Context(), cluster.PublicName, cluster.CipherSuite, cluster.MaxNameLen)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("ECH Key generation failed: %v", err))
		return
	}

	now := time.Now().UTC()
	next := now.Add(time.Duration(cluster.RotationIntervalHours) * time.Hour)
	newVersion, err := s.Repo.IncrementClusterVersion(r.Context(), cluster.ID, now, next)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	savedKey, err := s.Repo.SaveNewECHKey(r.Context(), cluster.ID, newVersion, key.Base64ECH, string(key.PrivateKeyPEM), string(key.ECHConfigPEM), string(key.PEMBytes))
	if err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save ECH key: %v", err))
		return
	}

	_, _ = s.Repo.AddECHLog(r.Context(), cluster.ID, nil, nil, "GENERATE", fmt.Sprintf("Generated new ECH key version %d", newVersion))

	RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"version": newVersion,
		"key_id":  savedKey.ID,
	})
}

type CreateNodeRequest struct {
	Name          string  `json:"name"`
	PullTransport string  `json:"pull_transport"` // "HTTPS" or "SSH"
	SSHPublicKey  *string `json:"ssh_public_key,omitempty"`
	ProxyType     string  `json:"proxy_type"`
	ClusterIDs    []int64 `json:"cluster_ids,omitempty"`
}

type UpdateNodeRequest struct {
	Name          string  `json:"name"`
	PullTransport string  `json:"pull_transport"`
	SSHPublicKey  *string `json:"ssh_public_key,omitempty"`
	ProxyType     string  `json:"proxy_type"`
}

type SetNodeClustersRequest struct {
	ClusterIDs []int64 `json:"cluster_ids"`
}

type AssignNodeRequest struct {
	NodeID int64 `json:"node_id"`
}

type CreateNodeResponse struct {
	Node            *db.ECHNode `json:"node"`
	AgentToken      string      `json:"agent_token,omitempty"`
	ServerPublicKey string      `json:"server_public_key"`
}

// ListAllECHNodes handles GET /api/ech/nodes
func (s *AppState) ListAllECHNodes(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	nodes, err := s.Repo.ListTenantECHNodes(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, nodes)
}

// CreateIndependentECHNode handles POST /api/ech/nodes
func (s *AppState) CreateIndependentECHNode(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var rawToken string
	var tokenHash *string
	if req.PullTransport == db.PullTransportHTTPS || req.SSHPublicKey == nil || *req.SSHPublicKey == "" {
		token, hash := generateToken("agt")
		rawToken = token
		tokenHash = &hash
	}

	node, err := s.Repo.CreateECHNode(r.Context(), user.ID, req.Name, req.PullTransport, tokenHash, req.SSHPublicKey, req.ProxyType)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.ClusterIDs) > 0 {
		_ = s.Repo.SetNodeClusters(r.Context(), node.ID, req.ClusterIDs)
	}

	// Fetch full node with cluster statuses
	allNodes, err := s.Repo.ListTenantECHNodes(r.Context(), user.ID)
	if err == nil {
		for _, n := range allNodes {
			if n.ID == node.ID {
				node = &n
				break
			}
		}
	}

	serverPubKey, _, _ := s.Repo.GetOrCreateServerSigningKey(r.Context())

	RespondJSON(w, http.StatusOK, CreateNodeResponse{
		Node:            node,
		AgentToken:      rawToken,
		ServerPublicKey: serverPubKey,
	})
}

// GetECHNodeDetail handles GET /api/ech/nodes/{id}
func (s *AppState) GetECHNodeDetail(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	node, err := s.Repo.GetECHNode(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil || node.TenantID != user.ID {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	RespondJSON(w, http.StatusOK, node)
}

// UpdateECHNode handles PUT /api/ech/nodes/{id}
func (s *AppState) UpdateECHNode(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	var req UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	node, err := s.Repo.UpdateECHNode(r.Context(), user.ID, id, req.Name, req.PullTransport, req.ProxyType, req.SSHPublicKey)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, node)
}

// DeleteECHNode handles DELETE /api/ech/nodes/{id}
func (s *AppState) DeleteECHNode(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	if err := s.Repo.DeleteECHNode(r.Context(), user.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// SetNodeClusters handles PUT /api/ech/nodes/{id}/clusters
func (s *AppState) SetNodeClusters(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	node, err := s.Repo.GetECHNode(r.Context(), id)
	if err != nil || node == nil || node.TenantID != user.ID {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	var req SetNodeClustersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := s.Repo.SetNodeClusters(r.Context(), id, req.ClusterIDs); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ListECHNodesForCluster handles GET /api/ech/clusters/{id}/nodes
func (s *AppState) ListECHNodesForCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, id)
	if err != nil || cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	nodes, err := s.Repo.ListClusterNodes(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, nodes)
}

// ListECHNodes aliases ListECHNodesForCluster for route compatibility
func (s *AppState) ListECHNodes(w http.ResponseWriter, r *http.Request) {
	s.ListECHNodesForCluster(w, r)
}

// CreateECHNode handles POST /api/ech/clusters/{id}/nodes (creates node & assigns to cluster)
func (s *AppState) CreateECHNode(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, id)
	if err != nil || cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var rawToken string
	var tokenHash *string
	if req.PullTransport == db.PullTransportHTTPS || req.SSHPublicKey == nil || *req.SSHPublicKey == "" {
		token, hash := generateToken("agt")
		rawToken = token
		tokenHash = &hash
	}

	node, err := s.Repo.CreateECHNode(r.Context(), user.ID, req.Name, req.PullTransport, tokenHash, req.SSHPublicKey, req.ProxyType)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.Repo.AssignNodeToCluster(r.Context(), id, node.ID)

	serverPubKey, _, _ := s.Repo.GetOrCreateServerSigningKey(r.Context())

	RespondJSON(w, http.StatusOK, CreateNodeResponse{
		Node:            node,
		AgentToken:      rawToken,
		ServerPublicKey: serverPubKey,
	})
}

// AssignNodeToCluster handles POST /api/ech/clusters/{id}/nodes/assign
func (s *AppState) AssignNodeToCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, id)
	if err != nil || cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	var req AssignNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	node, err := s.Repo.GetECHNode(r.Context(), req.NodeID)
	if err != nil || node == nil || node.TenantID != user.ID {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	if err := s.Repo.AssignNodeToCluster(r.Context(), id, req.NodeID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// UnassignNodeFromCluster handles DELETE /api/ech/clusters/{id}/nodes/{node_id}
func (s *AppState) UnassignNodeFromCluster(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	clusterID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}
	nodeID, err := strconv.ParseInt(chi.URLParam(r, "node_id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	cluster, err := s.Repo.GetECHCluster(r.Context(), user.ID, clusterID)
	if err != nil || cluster == nil {
		RespondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	if err := s.Repo.UnassignNodeFromCluster(r.Context(), clusterID, nodeID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type CreateDomainRequest struct {
	DNSProviderID int64   `json:"dns_provider_id"`
	TargetGroupID *int64  `json:"target_group_id,omitempty"`
	Domain        string  `json:"domain"`
	TTL           int     `json:"ttl"`
	ALPN          string  `json:"alpn"`
	IPv4Hint      *string `json:"ipv4_hint,omitempty"`
	IPv6Hint      *string `json:"ipv6_hint,omitempty"`
}

// ListECHDomains handles GET /api/ech/clusters/{id}/domains
func (s *AppState) ListECHDomains(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	domains, err := s.Repo.ListECHDomains(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, domains)
}

// CreateECHDomain handles POST /api/ech/clusters/{id}/domains
func (s *AppState) CreateECHDomain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	dom, err := s.Repo.CreateECHDomain(r.Context(), id, req.DNSProviderID, req.TargetGroupID,
		req.Domain, req.TTL, req.ALPN, req.IPv4Hint, req.IPv6Hint)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, dom)
}

// DeleteECHDomain handles DELETE /api/ech/domains/{id}
func (s *AppState) DeleteECHDomain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid domain ID")
		return
	}

	if err := s.Repo.DeleteECHDomain(r.Context(), id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ListECHLogs handles GET /api/ech/clusters/{id}/logs
func (s *AppState) ListECHLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid cluster ID")
		return
	}

	logs, err := s.Repo.ListRecentECHLogs(r.Context(), id, 50)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, logs)
}

type ServerPublicKeyResponse struct {
	ServerPublicKey string `json:"server_public_key"`
	Algorithm       string `json:"algorithm"`
}

// GetServerPublicKey handles GET /api/ech/server-key
func (s *AppState) GetServerPublicKey(w http.ResponseWriter, r *http.Request) {
	serverPubKey, _, err := s.Repo.GetOrCreateServerSigningKey(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get server public key: %v", err))
		return
	}

	RespondJSON(w, http.StatusOK, ServerPublicKeyResponse{
		ServerPublicKey: serverPubKey,
		Algorithm:       "ed25519",
	})
}
