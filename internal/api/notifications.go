package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/minoplhy/nodem/internal/notifications"
)

type CreateNotificationChannelRequest struct {
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"`
	ConfigJSON  string `json:"config_json"`
}

type LinkGroupNotificationRequest struct {
	ChannelID    int64 `json:"channel_id"`
	NotifyOnUp   bool  `json:"notify_on_up"`
	NotifyOnDown bool  `json:"notify_on_down"`
}

type GroupNotificationResponse struct {
	ChannelID    int64  `json:"channel_id"`
	Name         string `json:"name"`
	ChannelType  string `json:"channel_type"`
	NotifyOnUp   bool   `json:"notify_on_up"`
	NotifyOnDown bool   `json:"notify_on_down"`
}

func (s *AppState) ListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	list, err := s.Repo.ListNotificationChannels(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, list)
}

func (s *AppState) CreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var payload CreateNotificationChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	chanObj, err := s.Repo.CreateNotificationChannel(r.Context(), user.ID, payload.Name, payload.ChannelType, payload.ConfigJSON)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, chanObj)
}

func (s *AppState) UpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid notification channel ID")
		return
	}

	var payload CreateNotificationChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	chanObj, err := s.Repo.UpdateNotificationChannel(r.Context(), user.ID, id, payload.Name, payload.ChannelType, payload.ConfigJSON)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, chanObj)
}

func (s *AppState) DeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid notification channel ID")
		return
	}

	if err := s.Repo.DeleteNotificationChannel(r.Context(), user.ID, id); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) TestNotificationChannel(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid notification channel ID")
		return
	}

	chanObj, err := s.Repo.GetNotificationChannel(r.Context(), user.ID, id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if chanObj == nil {
		RespondError(w, http.StatusNotFound, "Notification channel not found")
		return
	}

	notifier, err := notifications.CreateNotifier(chanObj)
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid notifier configuration: %v", err))
		return
	}

	if err := notifier.SendNotification(r.Context(), "🔔 This is a test notification from Node Monitor!", "info"); err != nil {
		RespondError(w, http.StatusBadGateway, fmt.Sprintf("Failed to send notification: %v", err))
		return
	}

	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) ListGroupNotifications(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	list, err := s.Repo.ListGroupNotifications(r.Context(), group.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var response []GroupNotificationResponse
	for _, item := range list {
		response = append(response, GroupNotificationResponse{
			ChannelID:    item.GroupNotification.ChannelID,
			Name:         item.Channel.Name,
			ChannelType:  item.Channel.ChannelType,
			NotifyOnUp:   item.GroupNotification.NotifyOnUp,
			NotifyOnDown: item.GroupNotification.NotifyOnDown,
		})
	}

	RespondJSON(w, http.StatusOK, response)
}

func (s *AppState) LinkGroupNotification(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	var payload LinkGroupNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.Repo.LinkGroupNotification(r.Context(), group.ID, payload.ChannelID, payload.NotifyOnUp, payload.NotifyOnDown); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *AppState) UnlinkGroupNotification(w http.ResponseWriter, r *http.Request) {
	group := GetGroupFromContext(r.Context())
	chanIDStr := chi.URLParam(r, "channel_id")
	chanID, err := strconv.ParseInt(chanIDStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := s.Repo.UnlinkGroupNotification(r.Context(), group.ID, chanID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
