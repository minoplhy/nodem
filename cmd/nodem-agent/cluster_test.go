package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minoplhy/nodem/internal/agent"
)

func TestAgentCLIClusterCommands(t *testing.T) {
	storageDir := t.TempDir()

	// Pre-create cluster_5 directory and state
	clusterDir := filepath.Join(storageDir, "cluster_5")
	_ = os.MkdirAll(clusterDir, 0700)
	_ = os.WriteFile(filepath.Join(clusterDir, "ech_current.pem"), []byte("KEY_DATA"), 0600)

	stateFile := filepath.Join(storageDir, "agent_state.json")
	initialState := agent.State{
		Clusters: map[string]agent.ClusterState{
			"5": {
				ClusterID:      5,
				ClusterName:    "CLI-Cluster",
				AppliedVersion: 1,
			},
		},
	}
	_ = agent.SaveState(stateFile, initialState)

	// 1. Test cluster list
	if err := handleClusterCommand([]string{"cluster", "list", "--storage-dir", storageDir}); err != nil {
		t.Errorf("cluster list failed: %v", err)
	}

	// 2. Test cluster remove dry-run
	if err := handleClusterCommand([]string{"cluster", "remove", "5", "--storage-dir", storageDir, "--reload-cmd", "true", "--yes", "--dry-run"}); err != nil {
		t.Fatalf("cluster remove --dry-run failed: %v", err)
	}
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		t.Errorf("cluster_5 directory was removed during dry-run")
	}

	// 3. Test cluster remove
	if err := handleClusterCommand([]string{"cluster", "remove", "5", "--storage-dir", storageDir, "--reload-cmd", "true", "--yes"}); err != nil {
		t.Fatalf("cluster remove failed: %v", err)
	}

	// Verify directory deleted
	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		t.Errorf("cluster_5 directory was not removed")
	}

	// Verify state updated
	st := agent.LoadState(stateFile)
	if _, has5 := st.Clusters["5"]; has5 {
		t.Errorf("cluster 5 still present in state after remove")
	}

	// 4. Test cluster prune with orphan directory
	orphanDir := filepath.Join(storageDir, "cluster_99")
	_ = os.MkdirAll(orphanDir, 0700)
	_ = os.WriteFile(filepath.Join(orphanDir, "ech_current.pem"), []byte("ORPHAN_KEY"), 0600)

	// Prune dry-run
	if err := handleClusterCommand([]string{"cluster", "prune", "--storage-dir", storageDir, "--dry-run", "--yes"}); err != nil {
		t.Fatalf("cluster prune --dry-run failed: %v", err)
	}
	if _, err := os.Stat(orphanDir); os.IsNotExist(err) {
		t.Errorf("orphan directory was removed during dry-run")
	}

	// Prune execution
	if err := handleClusterCommand([]string{"cluster", "prune", "--storage-dir", storageDir, "--reload-cmd", "true", "--yes"}); err != nil {
		t.Fatalf("cluster prune failed: %v", err)
	}
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("orphan directory was not removed by prune")
	}

	// 5. Test error cases: missing action, unknown action, missing target
	if err := handleClusterCommand([]string{"cluster"}); err == nil {
		t.Errorf("expected error for missing cluster action")
	}
	if err := handleClusterCommand([]string{"cluster", "unknown_action"}); err == nil {
		t.Errorf("expected error for unknown cluster action")
	}
	if err := handleClusterCommand([]string{"cluster", "remove"}); err == nil {
		t.Errorf("expected error for cluster remove without target")
	}
}
