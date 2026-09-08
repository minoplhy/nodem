package db

import (
	"context"
	"time"
)

// LinkedGroupNotification represents a group-channel notification binding with channel metadata.
type LinkedGroupNotification struct {
	GroupNotification GroupNotification
	Channel           NotificationChannel
}

// Repository defines all database operations for node_monitor_go.
type Repository interface {
	InitDB(ctx context.Context) error
	Close() error

	// User Operations
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	CreateUser(ctx context.Context, username, passwordHash, role string) (*User, error)
	UserExists(ctx context.Context) (bool, error)

	// Session Operations
	CreateSession(ctx context.Context, sessionID, publicID string, userID int64, expiresAt time.Time, ipAddress, userAgent *string) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	DeleteSessionByPublicID(ctx context.Context, userID int64, id string) (bool, error)
	ListSessions(ctx context.Context, userID int64) ([]Session, error)
	CleanupExpiredSessions(ctx context.Context) error

	// DNS Provider Operations
	GetProvider(ctx context.Context, tenantID, id int64) (*DnsProviderConfig, error)
	GetProviderByIDDirect(ctx context.Context, id int64) (*DnsProviderConfig, error)
	ListProviders(ctx context.Context, tenantID int64) ([]DnsProviderConfig, error)
	CreateProvider(ctx context.Context, tenantID int64, name, providerType, apiURL, token, zone string) (*DnsProviderConfig, error)
	DeleteProvider(ctx context.Context, tenantID, id int64) error
	UpdateProvider(ctx context.Context, tenantID, id int64, name, providerType, apiURL, token, zone string) (*DnsProviderConfig, error)

	// Target Group Operations
	GetGroup(ctx context.Context, tenantID, id int64) (*TargetGroup, error)
	GetGroupDirect(ctx context.Context, id int64) (*TargetGroup, error)
	ListGroups(ctx context.Context, tenantID int64) ([]TargetGroup, error)
	ListAllGroups(ctx context.Context) ([]TargetGroup, error)
	CreateGroup(ctx context.Context, tenantID int64, name, dnsRecord string, dnsProviderID, checkIntervalSecs int64) (*TargetGroup, error)
	UpdateGroup(ctx context.Context, tenantID, id int64, name, dnsRecord string, dnsProviderID, checkIntervalSecs int64) (*TargetGroup, error)
	DeleteGroup(ctx context.Context, tenantID, id int64) error
	UpdateGroupEnabled(ctx context.Context, tenantID, id int64, enabled bool) error

	// Target IP Operations
	ListIPs(ctx context.Context, groupID int64) ([]TargetIp, error)
	AddIP(ctx context.Context, groupID int64, ip string) (*TargetIp, error)
	DeleteIP(ctx context.Context, groupID, id int64) error
	UpdateIPStatus(ctx context.Context, id int64, status string, dnsAdded bool, lastChecked *time.Time) error
	SyncIPs(ctx context.Context, groupID int64, newIPs []TargetIpInput) error

	// Check Configuration Operations
	ListChecks(ctx context.Context, groupID int64) ([]CheckConfig, error)
	AddCheck(ctx context.Context, groupID int64, name, protocol string, domain *string, port uint16, path *string, downThreshold, upThreshold int64, bypassOnGlobalFailure bool) (*CheckConfig, error)
	UpdateCheck(ctx context.Context, groupID, id int64, name, protocol string, domain *string, port uint16, path *string, downThreshold, upThreshold int64, bypassOnGlobalFailure bool) (*CheckConfig, error)
	DeleteCheck(ctx context.Context, groupID, id int64) error

	// Check State Operations
	GetCheckState(ctx context.Context, ipID, checkID int64) (*CheckState, error)
	ListCheckStatesForIP(ctx context.Context, ipID int64) ([]CheckState, error)
	ListCheckStatesForGroup(ctx context.Context, groupID int64) ([]CheckState, error)
	UpdateCheckState(ctx context.Context, ipID, checkID, consecutiveUp, consecutiveDown int64, status string, message *string) error

	// Group Rule Operations
	ListRules(ctx context.Context, groupID int64) ([]GroupRule, error)
	AddRule(ctx context.Context, groupID int64, expressionJSON, action string) (*GroupRule, error)
	DeleteRule(ctx context.Context, groupID, id int64) error

	// Check Log Operations
	AddCheckLog(ctx context.Context, ipID, checkID int64, success bool, message *string) (*CheckLog, error)
	ListRecentGroupLogs(ctx context.Context, groupID int64, ipID *int64, limit int64) ([]CheckLog, error)

	// Notification Channel Operations
	GetNotificationChannel(ctx context.Context, tenantID, id int64) (*NotificationChannel, error)
	ListNotificationChannels(ctx context.Context, tenantID int64) ([]NotificationChannel, error)
	CreateNotificationChannel(ctx context.Context, tenantID int64, name, channelType, configJSON string) (*NotificationChannel, error)
	UpdateNotificationChannel(ctx context.Context, tenantID, id int64, name, channelType, configJSON string) (*NotificationChannel, error)
	DeleteNotificationChannel(ctx context.Context, tenantID, id int64) error

	// Group Notification Linkage Operations
	LinkGroupNotification(ctx context.Context, groupID, channelID int64, notifyOnUp, notifyOnDown bool) error
	UnlinkGroupNotification(ctx context.Context, groupID, channelID int64) error
	ListGroupNotifications(ctx context.Context, groupID int64) ([]LinkedGroupNotification, error)

	// Group Anomaly Operations
	ListGroupAnomalies(ctx context.Context, groupID int64) ([]string, error)
	AddGroupAnomaly(ctx context.Context, groupID int64, ip string) error
	DeleteGroupAnomaly(ctx context.Context, groupID int64, ip string) error
}
