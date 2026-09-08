package sqlite

import (
	"context"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/misc"
)

func (r *SqliteRepository) ListRules(ctx context.Context, groupID int64) ([]db.GroupRule, error) {
	query := "SELECT id, group_id, expression_json, action FROM group_rules WHERE group_id = ?"
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []db.GroupRule
	for rows.Next() {
		var gr db.GroupRule
		if err := rows.Scan(&gr.ID, &gr.GroupID, &gr.ExpressionJSON, &gr.Action); err != nil {
			return nil, err
		}
		rules = append(rules, gr)
	}
	return rules, rows.Err()
}

func (r *SqliteRepository) AddRule(ctx context.Context, groupID int64, expressionJSON, action string) (*db.GroupRule, error) {
	id := misc.GenerateRandomID()
	query := "INSERT INTO group_rules (id, group_id, expression_json, action) VALUES (?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, id, groupID, expressionJSON, action)
	if err != nil {
		return nil, err
	}

	return &db.GroupRule{
		ID:             id,
		GroupID:        groupID,
		ExpressionJSON: expressionJSON,
		Action:         action,
	}, nil
}

func (r *SqliteRepository) DeleteRule(ctx context.Context, _ int64, id int64) error {
	query := "DELETE FROM group_rules WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
