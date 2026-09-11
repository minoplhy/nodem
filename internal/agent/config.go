package agent

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

const DefaultStorageDir = "/opt/ech"

// Config contains runtime configuration parameters for nodem-agent.
type Config struct {
	ServerURL       string
	ServerPublicKey string
	Transport       string // "https" or "ssh"
	Token           string
	SSHPort         uint16
	SSHKeyPath      string
	StorageDir      string
	ProxyType       string // "nginx", "caddy", "haproxy", "hook"
	ReloadCmd       string
	HookScript      string
	IntervalSecs    int
	Once            bool
	DryRun          bool
	InitSystem      string // "auto", "systemd", "openrc"
}

// BindFlags registers standard agent CLI flags to the provided FlagSet and returns a finalize callback to call after Parse().
func BindFlags(fs *flag.FlagSet, cfg *Config) func() {
	var sshPortInt int

	fs.StringVar(&cfg.ServerURL, "server", GetEnv("ECH_SERVER", "http://localhost:8080"), "Central server URL or host")
	fs.StringVar(&cfg.ServerPublicKey, "server-public-key", GetEnv("ECH_SERVER_PUBLIC_KEY", ""), "Central server public key")
	fs.StringVar(&cfg.Transport, "transport", GetEnv("ECH_TRANSPORT", "https"), "Pull transport: 'https' or 'ssh'")
	fs.StringVar(&cfg.Token, "token", GetEnv("ECH_TOKEN", ""), "Agent authentication secret token")
	fs.IntVar(&sshPortInt, "ssh-port", GetEnvInt("ECH_SSH_PORT", 34234), "SSH server port for ssh transport")
	fs.StringVar(&cfg.SSHKeyPath, "ssh-key", GetEnv("ECH_SSH_KEY", ""), "Path to agent SSH private key")
	fs.StringVar(&cfg.StorageDir, "storage-dir", GetEnv("ECH_STORAGE_DIR", DefaultStorageDir), "Local directory to stage ECH files")
	fs.StringVar(&cfg.ProxyType, "proxy", GetEnv("ECH_PROXY", "nginx"), "Reverse proxy handler: nginx, caddy, haproxy, hook")
	fs.StringVar(&cfg.ReloadCmd, "reload-cmd", GetEnv("ECH_RELOAD_CMD", ""), "Custom proxy reload command override")
	fs.StringVar(&cfg.HookScript, "hook", GetEnv("ECH_HOOK_SCRIPT", ""), "Path to custom script for --proxy=hook")
	fs.StringVar(&cfg.InitSystem, "init-system", GetEnv("ECH_INIT_SYSTEM", "auto"), "Init system: 'auto', 'systemd', 'openrc'")
	fs.IntVar(&cfg.IntervalSecs, "interval", GetEnvInt("ECH_INTERVAL", 300), "Poll interval in seconds (daemon mode)")
	fs.BoolVar(&cfg.Once, "once", false, "Run single sync cycle and exit")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Simulate key staging and reload without modifying files")

	return func() {
		cfg.SSHPort = uint16(sshPortInt)
		cfg.Transport = strings.ToLower(strings.TrimSpace(cfg.Transport))
		cfg.ProxyType = strings.ToLower(strings.TrimSpace(cfg.ProxyType))
	}
}

// GetEnv retrieves the environment variable or falls back to defaultVal.
func GetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetEnvInt retrieves the integer environment variable or falls back to defaultVal.
func GetEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
