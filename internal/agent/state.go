package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minoplhy/nodem/internal/ech/transport"
)

// ClusterState holds the local applied state for a single cluster.
type ClusterState struct {
	ClusterID      int64     `json:"cluster_id,omitempty"`
	ClusterName    string    `json:"cluster_name,omitempty"`
	AppliedVersion int64     `json:"applied_version"`
	LastSyncTime   time.Time `json:"last_sync_time"`
}

// State represents the persisted local agent state across clusters.
type State struct {
	Clusters          map[string]ClusterState `json:"clusters"`
	LastSyncTimestamp int64                   `json:"last_sync_timestamp,omitempty"`
}

// ClusterDirName returns the standardized directory name for a cluster.
func ClusterDirName(clusterID int64) string {
	return fmt.Sprintf("cluster_%d", clusterID)
}

// ClusterDirPath returns the path to a cluster's directory inside storageDir.
func ClusterDirPath(storageDir string, clusterID int64) string {
	return filepath.Join(storageDir, ClusterDirName(clusterID))
}

// ParseClusterIDFromDir extracts the integer cluster ID from a directory name formatted as "cluster_<id>".
func ParseClusterIDFromDir(dirName string) (int64, bool) {
	if strings.HasPrefix(dirName, "cluster_") {
		idStr := strings.TrimPrefix(dirName, "cluster_")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			return id, true
		}
	}
	return 0, false
}

// LoadState reads agent state from stateFile or returns a fresh State if absent.
func LoadState(stateFile string) State {
	st := State{Clusters: make(map[string]ClusterState)}
	if data, err := os.ReadFile(stateFile); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	if st.Clusters == nil {
		st.Clusters = make(map[string]ClusterState)
	}
	for k, cs := range st.Clusters {
		if cs.ClusterID == 0 {
			if id, err := strconv.ParseInt(k, 10, 64); err == nil {
				cs.ClusterID = id
				st.Clusters[k] = cs
			}
		}
	}
	return st
}

// SaveState writes agent state to stateFile formatted as indented JSON.
func SaveState(stateFile string, st State) error {
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

// MigrateLegacyClusterFolders migrates older cluster directories named after clusterName to cluster_<id>.
func MigrateLegacyClusterFolders(storageDir string, st *State, clusters []transport.ClusterSyncItem) bool {
	migrated := false
	for _, c := range clusters {
		if c.ClusterID <= 0 {
			continue
		}
		idStr := fmt.Sprintf("%d", c.ClusterID)
		targetDir := ClusterDirPath(storageDir, c.ClusterID)
		legacyDir := filepath.Join(storageDir, c.ClusterName)

		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			if fi, err := os.Stat(legacyDir); err == nil && fi.IsDir() {
				slog.Info("Migrating legacy cluster folder to cluster_<id> format", "legacy", legacyDir, "target", targetDir)
				_ = os.Rename(legacyDir, targetDir)
				migrated = true
			}
		}

		if legacyState, exists := st.Clusters[c.ClusterName]; exists {
			if _, hasIDKey := st.Clusters[idStr]; !hasIDKey {
				legacyState.ClusterID = c.ClusterID
				legacyState.ClusterName = c.ClusterName
				st.Clusters[idStr] = legacyState
			}
			delete(st.Clusters, c.ClusterName)
			migrated = true
		}
	}
	return migrated
}
