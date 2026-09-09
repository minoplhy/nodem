package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/minoplhy/nodem/internal/db"
)

type contextKey string

const (
	userContextKey  contextKey = "authenticated_user"
	groupContextKey contextKey = "group_access"
)

// GetUserFromContext retrieves the authenticated User from the request context.
func GetUserFromContext(ctx context.Context) *db.User {
	if u, ok := ctx.Value(userContextKey).(*db.User); ok {
		return u
	}
	return nil
}

// GetGroupFromContext retrieves the authorized TargetGroup from the request context.
func GetGroupFromContext(ctx context.Context) *db.TargetGroup {
	if g, ok := ctx.Value(groupContextKey).(*db.TargetGroup); ok {
		return g
	}
	return nil
}

// RequireAuth middleware ensures the request has a valid session cookie.
func (s *AppState) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie == nil {
			RespondError(w, http.StatusUnauthorized, "Missing session cookie")
			return
		}

		sessionID := cookie.Value
		if sessionID == "" {
			RespondError(w, http.StatusUnauthorized, "Empty session ID")
			return
		}

		session, err := s.Repo.GetSession(r.Context(), sessionID)
		if err != nil || session == nil {
			RespondError(w, http.StatusUnauthorized, "Session expired or invalid")
			return
		}

		user, err := s.Repo.GetUserByID(r.Context(), session.UserID)
		if err != nil || user == nil {
			RespondError(w, http.StatusUnauthorized, "Invalid user")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireGroupAccess middleware verifies that the user has permission to access the target group.
func (s *AppState) RequireGroupAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		groupIDStr := chi.URLParam(r, "group_id")
		if groupIDStr == "" {
			groupIDStr = chi.URLParam(r, "id")
		}
		if groupIDStr == "" {
			RespondError(w, http.StatusBadRequest, "Missing group ID in path")
			return
		}

		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid group ID format")
			return
		}

		group, err := s.Repo.GetGroup(r.Context(), user.ID, groupID)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if group == nil {
			RespondError(w, http.StatusNotFound, "Group not found or access denied")
			return
		}

		ctx := context.WithValue(r.Context(), groupContextKey, group)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
