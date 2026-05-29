// slack.go posts a rendered attachment to a Slack incoming webhook.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// SlackClient posts JSON payloads to Slack incoming webhook URLs.
type SlackClient struct {
	client *http.Client
}

// NewSlackClient returns a SlackClient. Per-request total
// deadlines come from the context passed to Post; the transport
// only bounds the TCP connect phase.
func NewSlackClient() *SlackClient {
	return &SlackClient{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Post sends a rendered attachment to a Slack incoming webhook.
//
// renderedAttachment is the JSON bytes produced by the template,
// expected to be a single JSON object describing one Slack
// attachment. This function wraps it as:
//
//	{"attachments": [<renderedAttachment>]}
//
// and POSTs to the given URL. The destination channel is bound to
// the webhook URL by Slack itself.
func (s *SlackClient) Post(ctx context.Context, url string, renderedAttachment []byte) error {
	// Validate that the rendered attachment is itself valid JSON
	// before wrapping it. This produces a clearer error message
	// than letting Slack reject the resulting payload.
	var probe json.RawMessage
	if err := json.Unmarshal(renderedAttachment, &probe); err != nil {
		return fmt.Errorf("rendered template is not valid JSON: %w", err)
	}

	body := slackPayload{
		Attachments: []json.RawMessage{probe},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, string(excerpt))
	}
	return nil
}

// slackPayload is the JSON body sent to a Slack incoming webhook.
// Only attachments is populated; other top-level fields (text,
// username, icon_emoji, channel, ...) are intentionally not
// surfaced. If those are needed, they belong inside the rendered
// attachment.
type slackPayload struct {
	Attachments []json.RawMessage `json:"attachments"`
}
