package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type CheckPayload struct {
	Name                   string  `json:"name"`
	Protocol               string  `json:"protocol"`
	Domain                 *string `json:"domain"`
	Port                   uint16  `json:"port"`
	Path                   *string `json:"path"`
	DownThreshold          int64   `json:"down_threshold"`
	UpThreshold            int64   `json:"up_threshold"`
	BypassOnGlobalFailure bool    `json:"bypass_on_global_failure"`
}

func (s *AppState) ListChecks(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	checks, err := s.Repo.ListChecks(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, checks)
}

func (s *AppState) AddCheck(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	var payload CheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	check, err := s.Repo.AddCheck(r.Context(), group.ID, payload.Name, payload.Protocol, payload.Domain, payload.Port, payload.Path, payload.DownThreshold, payload.UpThreshold, payload.BypassOnGlobalFailure)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, check)
}

func (s *AppState) UpdateCheck(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid check ID")
		return
	}

	var payload CheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	check, err := s.Repo.UpdateCheck(r.Context(), group.ID, id, payload.Name, payload.Protocol, payload.Domain, payload.Port, payload.Path, payload.DownThreshold, payload.UpThreshold, payload.BypassOnGlobalFailure)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, check)
}

func (s *AppState) DeleteCheck(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid check ID")
		return
	}

	if err := s.Repo.DeleteCheck(r.Context(), group.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
