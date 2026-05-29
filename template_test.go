package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestTemplate_JsonFunc protects the only custom function we add
// to text/template. If json-encoding is broken, the rendered output
// is not valid JSON, and every Slack POST will fail.
//
// We use inputs that the default Go template escaping
// (printf "%q" and similar) would mishandle: a string with a
// newline, a quote, and an "=" sign.
func TestTemplate_JsonFunc(t *testing.T) {
	tmplPath := filepath.Join(t.TempDir(), "template.tmpl")
	body := `{"text": {{ .msg | json }}}`
	if err := os.WriteFile(tmplPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := LoadTemplate(tmplPath)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	rendered, err := tmpl.Render(map[string]any{
		"msg": "line one\nline two with \"quote\" and = sign",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var probe map[string]any
	if err := json.Unmarshal(rendered, &probe); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v\noutput: %s", err, rendered)
	}
	if got := probe["text"]; got != "line one\nline two with \"quote\" and = sign" {
		t.Errorf("string roundtrip failed: got %q", got)
	}
}

// TestTemplate_Example renders the bundled example template against
// the bundled example payload. The example is documentation — if it
// stops rendering, users following the README are misled.
func TestTemplate_Example(t *testing.T) {
	tmpl, err := LoadTemplate(filepath.Join("examples", "template.tmpl"))
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}

	payloadBytes, err := os.ReadFile(filepath.Join("examples", "payload.json"))
	if err != nil {
		t.Fatalf("read payload.json: %v", err)
	}
	var payload any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("parse payload.json: %v", err)
	}

	rendered, err := tmpl.Render(payload)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(rendered, &got); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v\noutput: %s", err, rendered)
	}
	// payload.json has level=warning, so the rendered color should
	// be "warning". If this assertion fails, either the example
	// payload, the example template, or both have drifted.
	if got["color"] != "warning" {
		t.Errorf("color: got %v, want warning", got["color"])
	}
}
