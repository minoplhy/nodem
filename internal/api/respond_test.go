package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondListEncodesNilAsEmptyArray(t *testing.T) {
	rec := httptest.NewRecorder()
	var items []string
	RespondList(rec, http.StatusOK, items)

	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("nil slice encoded as %q, want []", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestEmptyIfNilKeepsNestedSlicesAsArrays(t *testing.T) {
	resp := GroupStatusResponse{
		IPs:          emptyIfNil(GroupStatusResponse{}.IPs),
		Checks:       emptyIfNil(GroupStatusResponse{}.Checks),
		States:       emptyIfNil(GroupStatusResponse{}.States),
		UnmanagedIPs: emptyIfNil([]string(nil)),
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("response contains null: %s", raw)
	}

	// Non-nil input must be returned unchanged.
	in := []string{"a"}
	if out := emptyIfNil(in); len(out) != 1 || out[0] != "a" {
		t.Fatalf("emptyIfNil changed non-nil slice: %v", out)
	}
}
