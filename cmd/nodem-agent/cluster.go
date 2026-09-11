package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/minoplhy/nodem/internal/agent"
)

func handleClusterCommand(args []string) error {
	subArgs := args[1:]
	if len(subArgs) == 0 {
		return fmt.Errorf("missing cluster action: list, remove (or rm), prune\nUsage: nodem-agent cluster <action> [flags]")
	}

	action := strings.ToLower(subArgs[0])
	actionArgs := subArgs[1:]

	switch action {
	case "list", "ls":
		return handleClusterList(actionArgs)
	case "remove", "rm", "delete":
		return handleClusterRemove(actionArgs)
	case "prune":
		return handleClusterPrune(actionArgs)
	default:
		return fmt.Errorf("unknown cluster action '%s'; expected list, remove, prune", action)
	}
}

func handleClusterList(args []string) error {
	fs := flag.NewFlagSet("cluster list", flag.ExitOnError)
	storageDir := fs.String("storage-dir", agent.GetEnv("ECH_STORAGE_DIR", agent.DefaultStorageDir), "Directory where ECH keys are staged")
	_ = fs.Parse(args)

	stateFile := filepath.Join(*storageDir, "agent_state.json")
	localState := agent.LoadState(stateFile)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CLUSTER ID\tCLUSTER NAME\tVERSION\tDIRECTORY\tSTATUS\tLAST SYNC")

	if len(localState.Clusters) == 0 {
		w.Flush()
		fmt.Printf("\nNo clusters currently tracked in %s\n", stateFile)
		return nil
	}

	for key, cs := range localState.Clusters {
		cid := cs.ClusterID
		if cid == 0 {
			if parsed, err := strconv.ParseInt(key, 10, 64); err == nil {
				cid = parsed
			}
		}

		dirName := agent.ClusterDirName(cid)
		clusterDir := filepath.Join(*storageDir, dirName)
		currPath := filepath.Join(clusterDir, "ech_current.pem")

		status := "OK"
		if _, err := os.Stat(currPath); os.IsNotExist(err) {
			status = "MISSING_KEYS"
		}

		lastSync := "-"
		if !cs.LastSyncTime.IsZero() {
			lastSync = cs.LastSyncTime.Format("2006-01-02 15:04:05")
		}

		name := cs.ClusterName
		if name == "" {
			name = "-"
		}

		fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\n", cid, name, cs.AppliedVersion, dirName, status, lastSync)
	}
	return w.Flush()
}

func handleClusterRemove(args []string) error {
	fs := flag.NewFlagSet("cluster remove", flag.ExitOnError)
	storageDir := fs.String("storage-dir", agent.GetEnv("ECH_STORAGE_DIR", agent.DefaultStorageDir), "Directory where ECH keys are staged")
	proxyType := fs.String("proxy", agent.GetEnv("ECH_PROXY", "nginx"), "Proxy type: nginx, caddy, haproxy, hook")
	reloadCmd := fs.String("reload-cmd", agent.GetEnv("ECH_RELOAD_CMD", ""), "Custom proxy reload command override")
	hookScript := fs.String("hook", agent.GetEnv("ECH_HOOK_SCRIPT", ""), "Path to hook script for proxy=hook")
	initSys := fs.String("init-system", agent.GetEnv("ECH_INIT_SYSTEM", "auto"), "Init system: auto, systemd, openrc")
	yesFlag := fs.Bool("yes", false, "Skip confirmation prompt")
	yFlag := fs.Bool("y", false, "Alias for --yes")
	dryRun := fs.Bool("dry-run", false, "Simulate removal without modifying files")

	var filteredArgs []string
	target := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && target == "" {
			target = a
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}
	_ = fs.Parse(filteredArgs)

	if target == "" {
		return fmt.Errorf("usage: nodem-agent cluster remove <cluster-id-or-name> [--yes] [--dry-run]")
	}

	stateFile := filepath.Join(*storageDir, "agent_state.json")
	localState := agent.LoadState(stateFile)

	var targetID int64
	var targetName string
	var stateKey string

	// Try parsing target as int64 ID first
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		targetID = id
		for k, cs := range localState.Clusters {
			if cs.ClusterID == id || k == target {
				targetName = cs.ClusterName
				stateKey = k
				break
			}
		}
	} else {
		// Target is a name
		for k, cs := range localState.Clusters {
			if strings.EqualFold(cs.ClusterName, target) || strings.EqualFold(k, target) {
				targetID = cs.ClusterID
				targetName = cs.ClusterName
				stateKey = k
				break
			}
		}
	}

	if targetID <= 0 {
		if strings.HasPrefix(target, "cluster_") {
			if id, ok := agent.ParseClusterIDFromDir(target); ok {
				targetID = id
			}
		}
	}

	if targetID <= 0 {
		return fmt.Errorf("cluster '%s' not found in local state or storage directory", target)
	}

	skipConfirm := *yesFlag || *yFlag
	if !skipConfirm {
		fmt.Printf("Are you sure you want to remove cluster %d (%s) from %s? [y/N]: ", targetID, targetName, *storageDir)
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if *dryRun {
		fmt.Printf("[DRY-RUN] Would remove cluster directory %s and update state\n", agent.ClusterDirPath(*storageDir, targetID))
		return nil
	}

	cfg := agent.Config{
		StorageDir: *storageDir,
		ProxyType:  *proxyType,
		ReloadCmd:  *reloadCmd,
		HookScript: *hookScript,
		InitSystem: *initSys,
	}

	// 1. Remove files and trigger hooks
	clustersToRemove := map[int64]string{targetID: targetName}
	if err := agent.RemoveLocally(cfg, clustersToRemove); err != nil {
		return fmt.Errorf("failed removing cluster files: %w", err)
	}

	// 2. Remove from state
	if stateKey != "" {
		delete(localState.Clusters, stateKey)
	}
	delete(localState.Clusters, fmt.Sprintf("%d", targetID))
	_ = agent.SaveState(stateFile, localState)

	// 3. Reload proxy
	if err := agent.ReloadLocalProxies(cfg, nil); err != nil {
		return fmt.Errorf("failed reloading proxy: %w", err)
	}

	fmt.Printf("Cluster %d (%s) removed successfully.\n", targetID, targetName)
	return nil
}

func handleClusterPrune(args []string) error {
	fs := flag.NewFlagSet("cluster prune", flag.ExitOnError)
	storageDir := fs.String("storage-dir", agent.GetEnv("ECH_STORAGE_DIR", agent.DefaultStorageDir), "Directory where ECH keys are staged")
	proxyType := fs.String("proxy", agent.GetEnv("ECH_PROXY", "nginx"), "Proxy type: nginx, caddy, haproxy, hook")
	reloadCmd := fs.String("reload-cmd", agent.GetEnv("ECH_RELOAD_CMD", ""), "Custom proxy reload command override")
	initSys := fs.String("init-system", agent.GetEnv("ECH_INIT_SYSTEM", "auto"), "Init system: auto, systemd, openrc")
	yesFlag := fs.Bool("yes", false, "Skip confirmation prompt")
	yFlag := fs.Bool("y", false, "Alias for --yes")
	dryRun := fs.Bool("dry-run", false, "Simulate prune without deleting files")
	_ = fs.Parse(args)

	stateFile := filepath.Join(*storageDir, "agent_state.json")
	localState := agent.LoadState(stateFile)

	activeIDs := make(map[int64]bool)
	for _, cs := range localState.Clusters {
		if cs.ClusterID > 0 {
			activeIDs[cs.ClusterID] = true
		}
	}

	entries, err := os.ReadDir(*storageDir)
	if err != nil {
		return fmt.Errorf("failed reading storage dir: %w", err)
	}

	var orphanDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if cid, isClusterDir := agent.ParseClusterIDFromDir(entry.Name()); isClusterDir {
			if !activeIDs[cid] {
				orphanDirs = append(orphanDirs, filepath.Join(*storageDir, entry.Name()))
			}
		}
	}

	if len(orphanDirs) == 0 {
		fmt.Println("No orphaned cluster directories found.")
		return nil
	}

	fmt.Printf("Found %d orphaned cluster directory(ies):\n", len(orphanDirs))
	for _, d := range orphanDirs {
		fmt.Printf("  - %s\n", d)
	}

	if *dryRun {
		fmt.Println("[DRY-RUN] Would prune the above directories.")
		return nil
	}

	skipConfirm := *yesFlag || *yFlag
	if !skipConfirm {
		fmt.Print("Proceed with deletion? [y/N]: ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	for _, d := range orphanDirs {
		_ = os.RemoveAll(d)
	}

	cfg := agent.Config{
		StorageDir: *storageDir,
		ProxyType:  *proxyType,
		ReloadCmd:  *reloadCmd,
		InitSystem: *initSys,
	}
	_ = agent.ReloadLocalProxies(cfg, nil)

	fmt.Printf("Successfully pruned %d directory(ies).\n", len(orphanDirs))
	return nil
}
