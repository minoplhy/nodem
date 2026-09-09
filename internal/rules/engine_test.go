package rules

import (
	"encoding/json"
	"testing"

	"github.com/minoplhy/nodem/internal/db"
)

func TestRuleExprJSON(t *testing.T) {
	// 1. Leaf node: CheckFailed
	rawCheckFailed := `{"CheckFailed":{"check_id":42}}`
	var expr1 RuleExpr
	if err := json.Unmarshal([]byte(rawCheckFailed), &expr1); err != nil {
		t.Fatalf("Failed to unmarshal CheckFailed: %v", err)
	}
	if expr1.Type != RuleTypeCheckFailed || expr1.CheckID != 42 {
		t.Fatalf("Unexpected expr1: %+v", expr1)
	}

	bytes1, err := json.Marshal(expr1)
	if err != nil {
		t.Fatalf("Failed to marshal CheckFailed: %v", err)
	}
	if string(bytes1) != rawCheckFailed {
		t.Fatalf("Expected %s, got %s", rawCheckFailed, string(bytes1))
	}

	// 2. Complex nested node: And(Or(CheckFailed, CheckHealthy), Not(GlobalCheckFailed))
	rawComplex := `{"And":[{"Or":[{"CheckFailed":{"check_id":1}},{"CheckHealthy":{"check_id":2}}]},{"Not":{"GlobalCheckFailed":{"check_id":3}}}]}`
	var expr2 RuleExpr
	if err := json.Unmarshal([]byte(rawComplex), &expr2); err != nil {
		t.Fatalf("Failed to unmarshal complex: %v", err)
	}
	if expr2.Type != RuleTypeAnd || len(expr2.Children) != 2 {
		t.Fatalf("Unexpected expr2 type or children: %+v", expr2)
	}

	bytes2, err := json.Marshal(expr2)
	if err != nil {
		t.Fatalf("Failed to marshal complex: %v", err)
	}
	if string(bytes2) != rawComplex {
		t.Fatalf("Expected %s, got %s", rawComplex, string(bytes2))
	}
}

func TestRuleExprEvaluate(t *testing.T) {
	states := map[CheckStateKey]db.CheckState{
		{IPID: 10, CheckID: 100}: {IPID: 10, CheckID: 100, Status: "DOWN"},
		{IPID: 10, CheckID: 200}: {IPID: 10, CheckID: 200, Status: "UP"},
		{IPID: 20, CheckID: 100}: {IPID: 20, CheckID: 100, Status: "DOWN"},
		{IPID: 20, CheckID: 200}: {IPID: 20, CheckID: 200, Status: "DOWN"},
	}

	ctxIP10 := &EvaluationContext{
		IPID:        10,
		GroupID:     1,
		CheckStates: states,
		IPIDs:       []int64{10, 20},
	}

	// Rule 1: CheckFailed(100) -> true for IP 10
	r1 := RuleExpr{Type: RuleTypeCheckFailed, CheckID: 100}
	if !r1.Evaluate(ctxIP10) {
		t.Errorf("Expected r1 to evaluate to true")
	}

	// Rule 2: CheckHealthy(100) -> false for IP 10
	r2 := RuleExpr{Type: RuleTypeCheckHealthy, CheckID: 100}
	if r2.Evaluate(ctxIP10) {
		t.Errorf("Expected r2 to evaluate to false")
	}

	// Rule 3: CheckHealthy(200) -> true for IP 10
	r3 := RuleExpr{Type: RuleTypeCheckHealthy, CheckID: 200}
	if !r3.Evaluate(ctxIP10) {
		t.Errorf("Expected r3 to evaluate to true")
	}

	// Rule 4: GlobalCheckFailed(100) -> both 10 and 20 are DOWN on 100 -> true
	r4 := RuleExpr{Type: RuleTypeGlobalCheckFailed, CheckID: 100}
	if !r4.Evaluate(ctxIP10) {
		t.Errorf("Expected r4 to evaluate to true (global failure)")
	}

	// Rule 5: GlobalCheckFailed(200) -> IP 10 is UP on 200 -> false
	r5 := RuleExpr{Type: RuleTypeGlobalCheckFailed, CheckID: 200}
	if r5.Evaluate(ctxIP10) {
		t.Errorf("Expected r5 to evaluate to false")
	}

	// Rule 6: And(CheckFailed(100), CheckHealthy(200)) -> true
	r6 := RuleExpr{Type: RuleTypeAnd, Children: []RuleExpr{r1, r3}}
	if !r6.Evaluate(ctxIP10) {
		t.Errorf("Expected r6 to evaluate to true")
	}

	// Rule 7: Not(r6) -> false
	r7 := RuleExpr{Type: RuleTypeNot, Child: &r6}
	if r7.Evaluate(ctxIP10) {
		t.Errorf("Expected r7 to evaluate to false")
	}
}
