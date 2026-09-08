package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"node_monitor_go/internal/rules"
)

type AddRuleRequest struct {
	ExpressionJSON json.RawMessage `json:"expression_json"`
	Action         string          `json:"action"`
}

func (s *AppState) ListRules(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	rulesList, err := s.Repo.ListRules(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, rulesList)
}

func (s *AppState) AddRule(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	var payload AddRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	raw := strings.TrimSpace(string(payload.ExpressionJSON))
	if raw == "" || raw == "null" {
		RespondError(w, http.StatusBadRequest, "expression_json is required")
		return
	}

	var exprStr string
	if strings.HasPrefix(raw, "\"") {
		if err := json.Unmarshal(payload.ExpressionJSON, &exprStr); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid JSON string in expression_json")
			return
		}
	} else {
		exprStr = raw
	}

	// Validate rule AST
	var expr rules.RuleExpr
	if err := json.Unmarshal([]byte(exprStr), &expr); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid rule expression syntax: "+err.Error())
		return
	}

	if payload.Action != "RemoveFromDns" && payload.Action != "AddToDns" {
		RespondError(w, http.StatusBadRequest, "Action must be 'RemoveFromDns' or 'AddToDns'")
		return
	}

	rule, err := s.Repo.AddRule(r.Context(), group.ID, exprStr, payload.Action)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, rule)
}

func (s *AppState) DeleteRule(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	if err := s.Repo.DeleteRule(r.Context(), group.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
