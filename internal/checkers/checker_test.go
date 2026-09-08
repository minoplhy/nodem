package checkers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"node_monitor_go/internal/db"
)

func TestCheckTCP(t *testing.T) {
	// Start mock TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start TCP listener: %v", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.ParseUint(portStr, 10, 16)

	ctx := context.Background()
	checker := NewNodeChecker()

	// 1. Success case
	res := checker.RunCheck(ctx, "127.0.0.1", &db.CheckConfig{
		Protocol: "TCP",
		Port:     uint16(port),
	})
	if !res.Success {
		t.Fatalf("Expected TCP check to succeed on open port, got: %+v", res)
	}

	// 2. Closed port failure case
	closedPort := uint16(59999)
	resFail := checker.RunCheck(ctx, "127.0.0.1", &db.CheckConfig{
		Protocol: "TCP",
		Port:     closedPort,
	})
	if resFail.Success {
		t.Fatalf("Expected TCP check to fail on closed port, got success")
	}
}

func TestCheckHTTP(t *testing.T) {
	// Start mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "OK")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	port, _ := strconv.ParseUint(portStr, 10, 16)

	ctx := context.Background()
	checker := NewNodeChecker()

	// 1. Health check success
	path := "/health"
	res := checker.RunCheck(ctx, host, &db.CheckConfig{
		Protocol: "HTTP",
		Port:     uint16(port),
		Path:     &path,
	})
	if !res.Success {
		t.Fatalf("Expected HTTP check to succeed, got: %+v", res)
	}

	// 2. Health check failure
	failPath := "/error"
	resFail := checker.RunCheck(ctx, host, &db.CheckConfig{
		Protocol: "HTTP",
		Port:     uint16(port),
		Path:     &failPath,
	})
	if resFail.Success {
		t.Fatalf("Expected HTTP check to fail on 503, got success")
	}
}
