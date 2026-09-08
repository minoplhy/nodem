package checkers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"
	"time"
)

func checkUDP(ctx context.Context, ip net.IP, port uint16) CheckResult {
	raddr := &net.UDPAddr{
		IP:   ip,
		Port: int(port),
	}

	var laddr *net.UDPAddr
	if ip.To4() != nil {
		laddr = &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	} else {
		laddr = &net.UDPAddr{IP: net.IPv6zero, Port: 0}
	}

	slog.Debug("Checker (UDP): binding and connecting", "laddr", laddr.String(), "raddr", raddr.String())

	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		msg := fmt.Sprintf("UDP socket connect failed: %v", err)
		return CheckResult{Success: false, Message: &msg}
	}
	defer conn.Close()

	// Send dummy payload (4 bytes zero)
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		msg := fmt.Sprintf("UDP send failed: %v", err)
		return CheckResult{Success: false, Message: &msg}
	}

	// Wait 500ms for ICMP port unreachable
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(err.Error(), "connection refused") {
			msg := "UDP port unreachable (ConnectionRefused)"
			return CheckResult{Success: false, Message: &msg}
		}
	}

	msg := "UDP probe sent successfully"
	return CheckResult{Success: true, Message: &msg}
}
