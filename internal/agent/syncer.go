package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

// RunSyncCycle orchestrates a full pull, verify, deploy, and ack cycle.
func RunSyncCycle(ctx context.Context, cfg Config) error {
	stateFile := filepath.Join(cfg.StorageDir, "agent_state.json")
	localState := LoadState(stateFile)

	versionMap := make(map[string]int64)
	for key, cs := range localState.Clusters {
		if cs.ClusterID > 0 {
			versionMap[fmt.Sprintf("%d", cs.ClusterID)] = cs.AppliedVersion
		} else {
			versionMap[key] = cs.AppliedVersion
		}
	}

	slog.Info("Polling central server for ECH keys across assigned clusters...", "managed_clusters", len(versionMap))

	var resp *transport.SyncResponse
	var err error

	if cfg.Transport == "ssh" {
		resp, err = PullViaSSH(ctx, cfg, versionMap)
	} else {
		resp, err = PullViaHTTPS(ctx, cfg, versionMap)
	}

	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	// 1. Enforce pinned server public key presence
	if cfg.ServerPublicKey == "" {
		return fmt.Errorf("missing required parameter: --server-public-key (or ECH_SERVER_PUBLIC_KEY)")
	}

	// 2. Full sync response payload signature & checksum verification
	if resp.Signature == nil || resp.Signature.SigBase64 == "" || resp.Signature.Checksum == "" {
		return fmt.Errorf("sync response signature missing")
	}

	computedPayloadChecksum := resp.PayloadChecksum()
	if computedPayloadChecksum != resp.Signature.Checksum {
		return fmt.Errorf("sync response payload checksum mismatch")
	}

	signedPayloadMsg := transport.BuildSignableMessage(transport.PayloadTypeSyncPayload, resp.NodeID, resp.Timestamp, resp.Signature.Checksum)
	if !engine.VerifySignature(cfg.ServerPublicKey, signedPayloadMsg, resp.Signature.SigBase64) {
		return fmt.Errorf("sync response signature verification failed")
	}

	// 3. Stale or replayed response detection
	if resp.Timestamp > 0 && localState.LastSyncTimestamp > 0 && resp.Timestamp < localState.LastSyncTimestamp-30 {
		return fmt.Errorf("stale or replayed sync response detected (server timestamp %d < local %d)", resp.Timestamp, localState.LastSyncTimestamp)
	}

	slog.Info("Sync response signature and payload verified", "node_id", resp.NodeID, "timestamp", resp.Timestamp, "clusters", len(resp.Clusters))

	// Migrate any legacy name-based directories or state entries
	if MigrateLegacyClusterFolders(cfg.StorageDir, &localState, resp.Clusters) {
		_ = SaveState(stateFile, localState)
		_ = GenerateMasterIncludesConf(cfg.StorageDir)
	}

	// Build map of assigned clusters from server response
	assignedIDs := make(map[int64]bool)
	assignedNames := make(map[string]bool)
	for _, c := range resp.Clusters {
		assignedIDs[c.ClusterID] = true
		assignedNames[c.ClusterName] = true
	}

	// Identify clusters to remove
	clustersToRemoveMap := make(map[int64]string)

	for _, id := range resp.RemovedClusterIDs {
		clustersToRemoveMap[id] = fmt.Sprintf("Cluster %d", id)
	}
	for _, name := range resp.RemovedClusters {
		for _, cs := range localState.Clusters {
			if cs.ClusterName == name && cs.ClusterID > 0 {
				clustersToRemoveMap[cs.ClusterID] = cs.ClusterName
			}
		}
	}

	// Local state reconciliation (any local cluster not in server's assigned clusters)
	for key, cs := range localState.Clusters {
		cid := cs.ClusterID
		if cid == 0 {
			if parsed, err := strconv.ParseInt(key, 10, 64); err == nil {
				cid = parsed
			}
		}
		if cid > 0 && !assignedIDs[cid] {
			cname := cs.ClusterName
			if cname == "" {
				cname = fmt.Sprintf("Cluster %d", cid)
			}
			clustersToRemoveMap[cid] = cname
		} else if cid == 0 && !assignedNames[key] {
			delete(localState.Clusters, key)
		}
	}

	// Filter clusters that have new key updates
	var newClusters []transport.ClusterSyncItem
	for _, c := range resp.Clusters {
		if c.Status == "NEW_KEY" && c.Keys != nil {
			if cfg.ServerPublicKey == "" {
				return fmt.Errorf("missing required parameter: --server-public-key (or ECH_SERVER_PUBLIC_KEY)")
			}
			if c.Signature == nil || c.Signature.Checksum == "" || c.Signature.SigBase64 == "" {
				return fmt.Errorf("payload signature missing for cluster %d (%s)", c.ClusterID, c.ClusterName)
			}

			computedChecksum := c.Keys.Checksum()
			if computedChecksum != c.Signature.Checksum {
				return fmt.Errorf("payload checksum mismatch for cluster %d (%s)", c.ClusterID, c.ClusterName)
			}

			payloadType := c.Signature.Type
			if payloadType == "" {
				payloadType = transport.PayloadTypeSyncKeyUpdate
			}
			signedMessage := transport.BuildSignableMessage(payloadType, c.ClusterID, c.Version, c.Signature.Checksum)

			if !engine.VerifySignature(cfg.ServerPublicKey, signedMessage, c.Signature.SigBase64) {
				return fmt.Errorf("payload signature verification failed for cluster %d (%s)", c.ClusterID, c.ClusterName)
			}

			slog.Info("Payload signature and checksum verified", "cluster_id", c.ClusterID, "cluster", c.ClusterName, "type", payloadType)
			newClusters = append(newClusters, c)
		}
	}

	if len(newClusters) == 0 && len(clustersToRemoveMap) == 0 {
		slog.Info("All assigned ECH clusters are up to date.")
		if resp.Timestamp > localState.LastSyncTimestamp {
			localState.LastSyncTimestamp = resp.Timestamp
			_ = SaveState(stateFile, localState)
		}
		if cfg.ProxyType == "nginx" {
			includesPath := filepath.Join(cfg.StorageDir, "ech_includes.conf")
			if _, err := os.Stat(includesPath); os.IsNotExist(err) {
				_ = GenerateMasterIncludesConf(cfg.StorageDir)
			}
		}
		return nil
	}

	if len(clustersToRemoveMap) > 0 {
		slog.Info("Decommissioning unassigned/deleted clusters", "count", len(clustersToRemoveMap))
	}
	if len(newClusters) > 0 {
		slog.Info("Received new ECH keys for clusters", "count", len(newClusters))
	}

	if cfg.DryRun {
		for cid, cname := range clustersToRemoveMap {
			slog.Info("[DRY-RUN] Would remove cluster directory and keys", "cluster_id", cid, "cluster", cname, "dir", ClusterDirName(cid))
		}
		for _, c := range newClusters {
			slog.Info("[DRY-RUN] Would stage key files for cluster", "cluster_id", c.ClusterID, "cluster", c.ClusterName, "version", c.Version)
		}
		return nil
	}

	// Remove unassigned clusters locally first
	removeErr := RemoveLocally(cfg, clustersToRemoveMap)
	if removeErr != nil {
		slog.Error("Local cluster removal encountered errors", "error", removeErr)
	}

	// Deploy updated clusters locally
	var deployErr error
	if len(newClusters) > 0 {
		deployErr = DeployLocally(cfg, newClusters)
		if deployErr != nil {
			slog.Error("Local proxy deployment failed", "error", deployErr)
		}
	}

	// Build ACKs
	var acks []transport.ClusterAckItem
	for _, c := range newClusters {
		status := "SUCCESS"
		msg := "Deployed and reloaded successfully"
		if deployErr != nil {
			status = "FAILED"
			msg = deployErr.Error()
		}
		acks = append(acks, transport.ClusterAckItem{
			ClusterID:      c.ClusterID,
			ClusterName:    c.ClusterName,
			AppliedVersion: c.Version,
			Status:         status,
			Message:        msg,
		})
	}
	for cid, cname := range clustersToRemoveMap {
		status := "REMOVED"
		msg := "Cluster decommissioned successfully"
		if removeErr != nil {
			status = "FAILED"
			msg = removeErr.Error()
		}
		acks = append(acks, transport.ClusterAckItem{
			ClusterID:      cid,
			ClusterName:    cname,
			AppliedVersion: 0,
			Status:         status,
			Message:        msg,
		})
	}

	// Send ACK back to server
	if len(acks) > 0 {
		if cfg.Transport == "ssh" {
			_ = AckViaSSH(ctx, cfg, acks)
		} else {
			_ = AckViaHTTPS(ctx, cfg, acks)
		}
	}

	if deployErr != nil {
		return deployErr
	}
	if removeErr != nil {
		return removeErr
	}

	// Update local state: remove deleted clusters
	for cid := range clustersToRemoveMap {
		delete(localState.Clusters, fmt.Sprintf("%d", cid))
	}

	// Update local state: add/update deployed clusters
	now := time.Now().UTC()
	for _, c := range newClusters {
		localState.Clusters[fmt.Sprintf("%d", c.ClusterID)] = ClusterState{
			ClusterID:      c.ClusterID,
			ClusterName:    c.ClusterName,
			AppliedVersion: c.Version,
			LastSyncTime:   now,
		}
	}
	for _, c := range resp.Clusters {
		idKey := fmt.Sprintf("%d", c.ClusterID)
		if existing, ok := localState.Clusters[idKey]; ok {
			existing.ClusterID = c.ClusterID
			existing.ClusterName = c.ClusterName
			localState.Clusters[idKey] = existing
		} else if c.Status == "UP_TO_DATE" {
			localState.Clusters[idKey] = ClusterState{
				ClusterID:      c.ClusterID,
				ClusterName:    c.ClusterName,
				AppliedVersion: c.Version,
				LastSyncTime:   now,
			}
		}
	}
	if resp.Timestamp > localState.LastSyncTimestamp {
		localState.LastSyncTimestamp = resp.Timestamp
	}
	_ = SaveState(stateFile, localState)

	// If any clusters were removed and no new clusters deployed, ensure proxy is reloaded once
	if len(newClusters) == 0 && len(clustersToRemoveMap) > 0 {
		if err := ReloadLocalProxies(cfg, nil); err != nil {
			slog.Error("Proxy reload after removal failed", "error", err)
			return err
		}
	}

	slog.Info("ECH sync cycle completed successfully", "deployed", len(newClusters), "removed", len(clustersToRemoveMap))
	return nil
}
