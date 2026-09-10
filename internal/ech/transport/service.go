package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

const (
	PayloadTypeSyncKeyUpdate = "SYNC_KEY_UPDATE"
	PayloadTypeSyncPayload   = "SYNC_PAYLOAD"
)

// BuildSignableMessage creates a canonical domain-separated message digest string.
func BuildSignableMessage(payloadType string, clusterID, version int64, checksum string) []byte {
	return []byte(fmt.Sprintf("%s:%d:%d:%s", payloadType, clusterID, version, checksum))
}

type SyncKeysPayload struct {
	Base64ECH      string `json:"base64_ech"`
	ECHCurrentPEM  string `json:"ech_current_pem"`
	ECHPreviousPEM string `json:"ech_previous_pem,omitempty"`
	PrivateKeyPEM  string `json:"private_key_pem"`
	ECHConfigPEM   string `json:"ech_config_pem"`
}

// Checksum computes a deterministic SHA-256 hex digest over the cryptographic payload material.
func (p *SyncKeysPayload) Checksum() string {
	if p == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(p.Base64ECH))
	h.Write([]byte("\n"))
	h.Write([]byte(p.ECHCurrentPEM))
	h.Write([]byte("\n"))
	h.Write([]byte(p.ECHPreviousPEM))
	h.Write([]byte("\n"))
	h.Write([]byte(p.PrivateKeyPEM))
	h.Write([]byte("\n"))
	h.Write([]byte(p.ECHConfigPEM))
	return hex.EncodeToString(h.Sum(nil))
}

type SyncSignature struct {
	Type      string `json:"type"`       // Payload type, e.g. "SYNC_KEY_UPDATE"
	Algorithm string `json:"algorithm"`  // "ed25519"
	Checksum  string `json:"checksum"`   // SHA-256 hex digest of SyncKeysPayload
	SigBase64 string `json:"sig_base64"` // Ed25519 signature of BuildSignableMessage
	PublicKey string `json:"public_key,omitempty"`
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
	NodeID            int64             `json:"node_id,omitempty"`
	Status            string            `json:"status"` // "UP_TO_DATE" or "UPDATES_AVAILABLE"
	Clusters          []ClusterSyncItem `json:"clusters"`
	RemovedClusterIDs []int64           `json:"removed_cluster_ids,omitempty"`
	RemovedClusters   []string          `json:"removed_clusters,omitempty"`
	Timestamp         int64             `json:"timestamp"`
	Message           string            `json:"message,omitempty"`
	Signature         *SyncSignature    `json:"signature,omitempty"`
}

// PayloadChecksum computes a deterministic SHA-256 hex digest over the entire sync payload content.
func (r *SyncResponse) PayloadChecksum() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("NODE_ID:%d\n", r.NodeID))
	b.WriteString(fmt.Sprintf("STATUS:%s\n", r.Status))
	b.WriteString(fmt.Sprintf("TIMESTAMP:%d\n", r.Timestamp))

	// Sort clusters canonically by ClusterID ascending, then ClusterName ascending
	sortedClusters := make([]ClusterSyncItem, len(r.Clusters))
	copy(sortedClusters, r.Clusters)
	sort.Slice(sortedClusters, func(i, j int) bool {
		if sortedClusters[i].ClusterID != sortedClusters[j].ClusterID {
			return sortedClusters[i].ClusterID < sortedClusters[j].ClusterID
		}
		return sortedClusters[i].ClusterName < sortedClusters[j].ClusterName
	})

	for _, c := range sortedClusters {
		b.WriteString(fmt.Sprintf("CLUSTER:%d:%s:%s:%s:%d\n", c.ClusterID, c.ClusterName, c.PublicName, c.Status, c.Version))
		if c.Keys != nil {
			b.WriteString("KEY_ECH:" + c.Keys.Base64ECH + "\n")
			b.WriteString("KEY_CURRENT_PEM:" + c.Keys.ECHCurrentPEM + "\n")
			b.WriteString("KEY_PRIVATE_PEM:" + c.Keys.PrivateKeyPEM + "\n")
			b.WriteString("KEY_CONFIG_PEM:" + c.Keys.ECHConfigPEM + "\n")
			b.WriteString("KEY_PREVIOUS_PEM:" + c.Keys.ECHPreviousPEM + "\n")
		}
	}

	// Sort removed cluster IDs ascending
	sortedRemovedIDs := make([]int64, len(r.RemovedClusterIDs))
	copy(sortedRemovedIDs, r.RemovedClusterIDs)
	sort.Slice(sortedRemovedIDs, func(i, j int) bool {
		return sortedRemovedIDs[i] < sortedRemovedIDs[j]
	})
	for _, id := range sortedRemovedIDs {
		b.WriteString(fmt.Sprintf("REMOVED_ID:%d\n", id))
	}

	// Sort removed clusters alphabetically
	sortedRemovedNames := make([]string, len(r.RemovedClusters))
	copy(sortedRemovedNames, r.RemovedClusters)
	sort.Strings(sortedRemovedNames)
	for _, name := range sortedRemovedNames {
		b.WriteString(fmt.Sprintf("REMOVED_NAME:%s\n", name))
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
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

	serverPubKey, serverPrivKey, err := s.repo.GetOrCreateServerSigningKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed retrieving server signing key: %w", err)
	}

	var items []ClusterSyncItem
	var msg string
	updatesAvailable := len(removedClusterIDs) > 0 || len(removedClusters) > 0

	if len(clusters) == 0 {
		items = []ClusterSyncItem{}
		msg = "No clusters assigned to this node"
	} else {
		items = make([]ClusterSyncItem, 0, len(clusters))
		msg = fmt.Sprintf("Synchronized %d clusters", len(clusters))

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

			checksum := keysPayload.Checksum()
			signedMsg := BuildSignableMessage(PayloadTypeSyncKeyUpdate, cluster.ID, cluster.CurrentVersion, checksum)
			sigBase64, err := engine.SignPayload(serverPrivKey, signedMsg)
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
					Type:      PayloadTypeSyncKeyUpdate,
					Algorithm: "ed25519",
					Checksum:  checksum,
					SigBase64: sigBase64,
					PublicKey: serverPubKey,
				},
			})
			updatesAvailable = true
		}
	}

	overallStatus := "UP_TO_DATE"
	if updatesAvailable {
		overallStatus = "UPDATES_AVAILABLE"
	}

	resp := &SyncResponse{
		NodeID:            node.ID,
		Status:            overallStatus,
		Clusters:          items,
		RemovedClusterIDs: removedClusterIDs,
		RemovedClusters:   removedClusters,
		Timestamp:         time.Now().UTC().Unix(),
		Message:           msg,
	}

	checksum := resp.PayloadChecksum()
	signedMsg := BuildSignableMessage(PayloadTypeSyncPayload, resp.NodeID, resp.Timestamp, checksum)
	sigBase64, err := engine.SignPayload(serverPrivKey, signedMsg)
	if err != nil {
		return nil, fmt.Errorf("failed signing sync response: %w", err)
	}

	resp.Signature = &SyncSignature{
		Type:      PayloadTypeSyncPayload,
		Algorithm: "ed25519",
		Checksum:  checksum,
		SigBase64: sigBase64,
		PublicKey: serverPubKey,
	}

	return resp, nil
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
