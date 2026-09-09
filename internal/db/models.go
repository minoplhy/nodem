package db

import (
	"time"
)

// User represents an administrator or tenant account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an active authenticated user session.
type Session struct {
	SessionID  string     `json:"session_id"`
	PublicID   string     `json:"public_id"`
	UserID     int64      `json:"user_id"`
	ExpiresAt  time.Time  `json:"expires_at"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	LastActive *time.Time `json:"last_active,omitempty"`
}

// DnsProviderConfig holds credentials and settings for DNS providers.
type DnsProviderConfig struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenant_id"`
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"` // "Cloudflare", "Technitium"
	APIURL       string `json:"api_url"`
	Token        string `json:"token"`
	Zone         string `json:"zone"`
}

// TargetGroup represents a group of nodes monitored for a specific DNS record.
type TargetGroup struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	Name              string    `json:"name"`
	DnsRecord         string    `json:"dns_record"`
	DnsProviderID     int64     `json:"dns_provider_id"`
	CheckIntervalSecs int64     `json:"check_interval_secs"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
}

// TargetIp represents an individual IP belonging to a TargetGroup.
type TargetIp struct {
	ID           int64      `json:"id"`
	GroupID      int64      `json:"group_id"`
	IP           string     `json:"ip"`
	DnsAdded     bool       `json:"dns_added"`
	LastChecked  *time.Time `json:"last_checked,omitempty"`
	Status       string     `json:"status"` // "UP", "DOWN", "UNKNOWN"
	DisplayOrder int64      `json:"display_order"`
	Enabled      bool       `json:"enabled"`
}

// TargetIpInput represents batch IP input for synchronization.
type TargetIpInput struct {
	IP      string `json:"ip"`
	Enabled bool   `json:"enabled"`
}

// CheckConfig defines a health check for a TargetGroup.
type CheckConfig struct {
	ID                     int64   `json:"id"`
	GroupID                int64   `json:"group_id"`
	Name                   string  `json:"name"`
	Protocol               string  `json:"protocol"` // "TCP", "UDP", "HTTP", "HTTPS"
	Domain                 *string `json:"domain,omitempty"`
	Port                   uint16  `json:"port"`
	Path                   *string `json:"path,omitempty"`
	DownThreshold          int64   `json:"down_threshold"`
	UpThreshold            int64   `json:"up_threshold"`
	BypassOnGlobalFailure bool    `json:"bypass_on_global_failure"`
}

// CheckState tracks consecutive check successes and failures per IP and check.
type CheckState struct {
	IPID            int64   `json:"ip_id"`
	CheckID         int64   `json:"check_id"`
	ConsecutiveUp   int64   `json:"consecutive_up"`
	ConsecutiveDown int64   `json:"consecutive_down"`
	Status          string  `json:"status"` // "UP", "DOWN", "UNKNOWN"
	Message         *string `json:"message,omitempty"`
}

// CheckLog represents an audit log entry for a check execution.
type CheckLog struct {
	ID        int64     `json:"id"`
	IPID      int64     `json:"ip_id"`
	CheckID   int64     `json:"check_id"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Message   *string   `json:"message"`
	IPAddress *string   `json:"ip_address,omitempty"`
}

// GroupRule specifies custom logic for adding or removing an IP from DNS.
type GroupRule struct {
	ID             int64  `json:"id"`
	GroupID        int64  `json:"group_id"`
	ExpressionJSON string `json:"expression_json"`
	Action         string `json:"action"` // "AddToDns", "RemoveFromDns"
}

// NotificationChannel defines an alerting endpoint (Discord, Slack, Telegram).
type NotificationChannel struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"` // "DISCORD", "SLACK", "TELEGRAM"
	ConfigJSON  string `json:"config_json"`
}

// GroupNotification defines the linkage between a TargetGroup and a NotificationChannel.
type GroupNotification struct {
	GroupID      int64 `json:"group_id"`
	ChannelID    int64 `json:"channel_id"`
	NotifyOnUp   bool  `json:"notify_on_up"`
	NotifyOnDown bool  `json:"notify_on_down"`
}

// ECH Transport constants (strictly exclusive per node)
const (
	PullTransportHTTPS = "HTTPS"
	PullTransportSSH   = "SSH"
)

// ECH Proxy Types
const (
	ProxyTypeNginx   = "NGINX"
	ProxyTypeCaddy   = "CADDY"
	ProxyTypeHAProxy = "HAPROXY"
	ProxyTypeHook    = "HOOK"
)

// ECH Key Statuses
const (
	KeyStatusActive   = "ACTIVE"
	KeyStatusPrevious = "PREVIOUS"
	KeyStatusArchived = "ARCHIVED"
)

// ECH Node Sync Statuses
const (
	SyncStatusInSync   = "IN_SYNC"
	SyncStatusOutdated = "OUTDATED"
	SyncStatusFailed   = "FAILED"
	SyncStatusPending  = "PENDING"
)

// ECHCluster represents a public cover name and cryptographic configuration for ECH keys.
type ECHCluster struct {
	ID                    int64      `json:"id"`
	TenantID              int64      `json:"tenant_id"`
	Name                  string     `json:"name"`
	PublicName            string     `json:"public_name"`
	CipherSuite           string     `json:"cipher_suite"`
	MaxNameLen           int        `json:"max_name_len"`
	RotationIntervalHours int        `json:"rotation_interval_hours"`
	LastRotatedAt         *time.Time `json:"last_rotated_at,omitempty"`
	NextRotationAt        *time.Time `json:"next_rotation_at,omitempty"`
	AutoRotate            bool       `json:"auto_rotate"`
	CurrentVersion        int64      `json:"current_version"`
	SigningPublicKey      string     `json:"signing_public_key"`
	SigningPrivateKey     string     `json:"-"`
	CreatedAt             time.Time  `json:"created_at"`
}

// ECHKey stores active and historical ECH key pairs for a cluster.
type ECHKey struct {
	ID            int64     `json:"id"`
	ClusterID     int64     `json:"cluster_id"`
	Version       int64     `json:"version"`
	Status        string    `json:"status"` // "ACTIVE", "PREVIOUS", "ARCHIVED"
	Base64ECH     string    `json:"base64_ech"`
	PrivateKeyPEM string    `json:"-"`
	ECHConfigPEM  string    `json:"ech_config_pem"`
	FullPEM       string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

// ECHNode represents an independent edge reverse proxy host pulling keys from the control plane.
type ECHNode struct {
	ID            int64              `json:"id"`
	TenantID      int64              `json:"tenant_id"`
	Name          string             `json:"name"`
	PullTransport string             `json:"pull_transport"` // "HTTPS" or "SSH"
	AuthTokenHash string             `json:"-"`
	SSHPublicKey  *string            `json:"ssh_public_key,omitempty"`
	ProxyType     string             `json:"proxy_type"` // "NGINX", "CADDY", "HAPROXY", "HOOK"
	LastSeenAt    *time.Time         `json:"last_seen_at,omitempty"`
	LastIP        *string            `json:"last_ip,omitempty"`
	Clusters      []ECHClusterStatus `json:"clusters,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ECHClusterStatus represents the synchronization status of a specific cluster on an edge node.
type ECHClusterStatus struct {
	ClusterID          int64      `json:"cluster_id"`
	ClusterName        string     `json:"cluster_name"`
	PublicName         string     `json:"public_name"`
	LastAppliedVersion int64      `json:"last_applied_version"`
	SyncStatus         string     `json:"sync_status"` // "IN_SYNC", "OUTDATED", "FAILED", "PENDING"
	LastError          *string    `json:"last_error,omitempty"`
	LastSyncedAt       *time.Time `json:"last_synced_at,omitempty"`
}

// ECHClusterNode represents an edge node assigned to a cluster, viewed from the cluster's perspective.
type ECHClusterNode struct {
	ClusterID          int64      `json:"cluster_id"`
	NodeID             int64      `json:"node_id"`
	NodeName           string     `json:"node_name"`
	PullTransport      string     `json:"pull_transport"`
	ProxyType          string     `json:"proxy_type"`
	LastAppliedVersion int64      `json:"last_applied_version"`
	SyncStatus         string     `json:"sync_status"`
	LastError          *string    `json:"last_error,omitempty"`
	LastSyncedAt       *time.Time `json:"last_synced_at,omitempty"`
}

// ECHDomain links a target domain to an ECHCluster and DNS provider for HTTPS record publication.
type ECHDomain struct {
	ID            int64      `json:"id"`
	ClusterID     int64      `json:"cluster_id"`
	DNSProviderID int64      `json:"dns_provider_id"`
	TargetGroupID *int64     `json:"target_group_id,omitempty"`
	Domain        string     `json:"domain"`
	TTL           int        `json:"ttl"`
	ALPN          string     `json:"alpn"`
	IPv4Hint      *string    `json:"ipv4_hint,omitempty"`
	IPv6Hint      *string    `json:"ipv6_hint,omitempty"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	DNSStatus     string     `json:"dns_status"` // "SYNCED", "PENDING", "FAILED"
	CreatedAt     time.Time  `json:"created_at"`
}

// ECHLog stores audit records for ECH lifecycle events.
type ECHLog struct {
	ID        int64     `json:"id"`
	ClusterID int64     `json:"cluster_id"`
	NodeID    *int64    `json:"node_id,omitempty"`
	DomainID  *int64    `json:"domain_id,omitempty"`
	EventType string    `json:"event_type"` // "GENERATE", "SYNC_HTTPS", "SYNC_SSH", "ACK", "DNS_UPDATE", "ERROR"
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
