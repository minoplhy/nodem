package dns_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/provider"
	"github.com/minoplhy/nodem/internal/provider/dns"
)

func TestCloudflare_UpdateHTTPSRecord_Create(t *testing.T) {
	var postReceived bool
	var capturedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") {
			// No existing record
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []any{},
			})
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/dns_records") {
			postReceived = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &capturedPayload)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"id": "rec-12345",
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	// 32-char hex string as zone ID
	zoneID := "0123456789abcdef0123456789abcdef"
	p := dns.NewCloudflareProvider("test-token", zoneID)
	p.BaseURL = server.URL

	params := provider.HTTPSRecordParams{
		Domain:    "app.example.com",
		Base64ECH: "AED+TESTING==",
		TTL:       300,
		ALPN:      []string{"h2", "h3"},
	}

	err := p.UpdateHTTPSRecord(context.Background(), params)
	if err != nil {
		t.Fatalf("UpdateHTTPSRecord failed: %v", err)
	}

	if !postReceived {
		t.Fatal("expected POST request to create record, but none received")
	}

	// Verify top-level content field is NOT present
	if _, hasContent := capturedPayload["content"]; hasContent {
		t.Errorf("expected no 'content' field in Cloudflare HTTPS payload, got %v", capturedPayload["content"])
	}

	// Verify required structured data fields
	dataRaw, ok := capturedPayload["data"]
	if !ok {
		t.Fatalf("expected 'data' object in Cloudflare HTTPS payload, got: %+v", capturedPayload)
	}

	dataMap, ok := dataRaw.(map[string]any)
	if !ok {
		t.Fatalf("'data' field is not a JSON object: %+v", dataRaw)
	}

	if int(dataMap["priority"].(float64)) != 1 {
		t.Errorf("expected data.priority == 1, got %v", dataMap["priority"])
	}

	if dataMap["target"] != "." {
		t.Errorf("expected data.target == '.', got %v", dataMap["target"])
	}

	val, ok := dataMap["value"].(string)
	if !ok || !strings.Contains(val, `ech="AED+TESTING=="`) || !strings.Contains(val, `alpn="h2,h3"`) {
		t.Errorf("unexpected data.value: %v", dataMap["value"])
	}
}

func TestCloudflare_UpdateHTTPSRecord_Update(t *testing.T) {
	var putReceived bool
	var capturedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") {
			// Existing record present
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": []map[string]any{
					{"id": "rec-existing-999"},
				},
			})
			return
		}

		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/dns_records/rec-existing-999") {
			putReceived = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &capturedPayload)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]any{"id": "rec-existing-999"},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	zoneID := "0123456789abcdef0123456789abcdef"
	p := dns.NewCloudflareProvider("test-token", zoneID)
	p.BaseURL = server.URL

	params := provider.HTTPSRecordParams{
		Domain:    "app.example.com",
		Base64ECH: "AED+ROTATED==",
		TTL:       600,
	}

	err := p.UpdateHTTPSRecord(context.Background(), params)
	if err != nil {
		t.Fatalf("UpdateHTTPSRecord failed: %v", err)
	}

	if !putReceived {
		t.Fatal("expected PUT request to update record, but none received")
	}

	if _, hasContent := capturedPayload["content"]; hasContent {
		t.Errorf("expected no 'content' field in Cloudflare HTTPS payload, got %v", capturedPayload["content"])
	}

	dataMap, ok := capturedPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid 'data' object: %+v", capturedPayload)
	}

	if int(dataMap["priority"].(float64)) != 1 {
		t.Errorf("expected priority 1, got %v", dataMap["priority"])
	}
	if dataMap["target"] != "." {
		t.Errorf("expected target '.', got %v", dataMap["target"])
	}
	if val, _ := dataMap["value"].(string); !strings.Contains(val, `ech="AED+ROTATED=="`) {
		t.Errorf("expected updated ECH in value, got %v", val)
	}
}

func TestCloudflare_UpdateHTTPSRecord_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []any{},
			})
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":9101,"message":"priority is a required data field."}]}`))
	}))
	defer server.Close()

	zoneID := "0123456789abcdef0123456789abcdef"
	p := dns.NewCloudflareProvider("test-token", zoneID)
	p.BaseURL = server.URL

	err := p.UpdateHTTPSRecord(context.Background(), provider.HTTPSRecordParams{
		Domain: "app.example.com",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cloudflare create HTTPS record failed") {
		t.Errorf("unexpected error format: %v", err)
	}
}

func TestCloudflare_ZoneLookupAndRecordLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Zone lookup
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/zones") && r.URL.Query().Get("name") == "example.com" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{"id": "resolved-zone-id"},
				},
			})
			return
		}

		// Delete record query (has content parameter)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") && r.URL.Query().Get("content") != "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{"id": "del-rec-1"},
				},
			})
			return
		}

		// List records (no content parameter)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") && r.URL.Query().Get("name") == "node1.example.com" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{"type": "A", "content": "198.51.100.1"},
					{"type": "AAAA", "content": "2001:db8::1"},
				},
			})
			return
		}

		// Add record
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/dns_records") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}

		// Delete record action
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/dns_records/del-rec-1") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	ctx := context.Background()
	// Using human domain name to exercise getZoneID lookup
	p := dns.NewCloudflareProvider("test-token", "example.com")
	p.BaseURL = server.URL

	// 1. List
	ips, err := p.ListRecords(ctx, "node1.example.com")
	if err != nil {
		t.Fatalf("ListRecords failed: %v", err)
	}
	if len(ips) != 2 || ips[0] != "198.51.100.1" || ips[1] != "2001:db8::1" {
		t.Errorf("unexpected ips: %v", ips)
	}

	// 2. Add
	if err := p.AddRecord(ctx, "node1.example.com", "198.51.100.2", "A"); err != nil {
		t.Fatalf("AddRecord failed: %v", err)
	}

	// 3. Delete
	if err := p.DeleteRecord(ctx, "node1.example.com", "198.51.100.1", "A"); err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}
}
