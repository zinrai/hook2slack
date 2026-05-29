package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// slackRecorder is an in-process Slack stand-in that records each
// received POST body.
type slackRecorder struct {
	mu       sync.Mutex
	requests [][]byte
	server   *httptest.Server
}

func newSlackRecorder(status int) *slackRecorder {
	r := &slackRecorder{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.requests = append(r.requests, body)
		r.mu.Unlock()
		w.WriteHeader(status)
	}))
	return r
}

func (r *slackRecorder) Close()      { r.server.Close() }
func (r *slackRecorder) URL() string { return r.server.URL }
func (r *slackRecorder) Count() int  { r.mu.Lock(); defer r.mu.Unlock(); return len(r.requests) }
func (r *slackRecorder) Last() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[len(r.requests)-1]
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func minimalFixtures(t *testing.T) (*Schema, *Template) {
	t.Helper()
	dir := t.TempDir()

	schemaPath := filepath.Join(dir, "schema.json")
	schemaBody := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["status"],
  "properties": {
    "status": { "type": "string", "enum": ["firing", "resolved"] }
  }
}`
	if err := os.WriteFile(schemaPath, []byte(schemaBody), 0o600); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadSchema(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(dir, "template.tmpl")
	tmplBody := `{"color": "good", "title": {{ .status | json }}}`
	if err := os.WriteFile(tmplPath, []byte(tmplBody), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := LoadTemplate(tmplPath)
	if err != nil {
		t.Fatal(err)
	}
	return schema, tmpl
}

// TestHandler_HappyPath exercises the full validate -> render ->
// post pipeline for a valid payload, and verifies the attachment
// is correctly wrapped. If this regresses, Slack messages are
// malformed.
func TestHandler_HappyPath(t *testing.T) {
	recorder := newSlackRecorder(200)
	defer recorder.Close()
	schema, tmpl := minimalFixtures(t)

	h := NewHandler(
		Endpoint{Path: "/x", URL: recorder.URL()},
		schema, tmpl, NewSlackClient(), silentLogger(),
	)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json",
		strings.NewReader(`{"status":"firing"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("HTTP %d: %s", resp.StatusCode, body)
	}
	if recorder.Count() != 1 {
		t.Fatalf("Slack post count: got %d, want 1", recorder.Count())
	}

	var got map[string]any
	if err := json.Unmarshal(recorder.Last(), &got); err != nil {
		t.Fatalf("Slack payload not valid JSON: %v", err)
	}
	if _, hasChannel := got["channel"]; hasChannel {
		t.Errorf("payload must not carry channel, got %v", got["channel"])
	}
	attachments, _ := got["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("attachments count: got %d, want 1", len(attachments))
	}
}

// TestHandler_SchemaRejectedDoesNotCallSlack guards the property
// that a payload failing schema validation does not reach Slack.
// If this regresses, malformed payloads from misconfigured senders
// flow through to chat channels.
func TestHandler_SchemaRejectedDoesNotCallSlack(t *testing.T) {
	recorder := newSlackRecorder(200)
	defer recorder.Close()
	schema, tmpl := minimalFixtures(t)

	h := NewHandler(
		Endpoint{Path: "/x", URL: recorder.URL()},
		schema, tmpl, NewSlackClient(), silentLogger(),
	)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json",
		strings.NewReader(`{"status":"bogus"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("HTTP: got %d, want 400", resp.StatusCode)
	}
	if recorder.Count() != 0 {
		t.Errorf("Slack should not have been called, got count=%d", recorder.Count())
	}
}

// TestHandler_SlackFailureReturns500 protects the retry-delegation
// contract documented in DESIGN.md: when Slack returns a non-2xx,
// hook2slack must return 5xx to the caller so the upstream sender
// (alertchain, Alertmanager, ...) knows to retry. If this regresses
// to e.g. returning 200 on Slack failure, alerts are silently lost.
func TestHandler_SlackFailureReturns500(t *testing.T) {
	recorder := newSlackRecorder(500)
	defer recorder.Close()
	schema, tmpl := minimalFixtures(t)

	h := NewHandler(
		Endpoint{Path: "/x", URL: recorder.URL()},
		schema, tmpl, NewSlackClient(), silentLogger(),
	)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json",
		strings.NewReader(`{"status":"firing"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("HTTP: got %d, want 500", resp.StatusCode)
	}
}
