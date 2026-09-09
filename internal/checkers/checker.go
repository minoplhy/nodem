package checkers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/minoplhy/nodem/internal/db"
)

// CheckResult represents the outcome of a health check probe.
type CheckResult struct {
	Success bool    `json:"success"`
	Message *string `json:"message"`
}

// Checker defines the interface for running node health checks.
type Checker interface {
	RunCheck(ctx context.Context, ip string, config *db.CheckConfig) CheckResult
}

// NodeChecker coordinates protocol-specific health checks (TCP, UDP, HTTP, HTTPS).
type NodeChecker struct{}

// NewNodeChecker creates a new NodeChecker instance.
func NewNodeChecker() *NodeChecker {
	return &NodeChecker{}
}

// RunCheck validates target IP and delegates to the appropriate protocol checker.
func (n *NodeChecker) RunCheck(ctx context.Context, ip string, config *db.CheckConfig) CheckResult {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		msg := fmt.Sprintf("Invalid target IP address '%s'", ip)
		return CheckResult{Success: false, Message: &msg}
	}

	protocol := strings.ToUpper(config.Protocol)
	slog.Debug("Checker: running check", "protocol", protocol, "ip", ip, "port", config.Port)

	var res CheckResult
	switch protocol {
	case "TCP":
		res = checkTCP(ctx, parsedIP, config.Port)
	case "UDP":
		res = checkUDP(ctx, parsedIP, config.Port)
	case "HTTP", "HTTPS":
		res = checkHTTPHTTPS(ctx, parsedIP, config)
	default:
		msg := fmt.Sprintf("Unsupported protocol '%s'", config.Protocol)
		res = CheckResult{Success: false, Message: &msg}
	}

	var msgStr string
	if res.Message != nil {
		msgStr = *res.Message
	}
	slog.Debug("Checker: check finished", "protocol", protocol, "ip", ip, "port", config.Port, "success", res.Success, "message", msgStr)
	return res
}
