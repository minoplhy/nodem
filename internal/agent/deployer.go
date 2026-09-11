package agent

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/minoplhy/nodem/internal/ech/transport"
)

// DeployLocally writes the latest ECH key material for updated clusters and triggers proxy reload.
func DeployLocally(cfg Config, updatedClusters []transport.ClusterSyncItem) error {
	for _, cluster := range updatedClusters {
		clusterDir := ClusterDirPath(cfg.StorageDir, cluster.ClusterID)
		if err := os.MkdirAll(clusterDir, 0700); err != nil {
			return fmt.Errorf("failed creating cluster dir for cluster %d (%s): %w", cluster.ClusterID, cluster.ClusterName, err)
		}

		currPath := filepath.Join(clusterDir, "ech_current.pem")
		prevPath := filepath.Join(clusterDir, "ech_previous.pem")

		// Backup current to previous
		if currData, err := os.ReadFile(currPath); err == nil {
			_ = os.WriteFile(prevPath, currData, 0600)
		}

		// Write new current key atomically
		tmpPath := currPath + ".tmp"
		if err := os.WriteFile(tmpPath, []byte(cluster.Keys.ECHCurrentPEM), 0600); err != nil {
			return fmt.Errorf("failed writing new key for cluster %d (%s): %w", cluster.ClusterID, cluster.ClusterName, err)
		}
		if err := os.Rename(tmpPath, currPath); err != nil {
			return fmt.Errorf("failed committing new key for cluster %d (%s): %w", cluster.ClusterID, cluster.ClusterName, err)
		}
	}

	return ReloadLocalProxies(cfg, updatedClusters)
}

// RemoveLocally deletes local cluster directories and executes hook remove triggers.
func RemoveLocally(cfg Config, clustersToRemove map[int64]string) error {
	for clusterID, clusterName := range clustersToRemove {
		clusterDir := ClusterDirPath(cfg.StorageDir, clusterID)
		if err := os.RemoveAll(clusterDir); err != nil {
			slog.Warn("Failed to remove cluster directory", "dir", clusterDir, "error", err)
		}
		if clusterName != "" {
			legacyDir := filepath.Join(cfg.StorageDir, clusterName)
			if legacyDir != clusterDir {
				_ = os.RemoveAll(legacyDir)
			}
		}

		if cfg.ProxyType == "hook" && cfg.HookScript != "" {
			cmd := exec.Command(cfg.HookScript, "remove", fmt.Sprintf("%d", clusterID))
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env,
				"ECH_ACTION=remove",
				fmt.Sprintf("ECH_CLUSTER_ID=%d", clusterID),
				fmt.Sprintf("ECH_CLUSTER_NAME=%s", clusterName),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				slog.Error("Hook script failed on cluster remove", "cluster_id", clusterID, "error", err, "output", string(out))
			}
		}
	}
	return nil
}
