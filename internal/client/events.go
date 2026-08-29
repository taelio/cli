package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// The workspace stream: everything Tael does, as it happens. Replay reads
// what happened since a cursor; follow tails it live. The cursor is the
// event id, so a follower resumes after a disconnect with since=<last id>.

// Event is one entry on the workspace stream. Payload is whatever the
// event carries; task events carry task_id, title, status and the app.
type Event struct {
	ID          int64           `json:"id"`
	OperationID *string         `json:"operation_id"`
	StepID      *string         `json:"step_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   string          `json:"created_at"`
}

type ListEventsResponse struct {
	Events []Event `json:"events"`
}

func eventsPath(since int64, stream bool) string {
	path := "/api/v1/events?since=" + strconv.FormatInt(since, 10)
	if stream {
		path += "&stream=true"
	}
	return path
}

// ListEvents reads what happened after the cursor (0 for everything the
// stream keeps), oldest first.
func (client *Client) ListEvents(requestContext context.Context, since int64) (*ListEventsResponse, error) {
	var listResponse ListEventsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, eventsPath(since, false), nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// FollowEvents tails the workspace stream from now, calling handle for
// each event until handle returns false, the stream ends, or the context
// is cancelled.
func (client *Client) FollowEvents(requestContext context.Context, handle func(event Event) bool) error {
	return client.FollowEventsSince(requestContext, 0, handle)
}

// FollowEventsSince tails the stream after the cursor. Frames are `id:`
// then `data:` with a JSON event; comment lines are heartbeats.
func (client *Client) FollowEventsSince(requestContext context.Context, since int64, handle func(event Event) bool) error {
	request, buildError := client.newRequest(requestContext, http.MethodGet, eventsPath(since, true), nil)
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
		line := scanner.Text()
		payload, isData := strings.CutPrefix(line, "data:")
		if !isData {
			continue
		}
		var event Event
		if unmarshalError := json.Unmarshal([]byte(strings.TrimPrefix(payload, " ")), &event); unmarshalError != nil {
			continue
		}
		if !handle(event) {
			return nil
		}
	}
	if scanError := scanner.Err(); scanError != nil && requestContext.Err() == nil && scanError != io.EOF {
		return fmt.Errorf("event stream interrupted: %w", scanError)
	}
	return nil
}
