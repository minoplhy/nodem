package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"node_monitor_go/internal/db/sqlite"
)

func setupTestServer(t *testing.T) (*httptest.Server, *sqlite.SqliteRepository, func()) {
	tmpDir, err := os.MkdirTemp("", "api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	dbFile := filepath.Join(tmpDir, "test.db")

	repo, err := sqlite.New(dbFile)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open test db: %v", err)
	}

	if err := repo.InitDB(context.Background()); err != nil {
		_ = repo.Close()
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init test db: %v", err)
	}

	state := &AppState{
		Repo:           repo,
		BootstrapToken: "TESTBOOT1234",
		BasePath:       "",
	}

	router := BuildRouter(state)
	ts := httptest.NewServer(router)

	cleanup := func() {
		ts.Close()
		_ = repo.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return ts, repo, cleanup
}

func TestAPIWorkflow(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Setup Status (should require setup initially)
	resp, err := client.Get(ts.URL + "/api/setup-status")
	if err != nil {
		t.Fatalf("setup-status failed: %v", err)
	}
	var statusData map[string]bool
	_ = json.NewDecoder(resp.Body).Decode(&statusData)
	resp.Body.Close()
	if !statusData["setup_required"] {
		t.Fatalf("Expected setup_required = true, got false")
	}

	// 2. Setup with invalid token
	invalidSetupPayload := map[string]string{
		"token":    "WRONGTOKEN",
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ := json.Marshal(invalidSetupPayload)
	resp, err = client.Post(ts.URL+"/api/setup", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 on wrong token, got %v (err=%v)", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 3. Setup with valid token
	validSetupPayload := map[string]string{
		"token":    "TESTBOOT1234",
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ = json.Marshal(validSetupPayload)
	resp, err = client.Post(ts.URL+"/api/setup", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on valid setup, got %v (err=%v)", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 4. Setup Status again (should NOT require setup now)
	resp, err = client.Get(ts.URL + "/api/setup-status")
	if err != nil {
		t.Fatalf("setup-status failed: %v", err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&statusData)
	resp.Body.Close()
	if statusData["setup_required"] {
		t.Fatalf("Expected setup_required = false now, got true")
	}

	// 5. Login
	loginPayload := map[string]string{
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ = json.Marshal(loginPayload)
	resp, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on login, got %v (err=%v)", resp.StatusCode, err)
	}

	cookies := resp.Cookies()
	resp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_id" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("Expected session_id cookie set on login")
	}

	// Helper for authenticated requests
	doAuthReq := func(method, path string, body []byte) *http.Response {
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
		req.AddCookie(sessionCookie)
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		r, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request %s %s failed: %v", method, path, err)
		}
		return r
	}

	// 6. Test /api/me
	meResp := doAuthReq("GET", "/api/me", nil)
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from /api/me, got %d", meResp.StatusCode)
	}
	var meUser UserResponse
	_ = json.NewDecoder(meResp.Body).Decode(&meUser)
	meResp.Body.Close()
	if meUser.Username != "admin" || meUser.Role != "Admin" {
		t.Fatalf("Unexpected user from /api/me: %+v", meUser)
	}

	// 7. Create DNS Provider
	provPayload := map[string]string{
		"name":          "CF Test",
		"provider_type": "Cloudflare",
		"api_url":       "https://api.cloudflare.com",
		"token":         "cftoken",
		"zone":          "example.com",
	}
	pb, _ := json.Marshal(provPayload)
	provResp := doAuthReq("POST", "/api/providers", pb)
	if provResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from POST /api/providers, got %d", provResp.StatusCode)
	}
	var createdProv map[string]interface{}
	_ = json.NewDecoder(provResp.Body).Decode(&createdProv)
	provResp.Body.Close()
	provID := int64(createdProv["id"].(float64))

	// 8. Create Target Group
	grpPayload := map[string]interface{}{
		"name":                "Primary Nodes",
		"dns_record":          "node.example.com",
		"dns_provider_id":     provID,
		"check_interval_secs": 60,
	}
	gb, _ := json.Marshal(grpPayload)
	grpResp := doAuthReq("POST", "/api/groups", gb)
	if grpResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from POST /api/groups, got %d", grpResp.StatusCode)
	}
	var createdGrp map[string]interface{}
	_ = json.NewDecoder(grpResp.Body).Decode(&createdGrp)
	grpResp.Body.Close()
	grpID := int64(createdGrp["id"].(float64))

	// 9. Add IP to Target Group
	ipPayload := map[string]string{"ip": "192.0.2.1"}
	ib, _ := json.Marshal(ipPayload)
	ipResp := doAuthReq("POST", "/api/groups/"+jsonNumber(grpID)+"/ips", ib)
	if ipResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from POST /api/groups/:id/ips, got %d", ipResp.StatusCode)
	}
	ipResp.Body.Close()

	// 10. Toggle Group
	togglePayload := map[string]bool{"enabled": false}
	tb, _ := json.Marshal(togglePayload)
	toggleResp := doAuthReq("POST", "/api/groups/"+jsonNumber(grpID)+"/toggle", tb)
	if toggleResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from POST /api/groups/:id/toggle, got %d", toggleResp.StatusCode)
	}
	toggleResp.Body.Close()

	// 11. Get Groups Status
	statusResp := doAuthReq("GET", "/api/groups/status", nil)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from GET /api/groups/status, got %d", statusResp.StatusCode)
	}
	var unifiedStatus UnifiedGroupsStatusResponse
	_ = json.NewDecoder(statusResp.Body).Decode(&unifiedStatus)
	statusResp.Body.Close()
	if len(unifiedStatus.Groups) != 1 || unifiedStatus.Groups[0].Enabled != false {
		t.Fatalf("Unexpected unified status: %+v", unifiedStatus)
	}

	// 12. Logout
	logoutResp := doAuthReq("POST", "/api/logout", nil)
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 from POST /api/logout, got %d", logoutResp.StatusCode)
	}
	logoutResp.Body.Close()

	// 13. Verify /api/me is unauthorized after logout
	meAfterResp := doAuthReq("GET", "/api/me", nil)
	if meAfterResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 from /api/me after logout, got %d", meAfterResp.StatusCode)
	}
	meAfterResp.Body.Close()
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestStaticAssetsAndBasePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "static_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbFile := filepath.Join(tmpDir, "test.db")
	repo, err := sqlite.New(dbFile)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer repo.Close()

	if err := repo.InitDB(context.Background()); err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}

	// Test with custom BASE_PATH = "/monitor"
	state := &AppState{
		Repo:           repo,
		BootstrapToken: "TESTBOOT1234",
		BasePath:       "/monitor",
	}

	router := BuildRouter(state)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Client that does NOT follow redirects automatically so we can test 308/307
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Bare /monitor should redirect to /monitor/ with 308 Permanent Redirect
	resp, err := noRedirectClient.Get(ts.URL + "/monitor")
	if err != nil {
		t.Fatalf("GET /monitor failed: %v", err)
	}
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Errorf("Expected 308 on /monitor, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "/monitor/" {
		t.Errorf("Expected Location /monitor/, got %s", loc)
	}
	resp.Body.Close()

	// 2. Bare / should redirect to /monitor/ with 307
	resp, err = noRedirectClient.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Expected 307 on /, got %d", resp.StatusCode)
	}
	loc = resp.Header.Get("Location")
	if loc != "/monitor/" {
		t.Errorf("Expected Location /monitor/, got %s", loc)
	}
	resp.Body.Close()

	// 3. Normal client for GET /monitor/
	client := ts.Client()
	resp, err = client.Get(ts.URL + "/monitor/")
	if err != nil {
		t.Fatalf("GET /monitor/ failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on /monitor/, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(body)

	// Verify <base href="/monitor/"> and window.__BASE_PATH__ = "/monitor/";
	if !strings.Contains(bodyStr, `<base href="/monitor/">`) {
		t.Errorf("Expected <base href=\"/monitor/\"> in HTML, got:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, `window.__BASE_PATH__ = "/monitor/";`) {
		t.Errorf("Expected window.__BASE_PATH__ in HTML, got:\n%s", bodyStr)
	}
	// Verify rewritten asset paths
	if !strings.Contains(bodyStr, `src="/monitor/assets/index-`) {
		t.Errorf("Expected rewritten src=\"/monitor/assets/index- in HTML, got:\n%s", bodyStr)
	}

	// 4. Dynamically extract JS and CSS asset paths from HTML
	var jsAssetPath, cssAssetPath string
	if idx := strings.Index(bodyStr, `src="/monitor/assets/`); idx != -1 {
		endIdx := strings.Index(bodyStr[idx+len(`src="`):], `"`)
		if endIdx != -1 {
			jsAssetPath = bodyStr[idx+len(`src="`) : idx+len(`src="`)+endIdx]
		}
	}
	if jsAssetPath == "" {
		t.Fatalf("Could not find js asset path in HTML")
	}

	if idx := strings.Index(bodyStr, `href="/monitor/assets/`); idx != -1 {
		endIdx := strings.Index(bodyStr[idx+len(`href="`):], `"`)
		if endIdx != -1 {
			cssAssetPath = bodyStr[idx+len(`href="`) : idx+len(`href="`)+endIdx]
		}
	}

	// Test nested static JS asset
	resp, err = client.Get(ts.URL + jsAssetPath)
	if err != nil {
		t.Fatalf("GET %s failed: %v", jsAssetPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 for nested asset %s, got %d", jsAssetPath, resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/javascript") && !strings.HasPrefix(ctype, "application/javascript") {
		t.Errorf("Expected javascript Content-Type for .js asset, got: %q (MUST NOT BE text/plain)", ctype)
	}
	resp.Body.Close()

	// 5. Test direct outer static JS asset (without /monitor prefix)
	outerAssetPath := strings.TrimPrefix(jsAssetPath, "/monitor")
	resp, err = client.Get(ts.URL + outerAssetPath)
	if err != nil {
		t.Fatalf("GET %s failed: %v", outerAssetPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 for direct outer asset %s, got %d", outerAssetPath, resp.StatusCode)
	}
	ctype = resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/javascript") && !strings.HasPrefix(ctype, "application/javascript") {
		t.Errorf("Expected javascript Content-Type for outer .js asset, got: %q (MUST NOT BE text/plain)", ctype)
	}
	resp.Body.Close()

	// 6. Test nested CSS asset
	if cssAssetPath != "" {
		resp, err = client.Get(ts.URL + cssAssetPath)
		if err != nil {
			t.Fatalf("GET %s failed: %v", cssAssetPath, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 for CSS asset %s, got %d", cssAssetPath, resp.StatusCode)
		}
		ctype = resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ctype, "text/css") {
			t.Errorf("Expected text/css Content-Type for .css asset, got: %q", ctype)
		}
		resp.Body.Close()
	}

	// 7. Non-existent API route should return 404 JSON, NOT HTML
	resp, err = client.Get(ts.URL + "/monitor/api/nonexistent_route")
	if err != nil {
		t.Fatalf("GET /monitor/api/nonexistent_route failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for nonexistent API route, got %d", resp.StatusCode)
	}
	ctype = resp.Header.Get("Content-Type")
	if !strings.Contains(ctype, "application/json") {
		t.Errorf("Expected application/json for API 404, got %q", ctype)
	}
	resp.Body.Close()
}

func TestGroupIPsPayloadFormats(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Setup
	setupPayload := map[string]string{
		"token":    "TESTBOOT1234",
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ := json.Marshal(setupPayload)
	resp, err := client.Post(ts.URL+"/api/setup", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Setup failed: %v", err)
	}
	resp.Body.Close()

	// 2. Login
	loginPayload := map[string]string{"username": "admin", "password": "secretpassword"}
	b, _ = json.Marshal(loginPayload)
	resp, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed: %v", err)
	}
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			sessionCookie = c
			break
		}
	}
	resp.Body.Close()

	doReq := func(method, path string, body []byte) *http.Response {
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if sessionCookie != nil {
			req.AddCookie(sessionCookie)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", method, path, err)
		}
		return res
	}

	// Create Provider & Group via authenticated API
	pResp := doReq("POST", "/api/providers", []byte(`{
		"name": "CF Test",
		"provider_type": "Cloudflare",
		"api_url": "https://api.cloudflare.com",
		"token": "tok",
		"zone": "example.com"
	}`))
	if pResp.StatusCode != http.StatusOK {
		t.Fatalf("Create provider failed: %d", pResp.StatusCode)
	}
	var createdProv map[string]interface{}
	_ = json.NewDecoder(pResp.Body).Decode(&createdProv)
	pResp.Body.Close()
	provID := int64(createdProv["id"].(float64))

	gResp := doReq("POST", "/api/groups", []byte(fmt.Sprintf(`{
		"name": "TestGroup",
		"dns_record": "test.example.com",
		"dns_provider_id": %d,
		"check_interval_secs": 30
	}`, provID)))
	if gResp.StatusCode != http.StatusOK {
		t.Fatalf("Create group failed: %d", gResp.StatusCode)
	}
	var createdGrp map[string]interface{}
	_ = json.NewDecoder(gResp.Body).Decode(&createdGrp)
	gResp.Body.Close()
	grpID := int64(createdGrp["id"].(float64))

	grpURL := "/api/groups/" + strconv.FormatInt(grpID, 10) + "/ips"

	// Test 1: PUT with standard object {"ips": [{"ip": "10.0.0.1", "enabled": true}]}
	r1 := doReq("PUT", grpURL, []byte(`{"ips": [{"ip": "10.0.0.1", "enabled": true}]}`))
	if r1.StatusCode != http.StatusOK {
		t.Errorf("PUT standard object failed: status %d", r1.StatusCode)
	}
	r1.Body.Close()

	// Test 2: PUT with direct array [{"ip": "10.0.0.2", "enabled": true}]
	r2 := doReq("PUT", grpURL, []byte(`[{"ip": "10.0.0.2", "enabled": true}]`))
	if r2.StatusCode != http.StatusOK {
		t.Errorf("PUT direct array failed: status %d", r2.StatusCode)
	}
	r2.Body.Close()

	// Test 3: PUT with array of strings ["10.0.0.3", "10.0.0.4"]
	r3 := doReq("PUT", grpURL, []byte(`["10.0.0.3", "10.0.0.4"]`))
	if r3.StatusCode != http.StatusOK {
		t.Errorf("PUT string array failed: status %d", r3.StatusCode)
	}
	r3.Body.Close()

	// Test 4: PUT with {"ips": ["10.0.0.5"]}
	r4 := doReq("PUT", grpURL, []byte(`{"ips": ["10.0.0.5"]}`))
	if r4.StatusCode != http.StatusOK {
		t.Errorf("PUT string list in object failed: status %d", r4.StatusCode)
	}
	r4.Body.Close()

	// Test 5: POST with standard {"ip": "10.0.0.6"}
	r5 := doReq("POST", grpURL, []byte(`{"ip": "10.0.0.6"}`))
	if r5.StatusCode != http.StatusOK {
		t.Errorf("POST standard failed: status %d", r5.StatusCode)
	}
	r5.Body.Close()

	// Test 6: POST with {"ip_address": "10.0.0.7"}
	r6 := doReq("POST", grpURL, []byte(`{"ip_address": "10.0.0.7"}`))
	if r6.StatusCode != http.StatusOK {
		t.Errorf("POST ip_address failed: status %d", r6.StatusCode)
	}
	r6.Body.Close()

	// Test 7: POST with raw string "10.0.0.8"
	r7 := doReq("POST", grpURL, []byte(`"10.0.0.8"`))
	if r7.StatusCode != http.StatusOK {
		t.Errorf("POST raw string failed: status %d", r7.StatusCode)
	}
	r7.Body.Close()

	// Test 8: POST with batch object {"ips": [{"ip": "10.0.0.9", "enabled": true}]}
	r8 := doReq("POST", grpURL, []byte(`{"ips": [{"ip": "10.0.0.9", "enabled": true}]}`))
	if r8.StatusCode != http.StatusOK {
		t.Errorf("POST batch object failed: status %d", r8.StatusCode)
	}
	r8.Body.Close()

	// Test 9: Invalid empty body
	r9 := doReq("POST", grpURL, []byte(`{}`))
	if r9.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty object POST, got %d", r9.StatusCode)
	}
	r9.Body.Close()
}

func TestSessionManagementSecurity(t *testing.T) {
	ts, repo, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Setup Admin
	setupPayload := map[string]string{
		"token":    "TESTBOOT1234",
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ := json.Marshal(setupPayload)
	resp, err := client.Post(ts.URL+"/api/setup", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Setup failed: %v", err)
	}
	resp.Body.Close()

	// 2. Normal Login (24h default)
	login24hPayload := map[string]interface{}{
		"username":    "admin",
		"password":    "secretpassword",
		"remember_me": false,
	}
	b, _ = json.Marshal(login24hPayload)
	resp, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Login 24h failed: %v", err)
	}
	var cookie24h *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			cookie24h = c
			break
		}
	}
	resp.Body.Close()
	if cookie24h == nil || cookie24h.Value == "" {
		t.Fatalf("Expected session_id cookie from 24h login")
	}

	// Verify 24 hour duration (~86400s)
	dur24h := time.Until(cookie24h.Expires)
	if dur24h < 23*time.Hour || dur24h > 25*time.Hour {
		t.Errorf("Expected ~24h expiration on normal login, got %v", dur24h)
	}

	// 3. Remember Me Login (30 days)
	login30dPayload := map[string]interface{}{
		"username":    "admin",
		"password":    "secretpassword",
		"remember_me": true,
	}
	b, _ = json.Marshal(login30dPayload)
	resp, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Login 30d failed: %v", err)
	}
	var cookie30d *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			cookie30d = c
			break
		}
	}
	resp.Body.Close()
	if cookie30d == nil || cookie30d.Value == "" {
		t.Fatalf("Expected session_id cookie from 30d login")
	}

	// Verify 30 days duration (~720h)
	dur30d := time.Until(cookie30d.Expires)
	if dur30d < 29*24*time.Hour || dur30d > 31*24*time.Hour {
		t.Errorf("Expected ~30 days expiration on remember me login, got %v", dur30d)
	}

	// 4. Test ListActiveSessions does NOT leak session cookie
	req, _ := http.NewRequest("GET", ts.URL+"/api/sessions", nil)
	req.AddCookie(cookie30d)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/sessions failed: %v", err)
	}
	rawBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// CRITICAL SECURITY ASSERTION:
	// The secret cookie values (cookie24h and cookie30d) MUST NOT appear in the JSON response!
	if strings.Contains(string(rawBody), cookie24h.Value) {
		t.Fatalf("SECURITY LEAK: Found cookie24h secret token in /api/sessions JSON: %s", string(rawBody))
	}
	if strings.Contains(string(rawBody), cookie30d.Value) {
		t.Fatalf("SECURITY LEAK: Found cookie30d secret token in /api/sessions JSON: %s", string(rawBody))
	}

	var sessionList []SessionResponse
	if err := json.Unmarshal(rawBody, &sessionList); err != nil {
		t.Fatalf("Failed to parse session list: %v", err)
	}
	if len(sessionList) < 2 {
		t.Fatalf("Expected at least 2 sessions, got %d", len(sessionList))
	}

	// Verify public IDs start with 'sess_' and is_current is true for current cookie
	var currentFound bool
	var otherSessionID string
	for _, s := range sessionList {
		if !strings.HasPrefix(s.ID, "sess_") && !strings.HasPrefix(s.SessionID, "sess_") {
			t.Errorf("Expected public ID format with 'sess_' prefix, got ID=%s SessionID=%s", s.ID, s.SessionID)
		}
		if s.IsCurrent {
			currentFound = true
		} else {
			otherSessionID = s.ID
		}
	}
	if !currentFound {
		t.Errorf("Expected one session to have is_current = true")
	}
	if otherSessionID == "" {
		t.Fatalf("Expected non-current session to revoke")
	}

	// 5. Revoke the other session using its public ID
	delReq, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions/"+otherSessionID, nil)
	delReq.AddCookie(cookie30d)
	delResp, err := client.Do(delReq)
	if err != nil || delResp.StatusCode != http.StatusOK {
		t.Fatalf("Revoke session by public ID failed: %v, status: %d", err, delResp.StatusCode)
	}
	delResp.Body.Close()

	// Verify it was removed
	req, _ = http.NewRequest("GET", ts.URL+"/api/sessions", nil)
	req.AddCookie(cookie30d)
	resp, _ = client.Do(req)
	var afterList []SessionResponse
	_ = json.NewDecoder(resp.Body).Decode(&afterList)
	resp.Body.Close()
	for _, s := range afterList {
		if s.ID == otherSessionID {
			t.Errorf("Revoked session %s still returned in list", otherSessionID)
		}
	}

	// 6. Test Garbage Collection of expired sessions
	u, _ := repo.GetUserByUsername(context.Background(), "admin")
	expiredID := "expired_token_test"
	expiredPubID := "sess_expired_test"
	pastTime := time.Now().UTC().Add(-2 * time.Hour)
	_ = repo.CreateSession(context.Background(), expiredID, expiredPubID, u.ID, pastTime, nil, nil)

	// Run cleanup
	if err := repo.CleanupExpiredSessions(context.Background()); err != nil {
		t.Fatalf("CleanupExpiredSessions failed: %v", err)
	}

	// Verify expired session is gone
	expSess, err := repo.GetSession(context.Background(), expiredID)
	if err != nil || expSess != nil {
		t.Errorf("Expected expired session to be deleted by GC, got: %+v", expSess)
	}
}

func TestRulesEndpointAndValidation(t *testing.T) {
	ts, repo, cleanup := setupTestServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Setup admin and login
	setupPayload := map[string]string{
		"token":    "TESTBOOT1234",
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ := json.Marshal(setupPayload)
	resp, err := client.Post(ts.URL+"/api/setup", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Setup failed: %v", err)
	}
	resp.Body.Close()

	loginPayload := map[string]string{
		"username": "admin",
		"password": "secretpassword",
	}
	b, _ = json.Marshal(loginPayload)
	resp, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed: %v", err)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			cookie = c
			break
		}
	}
	resp.Body.Close()
	if cookie == nil {
		t.Fatalf("Cookie was nil after login")
	}

	// 2. Create provider and group
	admin, err := repo.GetUserByUsername(context.Background(), "admin")
	if err != nil || admin == nil {
		t.Fatalf("Failed to get admin user: %v", err)
	}
	prov, err := repo.CreateProvider(context.Background(), admin.ID, "Mock Provider", "mock", "", "", "example.com")
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}
	grp, err := repo.CreateGroup(context.Background(), admin.ID, "Test Group", "test.example.com", prov.ID, 30)
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	rulesURL := fmt.Sprintf("%s/api/groups/%d/rules", ts.URL, grp.ID)

	// 3. Post rule with stringified JSON
	payloadStringified := `{"expression_json":"{\"CheckFailed\":{\"check_id\":1}}","action":"RemoveFromDns"}`
	req, _ := http.NewRequest("POST", rulesURL, strings.NewReader(payloadStringified))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to add stringified rule: %v, code: %d", err, resp.StatusCode)
	}
	var rule1 map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&rule1)
	resp.Body.Close()
	if rule1["action"] != "RemoveFromDns" {
		t.Fatalf("Expected action RemoveFromDns, got %v", rule1["action"])
	}

	// 4. Post rule with raw JSON AST object
	payloadRawObj := `{"expression_json":{"CheckHealthy":{"check_id":1}},"action":"AddToDns"}`
	req, _ = http.NewRequest("POST", rulesURL, strings.NewReader(payloadRawObj))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to add raw object rule: %v, code: %d", err, resp.StatusCode)
	}
	var rule2 map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&rule2)
	resp.Body.Close()
	if rule2["action"] != "AddToDns" {
		t.Fatalf("Expected action AddToDns, got %v", rule2["action"])
	}

	// 5. Post invalid expression syntax
	payloadInvalidExpr := `{"expression_json":{"InvalidVariant":{"check_id":1}},"action":"RemoveFromDns"}`
	req, _ = http.NewRequest("POST", rulesURL, strings.NewReader(payloadInvalidExpr))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 on invalid expression, got code: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. Post invalid action
	payloadInvalidAction := `{"expression_json":{"CheckHealthy":{"check_id":1}},"action":"BypassAll"}`
	req, _ = http.NewRequest("POST", rulesURL, strings.NewReader(payloadInvalidAction))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 on invalid action, got code: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. List rules
	req, _ = http.NewRequest("GET", rulesURL, nil)
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("ListRules failed: %v", err)
	}
	var list []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(list))
	}

	// 8. Delete rule
	r1ID := int64(rule1["id"].(float64))
	delURL := fmt.Sprintf("%s/api/groups/%d/rules/%d", ts.URL, grp.ID, r1ID)
	req, _ = http.NewRequest("DELETE", delURL, nil)
	req.AddCookie(cookie)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("DeleteRule failed: %v", err)
	}
	resp.Body.Close()
}

