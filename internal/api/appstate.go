package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/ech/transport"
)

// AppState contains shared dependencies and runtime configuration.
type AppState struct {
	Repo           db.Repository
	BootstrapToken string
	BasePath       string
	OpenSSLPath    string
	PullService    interface {
		AuthenticateByToken(ctx context.Context, token, callerTransport string) (*db.ECHNode, error)
		SyncNode(ctx context.Context, node *db.ECHNode, req transport.SyncRequest, callerIP string) (*transport.SyncResponse, error)
		AckNode(ctx context.Context, node *db.ECHNode, req transport.AckRequest, callerIP string) error
	}
}

// RespondJSON writes an HTTP status code and serializes payload as JSON.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondList writes a JSON array response. A nil slice is encoded as []
// instead of null, so clients can always treat list endpoints as arrays.
func RespondList[T any](w http.ResponseWriter, status int, items []T) {
	if items == nil {
		items = []T{}
	}
	RespondJSON(w, status, items)
}

// emptyIfNil returns s, or an empty (non-nil) slice when s is nil, so that
// slice fields in response structs encode as [] instead of null.
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// RespondError writes an API error response in the standard {"message": "..."} format.
func RespondError(w http.ResponseWriter, status int, message string) {
	if status >= 500 {
		slog.Error("API Server Error", "status", status, "message", message)
	}
	RespondJSON(w, status, map[string]string{
		"message": message,
	})
}
