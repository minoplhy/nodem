package agent

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/minoplhy/nodem/internal/ech/transport"
)

const (
	InitSystemAuto    = "auto"
	InitSystemSystemd = "systemd"
	InitSystemOpenRC  = "openrc"
)

// DetectInitSystem identifies if the host is using Systemd or OpenRC for proxy reloads.
func DetectInitSystem() (string, error) {
	if fi, err := os.Stat("/run/systemd/system"); err == nil && fi.IsDir() {
		return InitSystemSystemd, nil
	}
	if fi, err := os.Stat("/run/openrc"); err == nil && fi.IsDir() {
		return InitSystemOpenRC, nil
	}
	if fi, err := os.Stat("/sbin/openrc-run"); err == nil && !fi.IsDir() {
		return InitSystemOpenRC, nil
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return InitSystemOpenRC, nil
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return InitSystemSystemd, nil
	}
	return "", fmt.Errorf("unable to auto-detect init system")
}

// ReloadLocalProxies handles configuration generation and service reloading for updated clusters.
func ReloadLocalProxies(cfg Config, updatedClusters []transport.ClusterSyncItem) error {
	initSys := cfg.InitSystem
	if initSys == "" || initSys == InitSystemAuto {
		detected, err := DetectInitSystem()
		if err == nil {
			initSys = detected
		} else {
			initSys = "standalone"
		}
	}

	switch cfg.ProxyType {
	case "nginx":
		if err := GenerateMasterIncludesConf(cfg.StorageDir); err != nil {
			return err
		}
		if cfg.ReloadCmd != "" {
			slog.Info("Executing custom reload command for nginx", "command", cfg.ReloadCmd)
			return ExecuteCommand(cfg.ReloadCmd)
		}
		return ReloadNginx(initSys)

	case "caddy":
		if cfg.ReloadCmd != "" {
			slog.Info("Executing custom reload command for caddy", "command", cfg.ReloadCmd)
			return ExecuteCommand(cfg.ReloadCmd)
		}
		return ReloadCaddy()

	case "haproxy":
		if cfg.ReloadCmd != "" {
			slog.Info("Executing custom reload command for haproxy", "command", cfg.ReloadCmd)
			return ExecuteCommand(cfg.ReloadCmd)
		}
		return ReloadHAProxy(initSys)

	case "hook":
		if cfg.HookScript == "" {
			return fmt.Errorf("proxy=hook requested but --hook script not provided")
		}
		for _, cluster := range updatedClusters {
			clusterDir := ClusterDirPath(cfg.StorageDir, cluster.ClusterID)
			if cluster.ClusterID <= 0 {
				clusterDir = filepath.Join(cfg.StorageDir, cluster.ClusterName)
			}
			currPath := filepath.Join(clusterDir, "ech_current.pem")
			prevPath := filepath.Join(clusterDir, "ech_previous.pem")
			cmd := exec.Command(cfg.HookScript, currPath, prevPath)
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env,
				"ECH_ACTION=deploy",
				fmt.Sprintf("ECH_CLUSTER_ID=%d", cluster.ClusterID),
				fmt.Sprintf("ECH_CLUSTER_NAME=%s", cluster.ClusterName),
				fmt.Sprintf("ECH_CURRENT_PEM=%s", currPath),
				fmt.Sprintf("ECH_PREVIOUS_PEM=%s", prevPath),
				fmt.Sprintf("ECH_BASE64=%s", cluster.Keys.Base64ECH),
				fmt.Sprintf("ECH_VERSION=%d", cluster.Version),
				fmt.Sprintf("ECH_PUBLIC_NAME=%s", cluster.PublicName),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("hook script failed for cluster %d (%s): %s: %w", cluster.ClusterID, cluster.ClusterName, string(out), err)
			}
		}
		return nil

	default:
		slog.Warn("Unknown proxy adapter, skipping reload", "proxy", cfg.ProxyType)
		return nil
	}
}

// ReloadNginx validates configuration first, then reloads via OpenRC, Systemd, or signal.
func ReloadNginx(initSys string) error {
	testCmd := "nginx -t"
	if os.Geteuid() != 0 {
		testCmd = "sudo nginx -t 2>/dev/null || nginx -t"
	}
	if err := ExecuteCommand(testCmd); err != nil {
		return fmt.Errorf("nginx configuration test failed; refusing reload: %w", err)
	}

	var reloadCmd string
	switch initSys {
	case InitSystemOpenRC:
		if os.Geteuid() == 0 {
			reloadCmd = "rc-service nginx reload 2>/dev/null || nginx -s reload"
		} else {
			reloadCmd = "sudo rc-service nginx reload 2>/dev/null || rc-service nginx reload 2>/dev/null || sudo nginx -s reload 2>/dev/null || nginx -s reload"
		}

	case InitSystemSystemd:
		if os.Geteuid() == 0 {
			reloadCmd = "systemctl reload nginx 2>/dev/null || nginx -s reload"
		} else {
			reloadCmd = "sudo systemctl reload nginx 2>/dev/null || systemctl reload nginx 2>/dev/null || sudo nginx -s reload 2>/dev/null || nginx -s reload"
		}

	default:
		if os.Geteuid() == 0 {
			reloadCmd = "nginx -s reload"
		} else {
			reloadCmd = "sudo nginx -s reload 2>/dev/null || nginx -s reload"
		}
	}

	slog.Info("Reloading Nginx with init-aware command", "init_system", initSys, "cmd", reloadCmd)
	return ExecuteCommand(reloadCmd)
}

// ReloadHAProxy reloads HAProxy using Systemd or OpenRC service management.
func ReloadHAProxy(initSys string) error {
	var reloadCmd string
	switch initSys {
	case InitSystemOpenRC:
		if os.Geteuid() == 0 {
			reloadCmd = "rc-service haproxy reload"
		} else {
			reloadCmd = "sudo rc-service haproxy reload 2>/dev/null || rc-service haproxy reload"
		}

	case InitSystemSystemd:
		if os.Geteuid() == 0 {
			reloadCmd = "systemctl reload haproxy"
		} else {
			reloadCmd = "sudo systemctl reload haproxy 2>/dev/null || systemctl reload haproxy"
		}

	default:
		if os.Geteuid() == 0 {
			reloadCmd = "haproxy -f /etc/haproxy/haproxy.cfg -p /run/haproxy.pid -sf $(cat /run/haproxy.pid 2>/dev/null)"
		} else {
			reloadCmd = "sudo systemctl reload haproxy 2>/dev/null || sudo rc-service haproxy reload"
		}
	}

	slog.Info("Reloading HAProxy", "init_system", initSys, "cmd", reloadCmd)
	return ExecuteCommand(reloadCmd)
}

// ReloadCaddy reloads Caddy via its native CLI.
func ReloadCaddy() error {
	cmd := "caddy reload"
	slog.Info("Reloading Caddy", "cmd", cmd)
	return ExecuteCommand(cmd)
}

// ExecuteCommand executes a shell command returning an error with command output on failure.
func ExecuteCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command '%s' failed: %s: %w", command, strings.TrimSpace(string(out)), err)
	}
	return nil
}
