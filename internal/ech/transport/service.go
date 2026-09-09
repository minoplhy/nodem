package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/ech/engine"
)

var (
	ErrUnauthorized      = errors.New("unauthorized: invalid agent credentials")
	ErrTransportMismatch = errors.New("transport mismatch: node is not configured for this transport")
	ErrClusterNotFound   = errors.New("cluster not found")
)

// HashAgentToken computes the SHA-256 hex digest of an agent token.
func HashAgentToken(token string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(h[:])
}

type SyncRequest struct {
	ProxyType string           `json:"proxy_type,omitempty"`
	Version   string           `json:"version,omitempty"`
	Clusters  map[string]int64 `json:"clusters,omitempty"` // map of cluster_name -> local applied version
}

type SyncKeysPayload struct {
	Base64ECH      string `json:"base64_ech"`
	ECHCurrentPEM  string `json:"ech_current_pem"`
	ECHPreviousPEM string `json:"ech_previous_pem,omitempty"`
	PrivateKeyPEM  string `json:"private_key_pem"`
	ECHConfigPEM   string `json:"ech_config_pem"`
}

type SyncSignature struct {
	Algorithm string `json:"algorithm"` // "ed25519"
	SigBase64 string `json:"sig_base64"`
	PublicKey string `json:"public_key"`
}

type ClusterSyncItem struct {
	ClusterID   int64            `json:"cluster_id"`
	ClusterName string           `json:"cluster_name"`
	PublicName  string           `json:"public_name"`
	Status      string           `json:"status"` // "UP_TO_DATE" or "NEW_KEY"
	Version     int64            `json:"version"`
	Keys        *SyncKeysPayload `json:"keys,omitempty"`
	Signature   *SyncSignature   `json:"signature,omitempty"`
}

type SyncResponse struct {
	Status            string            `json:"status"` // "UP_TO_DATE" or "UPDATES_AVAILABLE"
	Clusters          []ClusterSyncItem `json:"clusters"`
	RemovedClusterIDs []int64           `json:"removed_cluster_ids,omitempty"`
	RemovedClusters   []string          `json:"removed_clusters,omitempty"`
	Message           string            `json:"message,omitempty"`
}

type ClusterAckItem struct {
	ClusterID      int64  `json:"cluster_id"`
	ClusterName    string `json:"cluster_name,omitempty"`
	AppliedVersion int64  `json:"applied_version"`
	Status         string `json:"status"` // "SUCCESS" or "FAILED"
	Message        string `json:"message,omitempty"`
}

type AckRequest struct {
	Acks []ClusterAckItem `json:"acks"`
}

// PullService orchestrates pulling key bundles and processing acknowledgments.
type PullService struct {
	repo db.Repository
}

// NewPullService creates a new PullService.
func NewPullService(repo db.Repository) *PullService {
	return &PullService{repo: repo}
}

// AuthenticateByToken looks up a node by token and verifies caller transport.
func (s *PullService) AuthenticateByToken(ctx context.Context, token, callerTransport string) (*db.ECHNode, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	hash := HashAgentToken(token)
	node, err := s.repo.GetECHNodeByTokenHash(ctx, hash)
	if err != nil || node == nil {
		return nil, ErrUnauthorized
	}

	if node.PullTransport != callerTransport {
		return nil, fmt.Errorf("%w: node is configured for %s, requested via %s", ErrTransportMismatch, node.PullTransport, callerTransport)
	}

	return node, nil
}

// AuthenticateBySSHPublicKey looks up a node by SSH public key and verifies caller transport.
func (s *PullService) AuthenticateBySSHPublicKey(ctx context.Context, sshPubKey, callerTransport string) (*db.ECHNode, error) {
	if sshPubKey == "" {
		return nil, ErrUnauthorized
	}
	node, err := s.repo.GetECHNodeBySSHPublicKey(ctx, sshPubKey)
	if err != nil || node == nil {
		return nil, ErrUnauthorized
	}

	if node.PullTransport != callerTransport {
		return nil, fmt.Errorf("%w: node is configured for %s, requested via %s", ErrTransportMismatch, node.PullTransport, callerTransport)
	}

	return node, nil
}

// SyncNode processes a node pull sync request across all assigned clusters.
func (s *PullService) SyncNode(ctx context.Context, node *db.ECHNode, req SyncRequest, callerIP string) (*SyncResponse, error) {
	_ = s.repo.UpdateECHNodeLastSeen(ctx, node.ID, callerIP)

	clusters, err := s.repo.ListNodeClusters(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed fetching node clusters: %w", err)
	}

	assignedIDMap := make(map[int64]bool)
	assignedNameMap := make(map[string]bool)
	for _, c := range clusters {
		assignedIDMap[c.ID] = true
		assignedNameMap[c.Name] = true
	}

	var removedClusterIDs []int64
	var removedClusters []string
	for key := range req.Clusters {
		if id, err := strconv.ParseInt(key, 10, 64); err == nil {
			if !assignedIDMap[id] {
				removedClusterIDs = append(removedClusterIDs, id)
			}
		} else {
			if !assignedNameMap[key] {
				removedClusters = append(removedClusters, key)
			}
		}
	}

	if len(clusters) == 0 {
		status := "UP_TO_DATE"
		if len(removedClusterIDs) > 0 || len(removedClusters) > 0 {
			status = "UPDATES_AVAILABLE"
		}
		return &SyncResponse{
			Status:            status,
			Clusters:          []ClusterSyncItem{},
			RemovedClusterIDs: removedClusterIDs,
			RemovedClusters:   removedClusters,
			Message:           "No clusters assigned to this node",
		}, nil
	}

	items := make([]ClusterSyncItem, 0, len(clusters))
	updatesAvailable := len(removedClusterIDs) > 0 || len(removedClusters) > 0

	for _, cluster := range clusters {
		localVersion, hasVersion := req.Clusters[fmt.Sprintf("%d", cluster.ID)]
		if !hasVersion {
			localVersion = req.Clusters[cluster.Name]
		}

		if localVersion == cluster.CurrentVersion && cluster.CurrentVersion > 0 {
			items = append(items, ClusterSyncItem{
				ClusterID:   cluster.ID,
				ClusterName: cluster.Name,
				PublicName:  cluster.PublicName,
				Status:      "UP_TO_DATE",
				Version:     cluster.CurrentVersion,
			})
			continue
		}

		activeKey, err := s.repo.GetActiveECHKey(ctx, cluster.ID)
		if err != nil || activeKey == nil {
			// No active key generated yet for this cluster
			items = append(items, ClusterSyncItem{
				ClusterID:   cluster.ID,
				ClusterName: cluster.Name,
				PublicName:  cluster.PublicName,
				Status:      "UP_TO_DATE",
				Version:     0,
			})
			continue
		}

		prevKey, _ := s.repo.GetPreviousECHKey(ctx, cluster.ID)

		keysPayload := &SyncKeysPayload{
			Base64ECH:     activeKey.Base64ECH,
			ECHCurrentPEM: activeKey.FullPEM,
			PrivateKeyPEM: activeKey.PrivateKeyPEM,
			ECHConfigPEM:  activeKey.ECHConfigPEM,
		}
		if prevKey != nil {
			keysPayload.ECHPreviousPEM = prevKey.FullPEM
		}

		payloadBytes, err := json.Marshal(keysPayload)
		if err != nil {
			return nil, fmt.Errorf("failed marshaling keys payload for cluster %s: %w", cluster.Name, err)
		}

		sigBase64, err := engine.SignPayload(cluster.SigningPrivateKey, payloadBytes)
		if err != nil {
			return nil, fmt.Errorf("failed signing keys payload for cluster %s: %w", cluster.Name, err)
		}

		items = append(items, ClusterSyncItem{
			ClusterID:   cluster.ID,
			ClusterName: cluster.Name,
			PublicName:  cluster.PublicName,
			Status:      "NEW_KEY",
			Version:     cluster.CurrentVersion,
			Keys:        keysPayload,
			Signature: &SyncSignature{
				Algorithm: "ed25519",
				SigBase64: sigBase64,
				PublicKey: cluster.SigningPublicKey,
			},
		})
		updatesAvailable = true
	}

	overallStatus := "UP_TO_DATE"
	if updatesAvailable {
		overallStatus = "UPDATES_AVAILABLE"
	}

	return &SyncResponse{
		Status:            overallStatus,
		Clusters:          items,
		RemovedClusterIDs: removedClusterIDs,
		RemovedClusters:   removedClusters,
		Message:           fmt.Sprintf("Synchronized %d clusters", len(items)),
	}, nil
}

// AckNode processes multi-cluster acknowledgment reports from an edge node.
func (s *PullService) AckNode(ctx context.Context, node *db.ECHNode, req AckRequest, callerIP string) error {
	assignedClusters, err := s.repo.ListNodeClusters(ctx, node.ID)
	if err != nil {
		return fmt.Errorf("failed retrieving assigned clusters: %w", err)
	}
	clusterMap := make(map[string]int64)
	for _, c := range assignedClusters {
		clusterMap[c.Name] = c.ID
	}

	for _, ack := range req.Acks {
		clusterID := ack.ClusterID
		if clusterID <= 0 && ack.ClusterName != "" {
			clusterID = clusterMap[ack.ClusterName]
		}
		if clusterID <= 0 && len(assignedClusters) == 1 {
			clusterID = assignedClusters[0].ID
		}

		if strings.EqualFold(ack.Status, "REMOVED") {
			msg := fmt.Sprintf("Node '%s' removed cluster %d (%s)", node.Name, clusterID, ack.ClusterName)
			_, _ = s.repo.AddECHLog(ctx, clusterID, &node.ID, nil, "DECOMMISSION", msg)
			continue
		}

		if clusterID <= 0 {
			continue
		}

		var errStr *string
		if ack.Message != "" && ack.Status != "SUCCESS" {
			errStr = &ack.Message
		}

		status := db.SyncStatusInSync
		if strings.ToUpper(ack.Status) != "SUCCESS" {
			status = db.SyncStatusFailed
		}

		if err := s.repo.RecordClusterNodeAck(ctx, clusterID, node.ID, ack.AppliedVersion, status, callerIP, errStr); err != nil {
			return err
		}

		logType := "ACK"
		msg := fmt.Sprintf("Node '%s' applied version %d for cluster %s (status: %s)", node.Name, ack.AppliedVersion, ack.ClusterName, status)
		if status == db.SyncStatusFailed {
			logType = "ERROR"
			msg = fmt.Sprintf("Node '%s' failed applying version %d for cluster %s: %s", node.Name, ack.AppliedVersion, ack.ClusterName, ack.Message)
		}

		_, _ = s.repo.AddECHLog(ctx, clusterID, &node.ID, nil, logType, msg)
	}

	return nil
}
