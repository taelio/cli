package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LogsResponse struct {
	Lines []string `json:"lines"`
}

// GetLogs returns a snapshot of recent log lines for one app.
func (client *Client) GetLogs(requestContext context.Context, appIDOrName string) (*LogsResponse, error) {
	var logsResponse LogsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, "/logs"), nil, &logsResponse); requestError != nil {
		return nil, requestError
	}
	return &logsResponse, nil
}

// FollowLogs streams live log lines for one app over server-sent events,
// invoking handleLine for the payload of each `data:` frame. It returns
// when the stream ends, the context is cancelled, or the connection drops.
func (client *Client) FollowLogs(requestContext context.Context, appIDOrName string, handleLine func(line string)) error {
	request, buildError := client.newRequest(requestContext, http.MethodGet, appPath(appIDOrName, "/logs?follow=true"), nil)
	if buildError != nil {
		return buildError
	}
	request.Header.Set("Accept", "text/event-stream")

	response, sendError := client.StreamingHTTP.Do(request)
	if sendError != nil {
		return fmt.Errorf("could not reach %s: %w", client.BaseURL, sendError)
	}
	defer closeBody(response)

	if response.StatusCode != http.StatusOK {
		responseBody, _ := readResponseBody(response)
		return errorFromResponse(response.StatusCode, responseBody)
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		frame := scanner.Text()
		payload, isData := strings.CutPrefix(frame, "data:")
		if !isData {
			continue
		}
		handleLine(strings.TrimPrefix(payload, " "))
	}
	if scanError := scanner.Err(); scanError != nil && requestContext.Err() == nil && scanError != io.EOF {
		return fmt.Errorf("log stream interrupted: %w", scanError)
	}
	return nil
}
