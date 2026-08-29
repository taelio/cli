package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Tael AI chat: a question in, the answer streamed back as server-sent
// events. The relay passes the agent's frames through as they come, so
// the client hands each frame up whole and lets the caller read it.

type ChatContext struct {
	AppID   string `json:"appId,omitempty"`
	Surface string `json:"surface,omitempty"`
}

type ChatRequest struct {
	Message        string      `json:"message"`
	Context        ChatContext `json:"context"`
	ConversationID string      `json:"conversation_id,omitempty"`
}

// SSEFrame is one server-sent event: its name ("message" when the stream
// gives none), its data lines joined with newlines, and its id if any.
type SSEFrame struct {
	Event string
	Data  string
	ID    string
}

// Ask sends a question and calls handle for each frame of the answer
// until handle returns false, the stream ends, or the context is
// cancelled. The API refuses an empty question with 422 and answers 503
// while Tael AI is still being set up for the workspace.
func (client *Client) Ask(requestContext context.Context, request ChatRequest, handle func(frame SSEFrame) bool) error {
	httpRequest, buildError := client.newRequest(requestContext, http.MethodPost, "/api/v1/chat", request)
	if buildError != nil {
		return buildError
	}
	httpRequest.Header.Set("Accept", "text/event-stream")

	response, sendError := client.StreamingHTTP.Do(httpRequest)
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
	var event, id string
	var dataLines []string
	emit := func() bool {
		if event == "" && len(dataLines) == 0 {
			return true
		}
		frame := SSEFrame{Event: event, Data: strings.Join(dataLines, "\n"), ID: id}
		if frame.Event == "" {
			frame.Event = "message"
		}
		event, id, dataLines = "", "", nil
		return handle(frame)
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if !emit() {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // a heartbeat
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			id = value
		}
	}
	if scanError := scanner.Err(); scanError != nil && requestContext.Err() == nil && scanError != io.EOF {
		return fmt.Errorf("answer interrupted: %w", scanError)
	}
	// A stream that ends without a trailing blank line still carries its
	// last frame; dropping the end of an answer is worse than being lenient.
	emit()
	return nil
}
