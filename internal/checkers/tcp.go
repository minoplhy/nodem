package checkers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"
)

func checkTCP(ctx context.Context, ip net.IP, port uint16) CheckResult {
	addr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
	slog.Debug("Checker (TCP): connecting to socket", "addr", addr)

	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			msg := "TCP connection timed out"
			return CheckResult{Success: false, Message: &msg}
		}
		msg := fmt.Sprintf("TCP connection failed: %v", err)
		return CheckResult{Success: false, Message: &msg}
	}
	defer conn.Close()

	msg := "TCP connection established"
	return CheckResult{Success: true, Message: &msg}
}
