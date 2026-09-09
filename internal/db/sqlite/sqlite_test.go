package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minoplhy/nodem/internal/db"
)

func setupTestDB(t *testing.T) (*SqliteRepository, func()) {
	tmpDir, err := os.MkdirTemp("", "nm_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	dbFile := filepath.Join(tmpDir, "test.db")

	repo, err := New(dbFile)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open test db: %v", err)
	}

	ctx := context.Background()
	if err := repo.InitDB(ctx); err != nil {
		_ = repo.Close()
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init test db: %v", err)
	}

	cleanup := func() {
		_ = repo.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return repo, cleanup
}

func TestSQLiteRepository(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Test Users
	exists, err := repo.UserExists(ctx)
	if err != nil || exists {
		t.Fatalf("Expected no users initially, got exists=%v, err=%v", exists, err)
	}

	u, err := repo.CreateUser(ctx, "admin", "hashed_pass", "Admin")
	if err != nil || u == nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	if u.Username != "admin" || u.Role != "Admin" {
		t.Fatalf("Unexpected user: %+v", u)
	}

	exists, err = repo.UserExists(ctx)
	if err != nil || !exists {
		t.Fatalf("Expected users to exist now, got exists=%v", exists)
	}

	uFound, err := repo.GetUserByUsername(ctx, "admin")
	if err != nil || uFound == nil || uFound.ID != u.ID {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	// 2. Test Sessions
	sessID := "test_session_12345"
	pubID := "sess_test_public_12345"
	ipAddr := "127.0.0.1"
	ua := "GoTest/1.0"
	exp := time.Now().UTC().Add(time.Hour)
	if err := repo.CreateSession(ctx, sessID, pubID, u.ID, exp, &ipAddr, &ua); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	sFound, err := repo.GetSession(ctx, sessID)
	if err != nil || sFound == nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if sFound.UserID != u.ID || sFound.IPAddress == nil || *sFound.IPAddress != ipAddr || sFound.PublicID != pubID {
		t.Fatalf("Session mismatch: %+v", sFound)
	}

	sessions, err := repo.ListSessions(ctx, u.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("ListSessions failed: %v, count=%d", err, len(sessions))
	}
	if sessions[0].PublicID != pubID {
		t.Fatalf("Expected public ID %s, got %s", pubID, sessions[0].PublicID)
	}

	// Test revocation by public ID
	deleted, err := repo.DeleteSessionByPublicID(ctx, u.ID, pubID)
	if err != nil || !deleted {
		t.Fatalf("Failed to delete session by public ID: %v, deleted=%v", err, deleted)
	}
	// Re-create for remaining tests if any
	_ = repo.CreateSession(ctx, sessID, pubID, u.ID, exp, &ipAddr, &ua)

	// 3. Test Providers
	prov, err := repo.CreateProvider(ctx, u.ID, "Cloudflare Main", "Cloudflare", "https://api.cloudflare.com", "token123", "example.com")
	if err != nil || prov == nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	providers, err := repo.ListProviders(ctx, u.ID)
	if err != nil || len(providers) != 1 {
		t.Fatalf("ListProviders failed: %v, count=%d", err, len(providers))
	}

	// 4. Test Target Groups
	grp, err := repo.CreateGroup(ctx, u.ID, "Production Nodes", "nodes.example.com", prov.ID, 30)
	if err != nil || grp == nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	if !grp.Enabled || grp.CheckIntervalSecs != 30 {
		t.Fatalf("Unexpected group: %+v", grp)
	}

	allGroups, err := repo.ListAllGroups(ctx)
	if err != nil || len(allGroups) != 1 {
		t.Fatalf("ListAllGroups failed: %v, count=%d", err, len(allGroups))
	}

	// 5. Test Target IPs & Sync
	ip1, err := repo.AddIP(ctx, grp.ID, "1.1.1.1")
	if err != nil || ip1 == nil {
		t.Fatalf("Failed to add IP: %v", err)
	}
	ip2, err := repo.AddIP(ctx, grp.ID, "2.2.2.2")
	if err != nil || ip2 == nil {
		t.Fatalf("Failed to add IP: %v", err)
	}

	ips, err := repo.ListIPs(ctx, grp.ID)
	if err != nil || len(ips) != 2 {
		t.Fatalf("ListIPs failed: %v, count=%d", err, len(ips))
	}

	// Sync IPs: reorder, disable 1.1.1.1, add 3.3.3.3, remove 2.2.2.2
	newIPs := []db.TargetIpInput{
		{IP: "3.3.3.3", Enabled: true},
		{IP: "1.1.1.1", Enabled: false},
	}
	if err := repo.SyncIPs(ctx, grp.ID, newIPs); err != nil {
		t.Fatalf("SyncIPs failed: %v", err)
	}

	syncedIPs, err := repo.ListIPs(ctx, grp.ID)
	if err != nil || len(syncedIPs) != 2 {
		t.Fatalf("Expected 2 synced IPs, got %d (err=%v)", len(syncedIPs), err)
	}
	if syncedIPs[0].IP != "3.3.3.3" || syncedIPs[1].IP != "1.1.1.1" || syncedIPs[1].Enabled != false {
		t.Fatalf("Sync order/enabled mismatch: %+v", syncedIPs)
	}

	// 6. Test Checks
	check, err := repo.AddCheck(ctx, grp.ID, "HTTPS Health", "HTTPS", nil, 443, nil, 2, 3, true)
	if err != nil || check == nil {
		t.Fatalf("Failed to add check: %v", err)
	}
	if check.Port != 443 || !check.BypassOnGlobalFailure {
		t.Fatalf("Unexpected check: %+v", check)
	}

	// 7. Test CheckState
	msg := "connection timeout"
	if err := repo.UpdateCheckState(ctx, syncedIPs[0].ID, check.ID, 0, 1, "DOWN", &msg); err != nil {
		t.Fatalf("UpdateCheckState failed: %v", err)
	}
	cs, err := repo.GetCheckState(ctx, syncedIPs[0].ID, check.ID)
	if err != nil || cs == nil {
		t.Fatalf("GetCheckState failed: %v", err)
	}
	if cs.Status != "DOWN" || cs.Message == nil || *cs.Message != msg {
		t.Fatalf("CheckState mismatch: %+v", cs)
	}

	// 8. Test Rules
	ruleExprJSON := `{"CheckFailed":{"check_id":` + `1}}`
	rule, err := repo.AddRule(ctx, grp.ID, ruleExprJSON, "RemoveFromDns")
	if err != nil || rule == nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	rules, err := repo.ListRules(ctx, grp.ID)
	if err != nil || len(rules) != 1 {
		t.Fatalf("ListRules failed: %v, count=%d", err, len(rules))
	}

	// 9. Test Logs
	logEntry, err := repo.AddCheckLog(ctx, syncedIPs[0].ID, check.ID, false, &msg)
	if err != nil || logEntry == nil {
		t.Fatalf("AddCheckLog failed: %v", err)
	}
	logs, err := repo.ListRecentGroupLogs(ctx, grp.ID, nil, 10)
	if err != nil || len(logs) != 1 {
		t.Fatalf("ListRecentGroupLogs failed: %v, count=%d", err, len(logs))
	}
	if logs[0].IPAddress == nil || *logs[0].IPAddress != syncedIPs[0].IP {
		t.Fatalf("Log IP address mismatch: %+v", logs[0])
	}

	// 10. Test Notifications & Linkage
	notifChan, err := repo.CreateNotificationChannel(ctx, u.ID, "Discord Alerts", "DISCORD", `{"url":"https://discord.com/api/webhooks/test"}`)
	if err != nil || notifChan == nil {
		t.Fatalf("Failed to create notification channel: %v", err)
	}

	if err := repo.LinkGroupNotification(ctx, grp.ID, notifChan.ID, true, true); err != nil {
		t.Fatalf("LinkGroupNotification failed: %v", err)
	}

	linked, err := repo.ListGroupNotifications(ctx, grp.ID)
	if err != nil || len(linked) != 1 {
		t.Fatalf("ListGroupNotifications failed: %v, count=%d", err, len(linked))
	}
	if linked[0].Channel.Name != "Discord Alerts" || !linked[0].GroupNotification.NotifyOnUp {
		t.Fatalf("Linked notification mismatch: %+v", linked[0])
	}

	// 11. Test Anomalies
	if err := repo.AddGroupAnomaly(ctx, grp.ID, "8.8.8.8"); err != nil {
		t.Fatalf("AddGroupAnomaly failed: %v", err)
	}
	anomalies, err := repo.ListGroupAnomalies(ctx, grp.ID)
	if err != nil || len(anomalies) != 1 || anomalies[0] != "8.8.8.8" {
		t.Fatalf("ListGroupAnomalies failed: %v, list=%+v", err, anomalies)
	}
	if err := repo.DeleteGroupAnomaly(ctx, grp.ID, "8.8.8.8"); err != nil {
		t.Fatalf("DeleteGroupAnomaly failed: %v", err)
	}
	anomaliesAfter, _ := repo.ListGroupAnomalies(ctx, grp.ID)
	if len(anomaliesAfter) != 0 {
		t.Fatalf("Expected 0 anomalies after delete, got %d", len(anomaliesAfter))
	}
}
