package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/minoplhy/nodem/internal/checkers"
	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/providers"
)

type CreateGroupRequest struct {
	Name              string `json:"name"`
	DnsRecord         string `json:"dns_record"`
	DnsProviderID     int64  `json:"dns_provider_id"`
	CheckIntervalSecs int64  `json:"check_interval_secs"`
}

type ToggleGroupRequest struct {
	Enabled bool `json:"enabled"`
}

type TestCheckResult struct {
	IP        string  `json:"ip"`
	CheckName string  `json:"check_name"`
	Success   bool    `json:"success"`
	Message   *string `json:"message"`
}

type CheckStateResponse struct {
	IPID            int64   `json:"ip_id"`
	CheckID         int64   `json:"check_id"`
	ConsecutiveUp   int64   `json:"consecutive_up"`
	ConsecutiveDown int64   `json:"consecutive_down"`
	Status          string  `json:"status"`
	Message         *string `json:"message"`
}

type GroupStatusResponse struct {
	IPs          []db.TargetIp        `json:"ips"`
	Checks       []db.CheckConfig     `json:"checks"`
	States       []CheckStateResponse `json:"states"`
	UnmanagedIPs []string             `json:"unmanaged_ips"`
}

type GroupConfigResponse struct {
	Group         db.TargetGroup              `json:"group"`
	IPs           []db.TargetIp               `json:"ips"`
	Checks        []db.CheckConfig            `json:"checks"`
	Rules         []db.GroupRule              `json:"rules"`
	Subscriptions []GroupNotificationResponse `json:"subscriptions"`
}

type UnifiedGroupsStatusResponse struct {
	Groups   []db.TargetGroup               `json:"groups"`
	Statuses map[string]GroupStatusResponse `json:"statuses"` // Map key as string for JSON 1-1 compatibility
}

func (s *AppState) ListGroups(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	list, err := s.Repo.ListGroups(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, list)
}

func (s *AppState) CreateGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var payload CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	group, err := s.Repo.CreateGroup(r.Context(), user.ID, payload.Name, payload.DnsRecord, payload.DnsProviderID, payload.CheckIntervalSecs)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, group)
}

func (s *AppState) GetGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid group ID")
		return
	}

	group, err := s.Repo.GetGroup(r.Context(), user.ID, id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if group == nil {
		RespondError(w, http.StatusNotFound, "Group not found")
		return
	}
	RespondJSON(w, http.StatusOK, group)
}

func (s *AppState) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid group ID")
		return
	}

	var payload CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	group, err := s.Repo.UpdateGroup(r.Context(), user.ID, id, payload.Name, payload.DnsRecord, payload.DnsProviderID, payload.CheckIntervalSecs)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, group)
}

func (s *AppState) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid group ID")
		return
	}

	if err := s.Repo.DeleteGroup(r.Context(), user.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) ToggleGroup(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	user := GetUserFromContext(r.Context())

	var payload ToggleGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.Repo.UpdateGroupEnabled(r.Context(), user.ID, group.ID, payload.Enabled); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) TestGroupConfig(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())

	ips, err := s.Repo.ListIPs(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	checks, err := s.Repo.ListChecks(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(ips) == 0 {
		RespondError(w, http.StatusBadRequest, "No target IPs configured for this group")
		return
	}
	if len(checks) == 0 {
		RespondError(w, http.StatusBadRequest, "No health checks configured for this group")
		return
	}

	checker := checkers.NewNodeChecker()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []TestCheckResult

	for _, ip := range ips {
		for _, chk := range checks {
			wg.Add(1)
			go func(targetIP db.TargetIp, check db.CheckConfig) {
				defer wg.Done()
				res := checker.RunCheck(r.Context(), targetIP.IP, &check)
				mu.Lock()
				results = append(results, TestCheckResult{
					IP:        targetIP.IP,
					CheckName: check.Name,
					Success:   res.Success,
					Message:   res.Message,
				})
				mu.Unlock()
			}(ip, chk)
		}
	}
	wg.Wait()

	RespondJSON(w, http.StatusOK, results)
}

func (s *AppState) GetGroupStatus(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())

	ips, err := s.Repo.ListIPs(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	checks, err := s.Repo.ListChecks(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dbStates, err := s.Repo.ListCheckStatesForGroup(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var states []CheckStateResponse
	for _, cs := range dbStates {
		states = append(states, CheckStateResponse{
			IPID:            cs.IPID,
			CheckID:         cs.CheckID,
			ConsecutiveUp:   cs.ConsecutiveUp,
			ConsecutiveDown: cs.ConsecutiveDown,
			Status:          cs.Status,
			Message:         cs.Message,
		})
	}

	var unmanagedIPs []string
	providerConfig, _ := s.Repo.GetProviderByIDDirect(r.Context(), group.DnsProviderID)
	if providerConfig != nil {
		if client, err := providers.CreateProviderClient(providerConfig); err == nil {
			if records, err := client.ListRecords(r.Context(), group.DnsRecord); err == nil {
				for _, rec := range records {
					found := false
					for _, ip := range ips {
						if ip.IP == rec {
							found = true
							break
						}
					}
					if !found {
						unmanagedIPs = append(unmanagedIPs, rec)
					}
				}
			}
		}
	}

	RespondJSON(w, http.StatusOK, GroupStatusResponse{
		IPs:          ips,
		Checks:       checks,
		States:       states,
		UnmanagedIPs: unmanagedIPs,
	})
}

func (s *AppState) GetGroupConfig(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid group ID")
		return
	}

	group, err := s.Repo.GetGroup(r.Context(), user.ID, id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if group == nil {
		RespondError(w, http.StatusNotFound, "Group not found")
		return
	}

	ips, err := s.Repo.ListIPs(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	checks, err := s.Repo.ListChecks(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rulesList, err := s.Repo.ListRules(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	linkedNotifs, err := s.Repo.ListGroupNotifications(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var subscriptions []GroupNotificationResponse
	for _, item := range linkedNotifs {
		subscriptions = append(subscriptions, GroupNotificationResponse{
			ChannelID:    item.GroupNotification.ChannelID,
			Name:         item.Channel.Name,
			ChannelType:  item.Channel.ChannelType,
			NotifyOnUp:   item.GroupNotification.NotifyOnUp,
			NotifyOnDown: item.GroupNotification.NotifyOnDown,
		})
	}

	RespondJSON(w, http.StatusOK, GroupConfigResponse{
		Group:         *group,
		IPs:           ips,
		Checks:        checks,
		Rules:         rulesList,
		Subscriptions: subscriptions,
	})
}

func (s *AppState) GetAllGroupsStatus(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	groups, err := s.Repo.ListGroups(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	statuses := make(map[string]GroupStatusResponse)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, g := range groups {
		wg.Add(1)
		go func(group db.TargetGroup) {
			defer wg.Done()
			ips, _ := s.Repo.ListIPs(r.Context(), group.ID)
			checks, _ := s.Repo.ListChecks(r.Context(), group.ID)
			dbStates, _ := s.Repo.ListCheckStatesForGroup(r.Context(), group.ID)

			var states []CheckStateResponse
			for _, cs := range dbStates {
				states = append(states, CheckStateResponse{
					IPID:            cs.IPID,
					CheckID:         cs.CheckID,
					ConsecutiveUp:   cs.ConsecutiveUp,
					ConsecutiveDown: cs.ConsecutiveDown,
					Status:          cs.Status,
					Message:         cs.Message,
				})
			}

			var unmanagedIPs []string
			providerConfig, _ := s.Repo.GetProviderByIDDirect(r.Context(), group.DnsProviderID)
			if providerConfig != nil {
				if client, err := providers.CreateProviderClient(providerConfig); err == nil {
					if records, err := client.ListRecords(r.Context(), group.DnsRecord); err == nil {
						for _, rec := range records {
							found := false
							for _, ip := range ips {
								if ip.IP == rec {
									found = true
									break
								}
							}
							if !found {
								unmanagedIPs = append(unmanagedIPs, rec)
							}
						}
					}
				}
			}

			mu.Lock()
			statuses[strconv.FormatInt(group.ID, 10)] = GroupStatusResponse{
				IPs:          ips,
				Checks:       checks,
				States:       states,
				UnmanagedIPs: unmanagedIPs,
			}
			mu.Unlock()
		}(g)
	}
	wg.Wait()

	RespondJSON(w, http.StatusOK, UnifiedGroupsStatusResponse{
		Groups:   groups,
		Statuses: statuses,
	})
}
