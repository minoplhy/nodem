package provider_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/minoplhy/nodem/internal/provider"
)

func TestHTTPSRecordSerialization(t *testing.T) {
	params := provider.HTTPSRecordParams{
		Domain:     "app.example.com",
		Base64ECH:  "AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw==",
		PublicName: "cover.example.com",
		TTL:        300,
		ALPN:       []string{"h2", "h3"},
		IPv4Hint:   []string{"198.51.100.1"},
	}

	// RFC 9460 string
	rfcStr := params.ToRFC9460String()
	if !strings.Contains(rfcStr, "1 .") {
		t.Errorf("expected priority and target '1 .', got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `alpn="h2,h3"`) {
		t.Errorf("expected alpn=\"h2,h3\", got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `ech="AED+DQBIAAgABQAQAAwAAgABAAIAAQAAAAECAw=="`) {
		t.Errorf("expected ech param, got: %s", rfcStr)
	}
	if !strings.Contains(rfcStr, `ipv4hint="198.51.100.1"`) {
		t.Errorf("expected ipv4hint, got: %s", rfcStr)
	}

	// Technitium Pipe-separated syntax
	techStr := params.ToTechnitiumParams()
	if !strings.Contains(techStr, "alpn|h2,h3") {
		t.Errorf("expected Technitium alpn, got: %s", techStr)
	}
	if !strings.Contains(techStr, "ech|") {
		t.Errorf("expected Technitium ech hex, got: %s", techStr)
	}
}

type recordCall struct {
	Name       string
	IP         string
	RecordType string
}

type mockDNSProvider struct {
	records     []string
	addCalls    []recordCall
	deleteCalls []recordCall
	addErr      error
	deleteErr   error
}

func (m *mockDNSProvider) ListRecords(ctx context.Context, recordName string) ([]string, error) {
	return m.records, nil
}

func (m *mockDNSProvider) AddRecord(ctx context.Context, recordName, ip, recordType string) error {
	m.addCalls = append(m.addCalls, recordCall{Name: recordName, IP: ip, RecordType: recordType})
	if m.addErr != nil {
		return m.addErr
	}
	m.records = append(m.records, ip)
	return nil
}

func (m *mockDNSProvider) DeleteRecord(ctx context.Context, recordName, ip, recordType string) error {
	m.deleteCalls = append(m.deleteCalls, recordCall{Name: recordName, IP: ip, RecordType: recordType})
	if m.deleteErr != nil {
		return m.deleteErr
	}
	var remaining []string
	for _, r := range m.records {
		if r != ip {
			remaining = append(remaining, r)
		}
	}
	m.records = remaining
	return nil
}

func (m *mockDNSProvider) UpdateHTTPSRecord(ctx context.Context, params provider.HTTPSRecordParams) error {
	return nil
}

func TestRecordTypeForIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.1", "A"},
		{"1.1.1.1", "A"},
		{"10.0.0.1", "A"},
		{"::1", "AAAA"},
		{"2001:db8::1", "AAAA"},
		{"fe80::1", "AAAA"},
	}

	for _, tt := range tests {
		got := provider.RecordTypeForIP(tt.ip)
		if got != tt.expected {
			t.Errorf("RecordTypeForIP(%s) = %s, expected %s", tt.ip, got, tt.expected)
		}
	}
}

func TestRecordEventTrigger_Uptime(t *testing.T) {
	ctx := context.Background()
	recordName := "nodes.example.com"

	t.Run("IPv4 transition to UP triggers AddRecord with type A", func(t *testing.T) {
		mock := &mockDNSProvider{}
		currentRecords := []string{}
		ip := "198.51.100.10"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Errorf("expected updated to be true")
		}
		if len(mock.addCalls) != 1 {
			t.Fatalf("expected 1 AddRecord call, got %d", len(mock.addCalls))
		}
		call := mock.addCalls[0]
		if call.Name != recordName || call.IP != ip || call.RecordType != "A" {
			t.Errorf("unexpected AddRecord call: %+v", call)
		}
	})

	t.Run("IPv6 transition to UP triggers AddRecord with type AAAA", func(t *testing.T) {
		mock := &mockDNSProvider{}
		currentRecords := []string{}
		ip := "2001:db8::cafe"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Errorf("expected updated to be true")
		}
		if len(mock.addCalls) != 1 {
			t.Fatalf("expected 1 AddRecord call, got %d", len(mock.addCalls))
		}
		call := mock.addCalls[0]
		if call.Name != recordName || call.IP != ip || call.RecordType != "AAAA" {
			t.Errorf("unexpected AddRecord call: %+v", call)
		}
	})

	t.Run("IP already present on UP event is idempotent no-op", func(t *testing.T) {
		mock := &mockDNSProvider{records: []string{"198.51.100.10"}}
		currentRecords := []string{"198.51.100.10"}
		ip := "198.51.100.10"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated {
			t.Errorf("expected updated to be false for already present IP")
		}
		if len(mock.addCalls) != 0 {
			t.Errorf("expected no AddRecord calls, got %d", len(mock.addCalls))
		}
	})
}

func TestRecordEventTrigger_Downtime(t *testing.T) {
	ctx := context.Background()
	recordName := "nodes.example.com"

	t.Run("IPv4 transition to DOWN triggers DeleteRecord with type A", func(t *testing.T) {
		mock := &mockDNSProvider{records: []string{"198.51.100.10"}}
		currentRecords := []string{"198.51.100.10"}
		ip := "198.51.100.10"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Errorf("expected updated to be true")
		}
		if len(mock.deleteCalls) != 1 {
			t.Fatalf("expected 1 DeleteRecord call, got %d", len(mock.deleteCalls))
		}
		call := mock.deleteCalls[0]
		if call.Name != recordName || call.IP != ip || call.RecordType != "A" {
			t.Errorf("unexpected DeleteRecord call: %+v", call)
		}
	})

	t.Run("IPv6 transition to DOWN triggers DeleteRecord with type AAAA", func(t *testing.T) {
		mock := &mockDNSProvider{records: []string{"2001:db8::cafe"}}
		currentRecords := []string{"2001:db8::cafe"}
		ip := "2001:db8::cafe"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Errorf("expected updated to be true")
		}
		if len(mock.deleteCalls) != 1 {
			t.Fatalf("expected 1 DeleteRecord call, got %d", len(mock.deleteCalls))
		}
		call := mock.deleteCalls[0]
		if call.Name != recordName || call.IP != ip || call.RecordType != "AAAA" {
			t.Errorf("unexpected DeleteRecord call: %+v", call)
		}
	})

	t.Run("IP already absent on DOWN event is idempotent no-op", func(t *testing.T) {
		mock := &mockDNSProvider{records: []string{"198.51.100.99"}}
		currentRecords := []string{"198.51.100.99"}
		ip := "198.51.100.10"

		updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, ip, currentRecords)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated {
			t.Errorf("expected updated to be false for already absent IP")
		}
		if len(mock.deleteCalls) != 0 {
			t.Errorf("expected no DeleteRecord calls, got %d", len(mock.deleteCalls))
		}
	})
}

func TestRecordEventTrigger_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	recordName := "nodes.example.com"
	expectedErr := errors.New("provider API connection timeout")

	t.Run("AddRecord failure propagates error", func(t *testing.T) {
		mock := &mockDNSProvider{addErr: expectedErr}
		_, err := provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, "1.2.3.4", nil)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("DeleteRecord failure propagates error", func(t *testing.T) {
		mock := &mockDNSProvider{deleteErr: expectedErr}
		_, err := provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, "1.2.3.4", []string{"1.2.3.4"})
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Unknown event returns error", func(t *testing.T) {
		mock := &mockDNSProvider{}
		_, err := provider.HandleRecordEvent(ctx, mock, "INVALID_EVENT", recordName, "1.2.3.4", nil)
		if err == nil {
			t.Errorf("expected error for invalid event, got nil")
		}
	})
}

func TestRecordEventTrigger_FullLifecycleTransitions(t *testing.T) {
	ctx := context.Background()
	recordName := "lb.example.com"
	nodeIP := "203.0.113.50"
	mock := &mockDNSProvider{}

	// Step 1: Initial state - node offline, not in DNS
	currentDNS, _ := mock.ListRecords(ctx, recordName)
	if len(currentDNS) != 0 {
		t.Fatalf("expected empty DNS records initially")
	}

	// Step 2: Node passes health check -> Uptime Event Trigger
	updated, err := provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, nodeIP, currentDNS)
	if err != nil || !updated {
		t.Fatalf("Uptime trigger failed: updated=%v, err=%v", updated, err)
	}

	// Step 3: Node remains healthy -> Subsequent Uptime Event Trigger (Idempotency)
	currentDNS, _ = mock.ListRecords(ctx, recordName)
	updated, err = provider.HandleRecordEvent(ctx, mock, provider.EventUptime, recordName, nodeIP, currentDNS)
	if err != nil || updated {
		t.Fatalf("Subsequent uptime trigger should not update: updated=%v, err=%v", updated, err)
	}

	// Step 4: Node fails health check -> Downtime Event Trigger
	updated, err = provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, nodeIP, currentDNS)
	if err != nil || !updated {
		t.Fatalf("Downtime trigger failed: updated=%v, err=%v", updated, err)
	}

	// Step 5: Node remains down -> Subsequent Downtime Event Trigger (Idempotency)
	currentDNS, _ = mock.ListRecords(ctx, recordName)
	updated, err = provider.HandleRecordEvent(ctx, mock, provider.EventDowntime, recordName, nodeIP, currentDNS)
	if err != nil || updated {
		t.Fatalf("Subsequent downtime trigger should not update: updated=%v, err=%v", updated, err)
	}

	// Verify total call count: exactly 1 AddRecord and 1 DeleteRecord throughout entire lifecycle
	if len(mock.addCalls) != 1 {
		t.Errorf("expected exactly 1 AddRecord call, got %d", len(mock.addCalls))
	}
	if len(mock.deleteCalls) != 1 {
		t.Errorf("expected exactly 1 DeleteRecord call, got %d", len(mock.deleteCalls))
	}
}
