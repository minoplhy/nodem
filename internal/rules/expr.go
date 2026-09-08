package rules

import (
	"encoding/json"
	"fmt"
)

// RuleType represents the variant of a rule expression.
type RuleType string

const (
	RuleTypeCheckFailed       RuleType = "CheckFailed"
	RuleTypeCheckHealthy      RuleType = "CheckHealthy"
	RuleTypeGlobalCheckFailed RuleType = "GlobalCheckFailed"
	RuleTypeAnd               RuleType = "And"
	RuleTypeOr                RuleType = "Or"
	RuleTypeNot               RuleType = "Not"
)

// CheckIDPayload holds the check_id for leaf nodes.
type CheckIDPayload struct {
	CheckID int64 `json:"check_id"`
}

// RuleExpr is the AST node representing an expression in the visual rule engine.
type RuleExpr struct {
	Type     RuleType
	CheckID  int64
	Children []RuleExpr
	Child    *RuleExpr
}

// UnmarshalJSON parses externally tagged Serde-compatible JSON from Rust / React RuleBuilder.
func (r *RuleExpr) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if payload, ok := raw[string(RuleTypeCheckFailed)]; ok {
		var p CheckIDPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		r.Type = RuleTypeCheckFailed
		r.CheckID = p.CheckID
		return nil
	}

	if payload, ok := raw[string(RuleTypeCheckHealthy)]; ok {
		var p CheckIDPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		r.Type = RuleTypeCheckHealthy
		r.CheckID = p.CheckID
		return nil
	}

	if payload, ok := raw[string(RuleTypeGlobalCheckFailed)]; ok {
		var p CheckIDPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		r.Type = RuleTypeGlobalCheckFailed
		r.CheckID = p.CheckID
		return nil
	}

	if payload, ok := raw[string(RuleTypeAnd)]; ok {
		var children []RuleExpr
		if err := json.Unmarshal(payload, &children); err != nil {
			return err
		}
		r.Type = RuleTypeAnd
		r.Children = children
		return nil
	}

	if payload, ok := raw[string(RuleTypeOr)]; ok {
		var children []RuleExpr
		if err := json.Unmarshal(payload, &children); err != nil {
			return err
		}
		r.Type = RuleTypeOr
		r.Children = children
		return nil
	}

	if payload, ok := raw[string(RuleTypeNot)]; ok {
		var child RuleExpr
		if err := json.Unmarshal(payload, &child); err != nil {
			return err
		}
		r.Type = RuleTypeNot
		r.Child = &child
		return nil
	}

	return fmt.Errorf("unknown rule expression variant: %s", string(data))
}

// MarshalJSON encodes the AST into externally tagged Serde-compatible JSON.
func (r RuleExpr) MarshalJSON() ([]byte, error) {
	switch r.Type {
	case RuleTypeCheckFailed:
		return json.Marshal(map[string]CheckIDPayload{
			"CheckFailed": {CheckID: r.CheckID},
		})
	case RuleTypeCheckHealthy:
		return json.Marshal(map[string]CheckIDPayload{
			"CheckHealthy": {CheckID: r.CheckID},
		})
	case RuleTypeGlobalCheckFailed:
		return json.Marshal(map[string]CheckIDPayload{
			"GlobalCheckFailed": {CheckID: r.CheckID},
		})
	case RuleTypeAnd:
		children := r.Children
		if children == nil {
			children = []RuleExpr{}
		}
		return json.Marshal(map[string][]RuleExpr{
			"And": children,
		})
	case RuleTypeOr:
		children := r.Children
		if children == nil {
			children = []RuleExpr{}
		}
		return json.Marshal(map[string][]RuleExpr{
			"Or": children,
		})
	case RuleTypeNot:
		return json.Marshal(map[string]*RuleExpr{
			"Not": r.Child,
		})
	default:
		return nil, fmt.Errorf("unknown rule expression type: %s", r.Type)
	}
}
