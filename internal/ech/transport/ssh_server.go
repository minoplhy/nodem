package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"github.com/minoplhy/nodem/internal/db"
)

// SSHServer runs an embedded, in-process SSH service for pull agent synchronization.
type SSHServer struct {
	service *PullService
	port    uint16
	hostKey ssh.Signer
	mu      sync.Mutex
	closed  bool
}

// NewSSHServer creates a new SSHServer instance.
func NewSSHServer(service *PullService, port uint16) (*SSHServer, error) {
	// Generate an in-memory ephemeral host key for the server
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ssh host key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh signer: %w", err)
	}

	return &SSHServer{
		service: service,
		port:    port,
		hostKey: signer,
	}, nil
}

// Start listens and serves SSH pull connections until context is cancelled.
func (s *SSHServer) Start(ctx context.Context) error {
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			pubKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
			node, err := s.service.AuthenticateBySSHPublicKey(ctx, pubKeyStr, db.PullTransportSSH)
			if err != nil {
				return nil, err
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					"node_id": strconv.FormatInt(node.ID, 10),
				},
			}, nil
		},
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			// Also allow token-based authentication over SSH (token passed as user or password)
			token := conn.User()
			if token == "" || strings.EqualFold(token, "sync") || strings.EqualFold(token, "ack") {
				token = string(password)
			}
			node, err := s.service.AuthenticateByToken(ctx, token, db.PullTransportSSH)
			if err != nil {
				return nil, err
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					"node_id": strconv.FormatInt(node.ID, 10),
				},
			}, nil
		},
		NoClientAuth: false,
	}
	serverConfig.AddHostKey(s.hostKey)

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind ssh server on %s: %w", addr, err)
	}
	defer listener.Close()

	slog.Info("ECH SSH Pull server listening", "addr", addr)

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		s.closed = true
		_ = listener.Close()
		s.mu.Unlock()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			slog.Debug("SSH accept error", "error", err)
			continue
		}

		go s.handleConn(ctx, conn, serverConfig)
	}
}

func (s *SSHServer) handleConn(ctx context.Context, netConn net.Conn, config *ssh.ServerConfig) {
	defer netConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	nodeIDStr := sshConn.Permissions.Extensions["node_id"]
	nodeID, _ := strconv.ParseInt(nodeIDStr, 10, 64)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}

		go s.handleSession(ctx, channel, requests, nodeID, netConn.RemoteAddr().String())
	}
}

func (s *SSHServer) handleSession(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, nodeID int64, clientAddr string) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			var payload struct {
				Command string
			}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)

			exitCode := s.executeCommand(ctx, channel, nodeID, payload.Command, clientAddr)
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: uint32(exitCode)}))
			return

		case "subsystem":
			var payload struct {
				Subsystem string
			}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)

			exitCode := s.executeCommand(ctx, channel, nodeID, payload.Subsystem, clientAddr)
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: uint32(exitCode)}))
			return

		default:
			_ = req.Reply(false, nil)
		}
	}
}

func (s *SSHServer) executeCommand(ctx context.Context, channel io.ReadWriter, nodeID int64, cmdStr string, clientAddr string) int {
	node, err := s.service.repo.GetECHNode(ctx, nodeID)
	if err != nil || node == nil {
		fmt.Fprintf(channel, "Error: node not found\n")
		return 1
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		fmt.Fprintf(channel, "Usage: sync [--versions <json>] | ack [--acks <json> | --cluster <name> --version <n>]\n")
		return 1
	}

	action := strings.ToLower(parts[0])

	switch action {
	case "sync":
		versionMap := make(map[string]int64)
		proxyType := node.ProxyType
		for i := 1; i < len(parts); i++ {
			switch parts[i] {
			case "--versions":
				if i+1 < len(parts) {
					_ = json.Unmarshal([]byte(parts[i+1]), &versionMap)
					i++
				}
			case "--proxy":
				if i+1 < len(parts) {
					proxyType = parts[i+1]
					i++
				}
			}
		}

		resp, err := s.service.SyncNode(ctx, node, SyncRequest{
			ProxyType: proxyType,
			Clusters:  versionMap,
		}, clientAddr)
		if err != nil {
			fmt.Fprintf(channel, "Error: %v\n", err)
			return 1
		}

		data, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(channel, "Error: json marshal failed: %v\n", err)
			return 1
		}
		_, _ = channel.Write(data)
		_, _ = channel.Write([]byte("\n"))
		return 0

	case "ack":
		var acks []ClusterAckItem
		var clusterID int64
		var clusterName string
		var appliedVer int64
		status := "SUCCESS"
		var msg string

		for i := 1; i < len(parts); i++ {
			switch parts[i] {
			case "--acks":
				if i+1 < len(parts) {
					_ = json.Unmarshal([]byte(parts[i+1]), &acks)
					i++
				}
			case "--cluster-id":
				if i+1 < len(parts) {
					clusterID, _ = strconv.ParseInt(parts[i+1], 10, 64)
					i++
				}
			case "--cluster", "-c":
				if i+1 < len(parts) {
					clusterName = strings.Trim(parts[i+1], "\"'")
					i++
				}
			case "--version", "-v":
				if i+1 < len(parts) {
					appliedVer, _ = strconv.ParseInt(parts[i+1], 10, 64)
					i++
				}
			case "--status", "-s":
				if i+1 < len(parts) {
					status = parts[i+1]
					i++
				}
			case "--msg", "-m":
				if i+1 < len(parts) {
					msg = strings.Trim(parts[i+1], "\"'")
					i++
				}
			}
		}

		if len(acks) == 0 && (clusterID > 0 || clusterName != "" || appliedVer > 0) {
			acks = append(acks, ClusterAckItem{
				ClusterID:      clusterID,
				ClusterName:    clusterName,
				AppliedVersion: appliedVer,
				Status:         status,
				Message:        msg,
			})
		}

		// Attempt reading JSON payload from stdin if pipe was used
		if len(acks) == 0 {
			var ackReq AckRequest
			decoder := json.NewDecoder(channel)
			if err := decoder.Decode(&ackReq); err == nil && len(ackReq.Acks) > 0 {
				acks = ackReq.Acks
			}
		}

		err := s.service.AckNode(ctx, node, AckRequest{Acks: acks}, clientAddr)
		if err != nil {
			fmt.Fprintf(channel, "Error: %v\n", err)
			return 1
		}

		fmt.Fprintf(channel, "OK\n")
		return 0

	default:
		fmt.Fprintf(channel, "Unknown command: %s\n", action)
		return 1
	}
}
