package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/minoplhy/nodem/internal/agent"
)

func TestResolveEndpointURL(t *testing.T) {
	testCases := []struct {
		server   string
		endpoint string
		expected string
	}{
		{"http://localhost:8080", "/api/v1/agent/ech/sync", "http://localhost:8080/api/v1/agent/ech/sync"},
		{"http://localhost:8080/", "/api/v1/agent/ech/sync", "http://localhost:8080/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/api", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"https://monitor.example.com/subpath/api/", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"monitor.example.com/subpath", "/api/v1/agent/ech/sync", "https://monitor.example.com/subpath/api/v1/agent/ech/sync"},
		{"localhost:8080/subpath", "/api/v1/agent/ech/ack", "http://localhost:8080/subpath/api/v1/agent/ech/ack"},
	}

	for _, tc := range testCases {
		got := agent.ResolveEndpointURL(tc.server, tc.endpoint)
		if got != tc.expected {
			t.Errorf("ResolveEndpointURL(%q, %q) = %q, expected %q", tc.server, tc.endpoint, got, tc.expected)
		}
	}
}

func TestGetEnvHelpers(t *testing.T) {
	os.Setenv("TEST_AGENT_STR", "custom_val")
	os.Setenv("TEST_AGENT_INT", "42")
	defer os.Unsetenv("TEST_AGENT_STR")
	defer os.Unsetenv("TEST_AGENT_INT")

	if val := agent.GetEnv("TEST_AGENT_STR", "default"); val != "custom_val" {
		t.Errorf("expected custom_val, got %s", val)
	}
	if val := agent.GetEnv("TEST_AGENT_NON_EXISTENT", "fallback"); val != "fallback" {
		t.Errorf("expected fallback, got %s", val)
	}

	if val := agent.GetEnvInt("TEST_AGENT_INT", 10); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
	if val := agent.GetEnvInt("TEST_AGENT_NON_EXISTENT_INT", 99); val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
}

func TestAgentEnvFileLoading(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env.agent")
	envContent := "ECH_SERVER=https://custom-monitor.internal\nECH_TOKEN=secret_agent_token\nECH_PROXY=caddy\nECH_INTERVAL=120\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed writing test env file: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	_ = os.Chdir(tempDir)

	os.Unsetenv("ECH_SERVER")
	os.Unsetenv("ECH_TOKEN")
	os.Unsetenv("ECH_PROXY")
	os.Unsetenv("ECH_INTERVAL")

	if err := godotenv.Load(".env.agent"); err != nil {
		t.Fatalf("failed loading .env.agent: %v", err)
	}

	if val := os.Getenv("ECH_SERVER"); val != "https://custom-monitor.internal" {
		t.Errorf("expected https://custom-monitor.internal, got %s", val)
	}
	if val := os.Getenv("ECH_TOKEN"); val != "secret_agent_token" {
		t.Errorf("expected secret_agent_token, got %s", val)
	}
	if val := os.Getenv("ECH_PROXY"); val != "caddy" {
		t.Errorf("expected caddy, got %s", val)
	}
	if val := os.Getenv("ECH_INTERVAL"); val != "120" {
		t.Errorf("expected 120, got %s", val)
	}
}
