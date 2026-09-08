package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type ProviderPayload struct {
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"`
	APIURL       string `json:"api_url"`
	Token        string `json:"token"`
	Zone         string `json:"zone"`
}

func (s *AppState) ListProviders(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	list, err := s.Repo.ListProviders(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, list)
}

func (s *AppState) CreateProvider(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var payload ProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	prov, err := s.Repo.CreateProvider(r.Context(), user.ID, payload.Name, payload.ProviderType, payload.APIURL, payload.Token, payload.Zone)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, prov)
}

func (s *AppState) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	var payload ProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	prov, err := s.Repo.UpdateProvider(r.Context(), user.ID, id, payload.Name, payload.ProviderType, payload.APIURL, payload.Token, payload.Zone)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, prov)
}

func (s *AppState) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	if err := s.Repo.DeleteProvider(r.Context(), user.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
