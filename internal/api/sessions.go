package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type SessionResponse struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"` // Matches public ID so secret cookie is NEVER leaked
	IPAddress  *string    `json:"ip_address"`
	UserAgent  *string    `json:"user_agent"`
	LastActive *time.Time `json:"last_active"`
	ExpiresAt  time.Time  `json:"expires_at"`
	IsCurrent  bool       `json:"is_current"`
}

func (s *AppState) ListActiveSessions(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())

	currentSessionID := ""
	if cookie, err := r.Cookie("session_id"); err == nil && cookie != nil {
		currentSessionID = cookie.Value
	}

	sessions, err := s.Repo.ListSessions(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Trigger opportunistic async cleanup of expired sessions on session list
	go func() {
		_ = s.Repo.CleanupExpiredSessions(context.Background())
	}()

	var response []SessionResponse
	for _, sess := range sessions {
		pubID := sess.PublicID
		if pubID == "" {
			if len(sess.SessionID) > 12 {
				pubID = "sess_" + sess.SessionID[:12]
			} else {
				pubID = "sess_device"
			}
		}

		response = append(response, SessionResponse{
			ID:         pubID,
			SessionID:  pubID, // NEVER leak sess.SessionID!
			IPAddress:  sess.IPAddress,
			UserAgent:  sess.UserAgent,
			LastActive: sess.LastActive,
			ExpiresAt:  sess.ExpiresAt,
			IsCurrent:  sess.SessionID == currentSessionID,
		})
	}

	RespondJSON(w, http.StatusOK, response)
}

func (s *AppState) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	targetSID := chi.URLParam(r, "id")
	if targetSID == "" {
		RespondError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	deleted, err := s.Repo.DeleteSessionByPublicID(r.Context(), user.ID, targetSID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		RespondError(w, http.StatusNotFound, "Session not found")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
