// Package client is the HTTP client for the tael public API. All requests
// carry Bearer authentication and a tael-cli User-Agent; responses are
// decoded from snake_case JSON into typed structs defined in the per-area
// files (apps.go, deploys.go, login.go, ...).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBodySize = 10 * 1024 * 1024

// ErrUnauthorized is returned for 401 API responses so callers — and the
// CLI's exit-code classification — can detect authentication failures with
// errors.Is instead of matching message text.
var ErrUnauthorized = errors.New("unauthorized: run `tael login` to re-authenticate")

// APIError carries the HTTP status and the platform's error detail for any
// response the client has no specific handling for. The CLI's exit-code
// classification pattern-matches on it (401/403 → auth exit code).
type APIError struct {
	StatusCode int
	Detail     string
}

func (apiError *APIError) Error() string {
	if apiError == nil {
		return ""
	}
	if apiError.Detail == "" {
		return fmt.Sprintf("the platform returned status %d without details", apiError.StatusCode)
	}
	return fmt.Sprintf("%s (status %d)", apiError.Detail, apiError.StatusCode)
}

// Client is the base HTTP client for the tael API.
type Client struct {
	Token     string
	BaseURL   string
	UserAgent string

	// HTTP serves ordinary request/response calls and is bounded by a 30s
	// timeout. StreamingHTTP has no client-side timeout so long-lived SSE
	// streams (`tael logs -f`) are bounded only by the caller's context.
	HTTP          *http.Client
	StreamingHTTP *http.Client
}

// New constructs a Client. The base URL is stored without a trailing slash
// so request paths can be joined with plain concatenation.
func New(token string, baseURL string, version string) *Client {
	return &Client{
		Token:         token,
		BaseURL:       strings.TrimRight(baseURL, "/"),
		UserAgent:     "tael-cli/" + version,
		HTTP:          &http.Client{Timeout: 30 * time.Second},
		StreamingHTTP: &http.Client{},
	}
}

// newRequest builds an authenticated request for path (which must start
// with "/"), JSON-encoding requestBody when it is non-nil.
func (client *Client) newRequest(requestContext context.Context, method string, path string, requestBody any) (*http.Request, error) {
	var bodyReader io.Reader
	if requestBody != nil {
		encoded, marshalError := json.Marshal(requestBody)
		if marshalError != nil {
			return nil, fmt.Errorf("marshal request: %w", marshalError)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	request, requestError := http.NewRequestWithContext(requestContext, method, client.BaseURL+path, bodyReader)
	if requestError != nil {
		return nil, fmt.Errorf("create request: %w", requestError)
	}
	request.Header.Set("Authorization", "Bearer "+client.Token)
	request.Header.Set("User-Agent", client.UserAgent)
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// doJSON performs an authenticated JSON round trip. Non-2xx statuses become
// typed errors (ErrUnauthorized for 401, *APIError otherwise); a nil target
// discards the response body.
func (client *Client) doJSON(requestContext context.Context, method string, path string, requestBody any, target any) error {
	request, buildError := client.newRequest(requestContext, method, path, requestBody)
	if buildError != nil {
		return buildError
	}

	response, sendError := client.HTTP.Do(request)
	if sendError != nil {
		return fmt.Errorf("could not reach %s: %w", client.BaseURL, sendError)
	}
	defer closeBody(response)

	responseBody, readError := readResponseBody(response)
	if readError != nil {
		return fmt.Errorf("read response: %w", readError)
	}
	if statusError := errorFromResponse(response.StatusCode, responseBody); statusError != nil {
		return statusError
	}
	if target == nil {
		return nil
	}
	if unmarshalError := json.Unmarshal(responseBody, target); unmarshalError != nil {
		return fmt.Errorf("parse response: %w", unmarshalError)
	}
	return nil
}

// errorFromResponse classifies a non-2xx response into a typed error, or
// returns nil for success statuses.
func errorFromResponse(statusCode int, responseBody []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	if statusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	return &APIError{StatusCode: statusCode, Detail: detailFromBody(responseBody)}
}

// detailFromBody extracts the platform's {"detail": "..."} or
// {"error": "..."} message from a JSON error body, falling back to the raw
// (truncated) body text.
func detailFromBody(responseBody []byte) string {
	var parsed struct {
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	if unmarshalError := json.Unmarshal(responseBody, &parsed); unmarshalError == nil {
		if parsed.Detail != "" {
			return parsed.Detail
		}
		if parsed.Error != "" {
			return parsed.Error
		}
	}
	message := strings.TrimSpace(string(responseBody))
	if len(message) > 500 {
		message = message[:500] + "... (truncated)"
	}
	return message
}

func readResponseBody(response *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(response.Body, maxResponseBodySize))
}

func closeBody(response *http.Response) {
	_ = response.Body.Close()
}
