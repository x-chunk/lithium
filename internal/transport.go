package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const DefaultBaseURL = "https://api.telegram.org"

// Transport sends Bot API calls over HTTP and unwraps the response envelope.
type Transport struct {
	HTTPClient *http.Client
	BaseURL    string
	Token      string
}

// Envelope is the common wrapper of every Bot API response.
type Envelope struct {
	Ok          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  *Parameters     `json:"parameters"`
}

type Parameters struct {
	MigrateToChatID int64 `json:"migrate_to_chat_id"`
	RetryAfter      int   `json:"retry_after"`
}

// Do posts body to the method endpoint and returns the decoded envelope.
// A non-ok envelope is returned as is, without an error: interpreting it is up to the caller.
func (t *Transport) Do(ctx context.Context, method, contentType string, body []byte) (*Envelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url(method), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := t.client().Do(req)
	if err != nil {
		// *url.Error embeds the request URL, which contains the bot token.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
	}
	return &env, nil
}

// DoJSON marshals payload as JSON and calls Do.
func (t *Transport) DoJSON(ctx context.Context, method string, payload any) (*Envelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	return t.Do(ctx, method, "application/json", body)
}

func (t *Transport) url(method string) string {
	base := t.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	return strings.TrimRight(base, "/") + "/bot" + t.Token + "/" + method
}

func (t *Transport) client() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return http.DefaultClient
}
