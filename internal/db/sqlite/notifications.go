package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/misc"
)

func (r *SqliteRepository) GetNotificationChannel(ctx context.Context, tenantID, id int64) (*db.NotificationChannel, error) {
	query := "SELECT id, tenant_id, name, channel_type, config_json FROM notification_channels WHERE tenant_id = ? AND id = ?"
	row := r.db.QueryRowContext(ctx, query, tenantID, id)

	var nc db.NotificationChannel
	err := row.Scan(&nc.ID, &nc.TenantID, &nc.Name, &nc.ChannelType, &nc.ConfigJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &nc, nil
}

func (r *SqliteRepository) ListNotificationChannels(ctx context.Context, tenantID int64) ([]db.NotificationChannel, error) {
	query := "SELECT id, tenant_id, name, channel_type, config_json FROM notification_channels WHERE tenant_id = ?"
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []db.NotificationChannel
	for rows.Next() {
		var nc db.NotificationChannel
		if err := rows.Scan(&nc.ID, &nc.TenantID, &nc.Name, &nc.ChannelType, &nc.ConfigJSON); err != nil {
			return nil, err
		}
		channels = append(channels, nc)
	}
	return channels, rows.Err()
}

func (r *SqliteRepository) CreateNotificationChannel(ctx context.Context, tenantID int64, name, channelType, configJSON string) (*db.NotificationChannel, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO notification_channels (id, tenant_id, name, channel_type, config_json) VALUES (?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, tenantID, name, channelType, configJSON)
	if err != nil {
		return nil, err
	}
	return r.GetNotificationChannel(ctx, tenantID, id)
}

func (r *SqliteRepository) UpdateNotificationChannel(ctx context.Context, tenantID, id int64, name, channelType, configJSON string) (*db.NotificationChannel, error) {
	query := "UPDATE notification_channels SET name = ?, channel_type = ?, config_json = ? WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, name, channelType, configJSON, tenantID, id)
	if err != nil {
		return nil, err
	}
	return r.GetNotificationChannel(ctx, tenantID, id)
}

func (r *SqliteRepository) DeleteNotificationChannel(ctx context.Context, tenantID, id int64) error {
	query := "DELETE FROM notification_channels WHERE tenant_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, tenantID, id)
	return err
}

func (r *SqliteRepository) LinkGroupNotification(ctx context.Context, groupID, channelID int64, notifyOnUp, notifyOnDown bool) error {
	query := "INSERT OR REPLACE INTO group_notifications (group_id, channel_id, notify_on_up, notify_on_down) VALUES (?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, groupID, channelID, notifyOnUp, notifyOnDown)
	return err
}

func (r *SqliteRepository) UnlinkGroupNotification(ctx context.Context, groupID, channelID int64) error {
	query := "DELETE FROM group_notifications WHERE group_id = ? AND channel_id = ?"
	_, err := r.db.ExecContext(ctx, query, groupID, channelID)
	return err
}

func (r *SqliteRepository) ListGroupNotifications(ctx context.Context, groupID int64) ([]db.LinkedGroupNotification, error) {
	query := `SELECT gn.group_id, gn.channel_id, gn.notify_on_up, gn.notify_on_down, nc.tenant_id, nc.name, nc.channel_type, nc.config_json
              FROM group_notifications gn
              JOIN notification_channels nc ON gn.channel_id = nc.id
              WHERE gn.group_id = ?`

	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.LinkedGroupNotification
	for rows.Next() {
		var gn db.GroupNotification
		var nc db.NotificationChannel
		if err := rows.Scan(&gn.GroupID, &gn.ChannelID, &gn.NotifyOnUp, &gn.NotifyOnDown, &nc.TenantID, &nc.Name, &nc.ChannelType, &nc.ConfigJSON); err != nil {
			return nil, err
		}
		nc.ID = gn.ChannelID
		list = append(list, db.LinkedGroupNotification{
			GroupNotification: gn,
			Channel:           nc,
		})
	}
	return list, rows.Err()
}
