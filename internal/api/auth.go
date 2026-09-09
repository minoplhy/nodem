package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/minoplhy/nodem/internal/misc"
)

type SetupRequest struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (s *AppState) SetupStatus(w http.ResponseWriter, r *http.Request) {
	exists, err := s.Repo.UserExists(r.Context())
	if err != nil {
		RespondJSON(w, http.StatusOK, map[string]bool{"setup_required": true})
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"setup_required": !exists})
}

func (s *AppState) Setup(w http.ResponseWriter, r *http.Request) {
	var payload SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	exists, _ := s.Repo.UserExists(r.Context())
	if exists {
		RespondError(w, http.StatusBadRequest, "Setup already completed")
		return
	}

	if payload.Token != s.BootstrapToken {
		RespondError(w, http.StatusUnauthorized, "Invalid bootstrap token")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user, err := s.Repo.CreateUser(r.Context(), payload.Username, string(hash), "Admin")
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}

func (s *AppState) Login(w http.ResponseWriter, r *http.Request) {
	var payload LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := s.Repo.GetUserByUsername(r.Context(), payload.Username)
	if err != nil || user == nil {
		RespondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		RespondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	sessionID, err := misc.GenerateSessionID()
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Extract Client IP
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress != "" {
		if idx := strings.Index(ipAddress, ","); idx != -1 {
			ipAddress = strings.TrimSpace(ipAddress[:idx])
		}
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ipAddress = strings.TrimSpace(realIP)
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			ipAddress = host
		} else {
			ipAddress = r.RemoteAddr
		}
	}

	// Extract User-Agent
	userAgent := r.Header.Get("User-Agent")
	var userAgentPtr *string
	if userAgent != "" {
		userAgentPtr = &userAgent
	}
	var ipAddressPtr *string
	if ipAddress != "" {
		ipAddressPtr = &ipAddress
	}

	sessionDuration := 24 * time.Hour
	if payload.RememberMe {
		sessionDuration = 30 * 24 * time.Hour
	}
	expiresAt := time.Now().UTC().Add(sessionDuration)

	publicID, err := misc.GeneratePublicSessionID()
	if err != nil {
		publicID = "sess_" + sessionID[:16]
	}

	if err := s.Repo.CreateSession(r.Context(), sessionID, publicID, user.ID, expiresAt, ipAddressPtr, userAgentPtr); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Trigger opportunistic async cleanup of expired sessions on login
	go func() {
		_ = s.Repo.CleanupExpiredSessions(context.Background())
	}()

	cookiePath := "/"
	if s.BasePath != "" {
		cookiePath = s.BasePath
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     cookiePath,
		Expires:  expiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	RespondJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}

func (s *AppState) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil && cookie != nil && cookie.Value != "" {
		_ = s.Repo.DeleteSession(r.Context(), cookie.Value)
	}

	cookiePath := "/"
	if s.BasePath != "" {
		cookiePath = s.BasePath
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     cookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) Me(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		RespondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	RespondJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
	})
}
