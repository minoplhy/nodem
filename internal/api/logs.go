package api

import (
	"net/http"
	"strconv"
)

func (s *AppState) GetGroupLogs(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())

	limit := int64(50)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if val, err := strconv.ParseInt(limitStr, 10, 64); err == nil && val > 0 {
			limit = val
		}
	}

	var ipID *int64
	if ipIDStr := r.URL.Query().Get("ip_id"); ipIDStr != "" {
		if val, err := strconv.ParseInt(ipIDStr, 10, 64); err == nil {
			ipID = &val
		}
	}

	logs, err := s.Repo.ListRecentGroupLogs(r.Context(), group.ID, ipID, limit)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, logs)
}
