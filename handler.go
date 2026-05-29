// handler.go implements the per-endpoint HTTP handler.
package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// Handler is the per-endpoint HTTP handler.
type Handler struct {
	endpoint Endpoint
	schema   *Schema
	template *Template
	slack    *SlackClient
	logger   *slog.Logger
}

// NewHandler returns a Handler bound to one endpoint.
func NewHandler(ep Endpoint, schema *Schema, tmpl *Template, slack *SlackClient,
	logger *slog.Logger) *Handler {
	return &Handler{
		endpoint: ep,
		schema:   schema,
		template: tmpl,
		slack:    slack,
		logger:   logger.With("path", ep.Path),
	}
}

const maxBodyBytes = 10 << 20 // 10 MiB

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		http.Error(w, "parse body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.schema.Validate(value); err != nil {
		h.logger.Warn("schema validation failed", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rendered, err := h.template.Render(value)
	if err != nil {
		h.logger.Error("template render failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.slack.Post(r.Context(), h.endpoint.URL, rendered); err != nil {
		h.logger.Error("slack post failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
