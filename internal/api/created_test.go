package api

import (
	"encoding/json"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

// sessionOf is the session of a POST /api/sessions 201 body, as JSON.
func sessionOf(t testing.TB, body []byte) []byte {
	t.Helper()
	var result struct {
		Session json.RawMessage `json:"session"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Session == nil {
		t.Fatalf("create result %s: %v", body, err)
	}
	return result.Session
}

// createdInfo decodes the session of a POST /api/sessions 201 body.
func createdInfo(t testing.TB, body []byte) contract.SessionInfo {
	t.Helper()
	var info contract.SessionInfo
	if err := json.Unmarshal(sessionOf(t, body), &info); err != nil {
		t.Fatal(err)
	}
	return info
}
