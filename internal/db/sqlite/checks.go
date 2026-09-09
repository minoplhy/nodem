package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/misc"
)

func (r *SqliteRepository) ListChecks(ctx context.Context, groupID int64) ([]db.CheckConfig, error) {
	query := "SELECT id, group_id, name, protocol, domain, port, path, down_threshold, up_threshold, bypass_on_global_failure FROM checks WHERE group_id = ?"
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []db.CheckConfig
	for rows.Next() {
		var c db.CheckConfig
		var domain, path sql.NullString
		var portInt int
		if err := rows.Scan(&c.ID, &c.GroupID, &c.Name, &c.Protocol, &domain, &portInt, &path, &c.DownThreshold, &c.UpThreshold, &c.BypassOnGlobalFailure); err != nil {
			return nil, err
		}
		c.Port = uint16(portInt)
		if domain.Valid {
			c.Domain = &domain.String
		}
		if path.Valid {
			c.Path = &path.String
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (r *SqliteRepository) AddCheck(ctx context.Context, groupID int64, name, protocol string, domain *string, port uint16, path *string, downThreshold, upThreshold int64, bypassOnGlobalFailure bool) (*db.CheckConfig, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO checks (id, group_id, name, protocol, domain, port, path, down_threshold, up_threshold, bypass_on_global_failure) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, groupID, name, protocol, domain, int(port), path, downThreshold, upThreshold, bypassOnGlobalFailure)
	if err != nil {
		return nil, err
	}

	selQuery := "SELECT id, group_id, name, protocol, domain, port, path, down_threshold, up_threshold, bypass_on_global_failure FROM checks WHERE id = ?"
	row := r.db.QueryRowContext(ctx, selQuery, id)

	var c db.CheckConfig
	var dom, pth sql.NullString
	var portInt int
	if err := row.Scan(&c.ID, &c.GroupID, &c.Name, &c.Protocol, &dom, &portInt, &pth, &c.DownThreshold, &c.UpThreshold, &c.BypassOnGlobalFailure); err != nil {
		return nil, err
	}
	c.Port = uint16(portInt)
	if dom.Valid {
		c.Domain = &dom.String
	}
	if pth.Valid {
		c.Path = &pth.String
	}
	return &c, nil
}

func (r *SqliteRepository) UpdateCheck(ctx context.Context, groupID, id int64, name, protocol string, domain *string, port uint16, path *string, downThreshold, upThreshold int64, bypassOnGlobalFailure bool) (*db.CheckConfig, error) {
	query := "UPDATE checks SET name = ?, protocol = ?, domain = ?, port = ?, path = ?, down_threshold = ?, up_threshold = ?, bypass_on_global_failure = ? WHERE group_id = ? AND id = ?"
	_, err := r.db.ExecContext(ctx, query, name, protocol, domain, int(port), path, downThreshold, upThreshold, bypassOnGlobalFailure, groupID, id)
	if err != nil {
		return nil, err
	}

	selQuery := "SELECT id, group_id, name, protocol, domain, port, path, down_threshold, up_threshold, bypass_on_global_failure FROM checks WHERE id = ?"
	row := r.db.QueryRowContext(ctx, selQuery, id)

	var c db.CheckConfig
	var dom, pth sql.NullString
	var portInt int
	if err := row.Scan(&c.ID, &c.GroupID, &c.Name, &c.Protocol, &dom, &portInt, &pth, &c.DownThreshold, &c.UpThreshold, &c.BypassOnGlobalFailure); err != nil {
		return nil, err
	}
	c.Port = uint16(portInt)
	if dom.Valid {
		c.Domain = &dom.String
	}
	if pth.Valid {
		c.Path = &pth.String
	}
	return &c, nil
}

func (r *SqliteRepository) DeleteCheck(ctx context.Context, _ int64, id int64) error {
	query := "DELETE FROM checks WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *SqliteRepository) GetCheckState(ctx context.Context, ipID, checkID int64) (*db.CheckState, error) {
	query := "SELECT ip_id, check_id, consecutive_up, consecutive_down, status, message FROM check_states WHERE ip_id = ? AND check_id = ?"
	row := r.db.QueryRowContext(ctx, query, ipID, checkID)

	var cs db.CheckState
	var msg sql.NullString
	if err := row.Scan(&cs.IPID, &cs.CheckID, &cs.ConsecutiveUp, &cs.ConsecutiveDown, &cs.Status, &msg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if msg.Valid {
		cs.Message = &msg.String
	}
	return &cs, nil
}

func (r *SqliteRepository) ListCheckStatesForIP(ctx context.Context, ipID int64) ([]db.CheckState, error) {
	query := "SELECT ip_id, check_id, consecutive_up, consecutive_down, status, message FROM check_states WHERE ip_id = ?"
	rows, err := r.db.QueryContext(ctx, query, ipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []db.CheckState
	for rows.Next() {
		var cs db.CheckState
		var msg sql.NullString
		if err := rows.Scan(&cs.IPID, &cs.CheckID, &cs.ConsecutiveUp, &cs.ConsecutiveDown, &cs.Status, &msg); err != nil {
			return nil, err
		}
		if msg.Valid {
			cs.Message = &msg.String
		}
		states = append(states, cs)
	}
	return states, rows.Err()
}

func (r *SqliteRepository) ListCheckStatesForGroup(ctx context.Context, groupID int64) ([]db.CheckState, error) {
	query := "SELECT cs.ip_id, cs.check_id, cs.consecutive_up, cs.consecutive_down, cs.status, cs.message FROM check_states cs JOIN target_ips ti ON cs.ip_id = ti.id WHERE ti.group_id = ?"
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []db.CheckState
	for rows.Next() {
		var cs db.CheckState
		var msg sql.NullString
		if err := rows.Scan(&cs.IPID, &cs.CheckID, &cs.ConsecutiveUp, &cs.ConsecutiveDown, &cs.Status, &msg); err != nil {
			return nil, err
		}
		if msg.Valid {
			cs.Message = &msg.String
		}
		states = append(states, cs)
	}
	return states, rows.Err()
}

func (r *SqliteRepository) UpdateCheckState(ctx context.Context, ipID, checkID, consecutiveUp, consecutiveDown int64, status string, message *string) error {
	query := "INSERT OR REPLACE INTO check_states (ip_id, check_id, consecutive_up, consecutive_down, status, message) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, ipID, checkID, consecutiveUp, consecutiveDown, status, message)
	return err
}
