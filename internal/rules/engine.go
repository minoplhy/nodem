package rules

import (
	"log/slog"
	"node_monitor_go/internal/db"
)

// CheckStateKey identifies a check state by IP ID and Check ID.
type CheckStateKey struct {
	IPID    int64
	CheckID int64
}

// EvaluationContext contains the runtime state needed to evaluate a RuleExpr.
type EvaluationContext struct {
	IPID        int64
	GroupID     int64
	CheckStates map[CheckStateKey]db.CheckState
	IPIDs       []int64 // All IP IDs in this group
}

// Evaluate recursively evaluates the rule expression against the given context.
func (r *RuleExpr) Evaluate(ctx *EvaluationContext) bool {
	var result bool
	switch r.Type {
	case RuleTypeCheckFailed:
		if state, ok := ctx.CheckStates[CheckStateKey{IPID: ctx.IPID, CheckID: r.CheckID}]; ok {
			result = (state.Status == "DOWN")
		} else {
			result = false
		}

	case RuleTypeCheckHealthy:
		if state, ok := ctx.CheckStates[CheckStateKey{IPID: ctx.IPID, CheckID: r.CheckID}]; ok {
			result = (state.Status == "UP")
		} else {
			result = false
		}

	case RuleTypeGlobalCheckFailed:
		if len(ctx.IPIDs) == 0 {
			result = false
		} else {
			isFailed := true
			for _, otherIPID := range ctx.IPIDs {
				if state, ok := ctx.CheckStates[CheckStateKey{IPID: otherIPID, CheckID: r.CheckID}]; ok {
					if state.Status != "DOWN" {
						isFailed = false
						break
					}
				} else {
					isFailed = false
					break
				}
			}
			result = isFailed
		}

	case RuleTypeAnd:
		if len(r.Children) == 0 {
			result = false
		} else {
			allTrue := true
			for i := range r.Children {
				if !r.Children[i].Evaluate(ctx) {
					allTrue = false
					break
				}
			}
			result = allTrue
		}

	case RuleTypeOr:
		if len(r.Children) == 0 {
			result = false
		} else {
			anyTrue := false
			for i := range r.Children {
				if r.Children[i].Evaluate(ctx) {
					anyTrue = true
					break
				}
			}
			result = anyTrue
		}

	case RuleTypeNot:
		if r.Child != nil {
			result = !r.Child.Evaluate(ctx)
		} else {
			result = false
		}
	}

	slog.Debug("RuleExpr evaluated", "type", r.Type, "check_id", r.CheckID, "ip_id", ctx.IPID, "group_id", ctx.GroupID, "result", result)
	return result
}
