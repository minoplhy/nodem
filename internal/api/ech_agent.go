package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

func (s *AppState) authenticateAgent(r *http.Request) (*db.ECHNode, error) {
	token := strings.TrimSpace(r.Header.Get("X-Agent-Token"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-ECH-Token"))
	}
	if token == "" {
		return nil, transport.ErrUnauthorized
	}

	if s.PullService == nil {
		s.PullService = transport.NewPullService(s.Repo)
	}

	return s.PullService.AuthenticateByToken(r.Context(), token, db.PullTransportHTTPS)
}

// AgentECHSync handles POST /api/v1/agent/ech/sync
func (s *AppState) AgentECHSync(w http.ResponseWriter, r *http.Request) {
	node, err := s.authenticateAgent(r)
	if err != nil {
		if errors.Is(err, transport.ErrTransportMismatch) || strings.Contains(err.Error(), "transport mismatch") {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusUnauthorized, "Invalid or missing X-Agent-Token header")
		return
	}

	var req transport.SyncRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	callerIP := r.RemoteAddr
	resp, err := s.PullService.SyncNode(r.Context(), node, req, callerIP)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, resp)
}

// AgentECHAck handles POST /api/v1/agent/ech/ack
func (s *AppState) AgentECHAck(w http.ResponseWriter, r *http.Request) {
	node, err := s.authenticateAgent(r)
	if err != nil {
		if errors.Is(err, transport.ErrTransportMismatch) || strings.Contains(err.Error(), "transport mismatch") {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusUnauthorized, "Invalid or missing X-Agent-Token header")
		return
	}

	var req transport.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	callerIP := r.RemoteAddr
	if err := s.PullService.AckNode(r.Context(), node, req, callerIP); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
