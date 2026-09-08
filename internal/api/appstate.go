package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"node_monitor_go/internal/db"
)

// AppState contains shared dependencies and runtime configuration.
type AppState struct {
	Repo           db.Repository
	BootstrapToken string
	BasePath       string
}

// RespondJSON writes an HTTP status code and serializes payload as JSON.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
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
