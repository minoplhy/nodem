package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"node_monitor_go/internal/db"
)

type AddIpRequest struct {
	IP        string `json:"ip"`
	IPAddress string `json:"ip_address,omitempty"`
}

type SyncIpsRequest struct {
	IPs []db.TargetIpInput `json:"ips"`
}

func (s *AppState) ListIPs(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	ips, err := s.Repo.ListIPs(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, ips)
}

func (s *AppState) AddIP(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		RespondError(w, http.StatusBadRequest, "Invalid request body: 'ip' field is required")
		return
	}

	var targetIP string

	// 1. Try JSON object: {"ip": "..."} or {"ip_address": "..."}
	var payload AddIpRequest
	if err := json.Unmarshal(trimmed, &payload); err == nil {
		if payload.IP != "" {
			targetIP = strings.TrimSpace(payload.IP)
		} else if payload.IPAddress != "" {
			targetIP = strings.TrimSpace(payload.IPAddress)
		}
	}

	// 2. Try JSON raw string: "1.2.3.4"
	if targetIP == "" {
		var rawStr string
		if err := json.Unmarshal(trimmed, &rawStr); err == nil && strings.TrimSpace(rawStr) != "" {
			targetIP = strings.TrimSpace(rawStr)
		}
	}

	// 3. Try sync array or object if a batch was sent to POST
	if targetIP == "" {
		inputs, err := parseSyncIps(trimmed)
		if err == nil && len(inputs) > 0 {
			if err := s.Repo.SyncIPs(r.Context(), group.ID, inputs); err != nil {
				RespondError(w, http.StatusInternalServerError, err.Error())
				return
			}
			RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
	}

	if targetIP == "" {
		RespondError(w, http.StatusBadRequest, "Invalid request body: 'ip' field is required")
		return
	}

	ip, err := s.Repo.AddIP(r.Context(), group.ID, targetIP)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, ip)
}

func parseSyncIps(body []byte) ([]db.TargetIpInput, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return []db.TargetIpInput{}, nil
	}

	// 1. Try standard object: {"ips": [{"ip": "...", "enabled": true}]}
	var objReq SyncIpsRequest
	if err := json.Unmarshal(trimmed, &objReq); err == nil && objReq.IPs != nil {
		return objReq.IPs, nil
	}

	// 2. Try object with string list: {"ips": ["1.2.3.4", "5.6.7.8"]}
	var strObjReq struct {
		IPs []string `json:"ips"`
	}
	if err := json.Unmarshal(trimmed, &strObjReq); err == nil && strObjReq.IPs != nil {
		inputs := make([]db.TargetIpInput, len(strObjReq.IPs))
		for i, s := range strObjReq.IPs {
			inputs[i] = db.TargetIpInput{IP: strings.TrimSpace(s), Enabled: true}
		}
		return inputs, nil
	}

	// 3. Try direct array of TargetIpInput: [{"ip": "...", "enabled": true}]
	var arrReq []db.TargetIpInput
	if err := json.Unmarshal(trimmed, &arrReq); err == nil {
		return arrReq, nil
	}

	// 4. Try direct array of strings: ["1.2.3.4", "5.6.7.8"]
	var strArr []string
	if err := json.Unmarshal(trimmed, &strArr); err == nil {
		inputs := make([]db.TargetIpInput, len(strArr))
		for i, s := range strArr {
			inputs[i] = db.TargetIpInput{IP: strings.TrimSpace(s), Enabled: true}
		}
		return inputs, nil
	}

	// 5. Try single TargetIpInput object: {"ip": "1.2.3.4", "enabled": true}
	var single db.TargetIpInput
	if err := json.Unmarshal(trimmed, &single); err == nil && single.IP != "" {
		return []db.TargetIpInput{single}, nil
	}

	return nil, fmt.Errorf("invalid request body format for syncing IPs")
}

func (s *AppState) SyncGroupIPs(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	inputs, err := parseSyncIps(body)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body: expected array of IPs or object with 'ips'")
		return
	}

	if err := s.Repo.SyncIPs(r.Context(), group.ID, inputs); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) DeleteIP(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid IP ID")
		return
	}

	if err := s.Repo.DeleteIP(r.Context(), group.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
