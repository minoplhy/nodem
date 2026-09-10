package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
	"github.com/minoplhy/nodem/internal/version"
)

type AgentConfig struct {
	ServerURL    string
	Transport    string // "https" or "ssh"
	Token        string
	SSHPort      uint16
	SSHKeyPath   string
	StorageDir   string
	ProxyType    string // "nginx", "caddy", "haproxy", "hook"
	ReloadCmd    string
	HookScript   string
	IntervalSecs int
	Once         bool
	DryRun       bool
	InitSystem   string // "auto", "systemd", "openrc"
}

type ClusterState struct {
	ClusterID      int64     `json:"cluster_id,omitempty"`
	ClusterName    string    `json:"cluster_name,omitempty"`
	AppliedVersion int64     `json:"applied_version"`
	LastSyncTime   time.Time `json:"last_sync_time"`
}

type AgentState struct {
	Clusters map[string]ClusterState `json:"clusters"`
}

func ClusterDirName(clusterID int64) string {
	return fmt.Sprintf("cluster_%d", clusterID)
}

func ClusterDirPath(storageDir string, clusterID int64) string {
	return filepath.Join(storageDir, ClusterDirName(clusterID))
}

func ParseClusterIDFromDir(dirName string) (int64, bool) {
	if strings.HasPrefix(dirName, "cluster_") {
		idStr := strings.TrimPrefix(dirName, "cluster_")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			return id, true
		}
	}
	return 0, false
}

func main() {
	// 1. Load environment variables from .env.agent, .env, or system service configuration if present
	_ = godotenv.Load(".env.agent", ".env", "/etc/nodem-agent/agent.env", "/etc/ech_agent/agent.env")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v", "--version":
			fmt.Printf("nodem-agent %s\n", version.Full())
			return
		case "cluster":
			if err := handleClusterCommand(os.Args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	var cfg AgentConfig

	flag.StringVar(&cfg.ServerURL, "server", getEnv("ECH_SERVER", "http://localhost:8080"), "Central server URL or host")
	flag.StringVar(&cfg.Transport, "transport", getEnv("ECH_TRANSPORT", "https"), "Pull transport: 'https' or 'ssh'")
	flag.StringVar(&cfg.Token, "token", getEnv("ECH_TOKEN", ""), "Agent authentication secret token")
	var sshPortInt int
	flag.IntVar(&sshPortInt, "ssh-port", getEnvInt("ECH_SSH_PORT", 34234), "SSH server port for ssh transport")
	flag.StringVar(&cfg.SSHKeyPath, "ssh-key", getEnv("ECH_SSH_KEY", ""), "Path to agent SSH private key")
	flag.StringVar(&cfg.StorageDir, "storage-dir", getEnv("ECH_STORAGE_DIR", "/opt/ech"), "Local directory to stage ECH files")
	flag.StringVar(&cfg.ProxyType, "proxy", getEnv("ECH_PROXY", "nginx"), "Reverse proxy handler: nginx, caddy, haproxy, hook")
	flag.StringVar(&cfg.ReloadCmd, "reload-cmd", getEnv("ECH_RELOAD_CMD", ""), "Custom proxy reload command override")
	flag.StringVar(&cfg.HookScript, "hook", getEnv("ECH_HOOK_SCRIPT", ""), "Path to custom script for --proxy=hook")
	flag.StringVar(&cfg.InitSystem, "init-system", getEnv("ECH_INIT_SYSTEM", "auto"), "Init system: 'auto', 'systemd', 'openrc'")
	flag.IntVar(&cfg.IntervalSecs, "interval", getEnvInt("ECH_INTERVAL", 300), "Poll interval in seconds (daemon mode)")
	flag.BoolVar(&cfg.Once, "once", false, "Run single sync cycle and exit")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Simulate key staging and reload without modifying files")

	flag.Parse()
	cfg.SSHPort = uint16(sshPortInt)
	cfg.Transport = strings.ToLower(strings.TrimSpace(cfg.Transport))
	cfg.ProxyType = strings.ToLower(strings.TrimSpace(cfg.ProxyType))

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	slog.Info("Starting ECH Agent", "version", version.Full(), "transport", cfg.Transport, "proxy", cfg.ProxyType, "storage", cfg.StorageDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Once {
		if err := runSyncCycle(ctx, cfg); err != nil {
			slog.Error("Sync cycle failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Single sync cycle completed successfully.")
		return
	}

	// Daemon mode: run periodic ticker
	if err := runSyncCycle(ctx, cfg); err != nil {
		slog.Warn("Initial sync cycle error", "error", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := runSyncCycle(ctx, cfg); err != nil {
				slog.Error("Sync cycle error", "error", err)
			}
		case <-ctx.Done():
			slog.Info("ECH Agent exiting.")
			return
		}
	}
}

func loadLocalState(stateFile string) AgentState {
	st := AgentState{Clusters: make(map[string]ClusterState)}
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

func saveLocalState(stateFile string, st AgentState) error {
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

func migrateLegacyClusterFolders(storageDir string, st *AgentState, clusters []transport.ClusterSyncItem) bool {
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

func runSyncCycle(ctx context.Context, cfg AgentConfig) error {
	stateFile := filepath.Join(cfg.StorageDir, "agent_state.json")
	localState := loadLocalState(stateFile)

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
		resp, err = pullViaSSH(ctx, cfg, versionMap)
	} else {
		resp, err = pullViaHTTPS(ctx, cfg, versionMap)
	}

	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	// Migrate any legacy name-based directories or state entries
	if migrateLegacyClusterFolders(cfg.StorageDir, &localState, resp.Clusters) {
		_ = saveLocalState(stateFile, localState)
		_ = generateMasterIncludesConf(cfg.StorageDir)
	}

	// Build map of assigned clusters from server response
	assignedIDs := make(map[int64]bool)
	assignedNames := make(map[string]bool)
	for _, c := range resp.Clusters {
		assignedIDs[c.ClusterID] = true
		assignedNames[c.ClusterName] = true
	}

	// Identify clusters to remove
	clustersToRemoveMap := make(map[int64]string) // clusterID -> clusterName

	// (a) From explicit server response
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

	// (b) From local state reconciliation (any local cluster not in server's assigned clusters)
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
			// Verify Ed25519 signature if provided
			if c.Signature != nil && c.Signature.PublicKey != "" {
				payloadBytes, err := json.Marshal(c.Keys)
				if err == nil {
					if !engine.VerifySignature(c.Signature.PublicKey, payloadBytes, c.Signature.SigBase64) {
						return fmt.Errorf("SECURITY ALERT: Ed25519 signature verification failed for cluster %d (%s)! Refusing to deploy", c.ClusterID, c.ClusterName)
					}
					slog.Info("Payload Ed25519 signature verified successfully", "cluster_id", c.ClusterID, "cluster", c.ClusterName)
				}
			}
			newClusters = append(newClusters, c)
		}
	}

	if len(newClusters) == 0 && len(clustersToRemoveMap) == 0 {
		slog.Info("All assigned ECH clusters are up to date.")
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
	removeErr := removeLocally(cfg, clustersToRemoveMap)
	if removeErr != nil {
		slog.Error("Local cluster removal encountered errors", "error", removeErr)
	}

	// Deploy updated clusters locally
	var deployErr error
	if len(newClusters) > 0 {
		deployErr = deployLocally(cfg, newClusters)
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
			_ = ackViaSSH(ctx, cfg, acks)
		} else {
			_ = ackViaHTTPS(ctx, cfg, acks)
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
	_ = saveLocalState(stateFile, localState)

	// If any clusters were removed and no new clusters deployed, ensure proxy is reloaded once
	if len(newClusters) == 0 && len(clustersToRemoveMap) > 0 {
		if err := reloadLocalProxies(cfg, nil); err != nil {
			slog.Error("Proxy reload after removal failed", "error", err)
			return err
		}
	}

	slog.Info("ECH sync cycle completed successfully", "deployed", len(newClusters), "removed", len(clustersToRemoveMap))
	return nil
}

func deployLocally(cfg AgentConfig, updatedClusters []transport.ClusterSyncItem) error {
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

	// Execute proxy-specific handler ONCE after all updated cluster keys are written
	return reloadLocalProxies(cfg, updatedClusters)
}

func removeLocally(cfg AgentConfig, clustersToRemove map[int64]string) error {
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

// generateMasterIncludesConf creates a unified ech_includes.conf referencing all active clusters.
func generateMasterIncludesConf(storageDir string) error {
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return fmt.Errorf("failed reading storage dir: %w", err)
	}

	stateFile := filepath.Join(storageDir, "agent_state.json")
	localState := loadLocalState(stateFile)
	nameMap := make(map[int64]string)
	for _, cs := range localState.Clusters {
		if cs.ClusterID > 0 {
			nameMap[cs.ClusterID] = cs.ClusterName
		}
	}

	var sb strings.Builder
	sb.WriteString("# Automatically generated by nodem-agent - multi-cluster configuration\n")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		clusterDir := filepath.Join(storageDir, entry.Name())
		currPath := filepath.Join(clusterDir, "ech_current.pem")
		prevPath := filepath.Join(clusterDir, "ech_previous.pem")

		if _, err := os.Stat(currPath); err == nil {
			clusterID, isClusterDir := ParseClusterIDFromDir(entry.Name())
			if isClusterDir {
				clusterName := nameMap[clusterID]
				if clusterName != "" {
					sb.WriteString(fmt.Sprintf("\n# Cluster ID %d (%s)\n", clusterID, clusterName))
				} else {
					sb.WriteString(fmt.Sprintf("\n# Cluster ID %d\n", clusterID))
				}
			} else {
				sb.WriteString(fmt.Sprintf("\n# Cluster: %s\n", entry.Name()))
			}
			sb.WriteString(fmt.Sprintf("ssl_ech_file \"%s\";\n", currPath))
			if _, err := os.Stat(prevPath); err == nil {
				sb.WriteString(fmt.Sprintf("ssl_ech_file \"%s\";\n", prevPath))
			}
		}
	}

	includesPath := filepath.Join(storageDir, "ech_includes.conf")
	tmpPath := includesPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed writing ech_includes.conf.tmp: %w", err)
	}
	return os.Rename(tmpPath, includesPath)
}

func executeCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command '%s' failed: %s: %w", command, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func resolveEndpointURL(serverURL, endpoint string) string {
	serverURL = strings.TrimSpace(serverURL)
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		if strings.HasPrefix(serverURL, "localhost") || strings.HasPrefix(serverURL, "127.0.0.1") {
			serverURL = "http://" + serverURL
		} else {
			serverURL = "https://" + serverURL
		}
	}
	serverURL = strings.TrimRight(serverURL, "/")
	serverURL = strings.TrimSuffix(serverURL, "/api")
	serverURL = strings.TrimRight(serverURL, "/")

	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	return serverURL + endpoint
}

func pullViaHTTPS(ctx context.Context, cfg AgentConfig, clusters map[string]int64) (*transport.SyncResponse, error) {
	reqURL := resolveEndpointURL(cfg.ServerURL, "/api/v1/agent/ech/sync")
	reqBody, _ := json.Marshal(transport.SyncRequest{
		ProxyType: cfg.ProxyType,
		Version:   version.Full(),
		Clusters:  clusters,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-Token", cfg.Token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var syncResp transport.SyncResponse
	if err := json.Unmarshal(bodyBytes, &syncResp); err != nil {
		return nil, fmt.Errorf("failed decoding server json: %w", err)
	}

	return &syncResp, nil
}

func ackViaHTTPS(ctx context.Context, cfg AgentConfig, acks []transport.ClusterAckItem) error {
	reqURL := resolveEndpointURL(cfg.ServerURL, "/api/v1/agent/ech/ack")
	reqBody, _ := json.Marshal(transport.AckRequest{
		Acks: acks,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-Token", cfg.Token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func pullViaSSH(ctx context.Context, cfg AgentConfig, clusters map[string]int64) (*transport.SyncResponse, error) {
	client, err := dialSSH(cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	versJSON, _ := json.Marshal(clusters)
	cmd := fmt.Sprintf("sync --proxy %s --versions %q", cfg.ProxyType, string(versJSON))
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("ssh command failed: %s: %w", string(out), err)
	}

	var syncResp transport.SyncResponse
	if err := json.Unmarshal(out, &syncResp); err != nil {
		return nil, fmt.Errorf("failed unmarshaling SSH json response: %w (raw: %s)", err, string(out))
	}

	return &syncResp, nil
}

func ackViaSSH(ctx context.Context, cfg AgentConfig, acks []transport.ClusterAckItem) error {
	client, err := dialSSH(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	acksJSON, _ := json.Marshal(acks)
	cmd := fmt.Sprintf("ack --acks %q", string(acksJSON))
	return session.Run(cmd)
}

func dialSSH(cfg AgentConfig) (*ssh.Client, error) {
	host := cfg.ServerURL
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if strings.Contains(host, "/") {
		host = strings.Split(host, "/")[0]
	}
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	addr := fmt.Sprintf("%s:%d", host, cfg.SSHPort)
	var authMethods []ssh.AuthMethod

	if cfg.SSHKeyPath != "" {
		keyBytes, err := os.ReadFile(cfg.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed reading ssh key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing ssh key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cfg.Token != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Token))
	}

	user := "sync"
	if cfg.Token != "" {
		user = cfg.Token
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	return ssh.Dial("tcp", addr, sshConfig)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

