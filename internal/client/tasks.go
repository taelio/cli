package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// The Activity surface: tasks, their plans, evidence and outcomes, the
// decisions that wait on a person, and the workspace's settings for how
// much Tael may do on its own. Field names follow the API's JSON.

type TaskApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskOutcome struct {
	Summary string `json:"summary"`
	Next    string `json:"next"`
}

type Task struct {
	ID          string       `json:"id"`
	Kind        string       `json:"kind"`
	Title       string       `json:"title"`
	Brief       string       `json:"brief"`
	Status      string       `json:"status"`
	Origin      string       `json:"origin"`
	App         *TaskApp     `json:"app"`
	Risk        *string      `json:"risk"`
	NeedsYou    bool         `json:"needs_you"`
	RequestedBy string       `json:"requested_by"`
	ParentID    *string      `json:"parent_id"`
	Outcome     *TaskOutcome `json:"outcome"`
	CreatedAt   string       `json:"created_at"`
	StartedAt   *string      `json:"started_at"`
	FinishedAt  *string      `json:"finished_at"`
	UpdatedAt   string       `json:"updated_at"`
}

type PlanStep struct {
	ID            string  `json:"id"`
	Position      int     `json:"position"`
	Title         string  `json:"title"`
	Detail        string  `json:"detail"`
	Risk          string  `json:"risk"`
	Reversible    bool    `json:"reversible"`
	NeedsApproval bool    `json:"needs_approval"`
	Status        string  `json:"status"`
	Error         *string `json:"error"`
}

type Artifact struct {
	ID         string  `json:"id"`
	StepID     *string `json:"step_id"`
	Kind       string  `json:"kind"`
	Subkind    string  `json:"subkind"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	URL        *string `json:"url"`
	OK         *bool   `json:"ok"`
	CapturedAt string  `json:"captured_at"`
}

type Approval struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id"`
	StepID      *string `json:"step_id"`
	Category    string  `json:"category"`
	Risk        string  `json:"risk"`
	Reversible  bool    `json:"reversible"`
	Summary     string  `json:"summary"`
	Status      string  `json:"status"`
	RequestedAt string  `json:"requested_at"`
	ExpiresAt   *string `json:"expires_at"`
	DecidedAt   *string `json:"decided_at"`
	DecidedBy   *string `json:"decided_by"`
	Note        *string `json:"note"`
}

type TaskComment struct {
	ID         string  `json:"id"`
	Author     string  `json:"author"`
	AuthorName *string `json:"author_name"`
	Body       string  `json:"body"`
	CreatedAt  string  `json:"created_at"`
}

type TaskDetail struct {
	Task      Task          `json:"task"`
	Plan      []PlanStep    `json:"plan"`
	Changes   []Artifact    `json:"changes"`
	Evidence  []Artifact    `json:"evidence"`
	Outcome   *TaskOutcome  `json:"outcome"`
	Approvals []Approval    `json:"approvals"`
	Comments  []TaskComment `json:"comments"`
}

type ListTasksResponse struct {
	Tasks []Task `json:"tasks"`
}

type AISettings struct {
	Plan             string  `json:"plan"`
	Paused           bool    `json:"paused"`
	PausedAt         *string `json:"paused_at"`
	PausedBy         *string `json:"paused_by"`
	Approvers        string  `json:"approvers"`
	Budget           int     `json:"budget"`
	ActionsThisMonth int     `json:"actions_this_month"`
}

// ListTasks lists the workspace's tasks. status is open, done or all
// (the API defaults to open); app narrows to one app by id or name.
func (client *Client) ListTasks(requestContext context.Context, status string, app string) (*ListTasksResponse, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if app != "" {
		query.Set("app", app)
	}
	path := "/api/v1/tasks"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var listResponse ListTasksResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, path, nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// GetTask reads one task with its plan, evidence, outcome, approvals and
// comments.
func (client *Client) GetTask(requestContext context.Context, taskID string) (*TaskDetail, error) {
	var detail TaskDetail
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/tasks/"+url.PathEscape(taskID), nil, &detail); requestError != nil {
		return nil, requestError
	}
	return &detail, nil
}

// CreateTask asks Tael to look into something, in the caller's words.
// app and kind are optional; the API defaults the kind to investigate.
func (client *Client) CreateTask(requestContext context.Context, brief string, app string, kind string) (*TaskDetail, error) {
	body := map[string]string{"brief": brief}
	if app != "" {
		body["app"] = app
	}
	if kind != "" {
		body["kind"] = kind
	}
	var detail TaskDetail
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/tasks", body, &detail); requestError != nil {
		return nil, requestError
	}
	return &detail, nil
}

func (client *Client) decide(requestContext context.Context, taskID string, verdict string, note string) (*TaskDetail, error) {
	body := map[string]string{}
	if note != "" {
		body["note"] = note
	}
	var detail TaskDetail
	path := "/api/v1/tasks/" + url.PathEscape(taskID) + "/" + verdict
	if requestError := client.doJSON(requestContext, http.MethodPost, path, body, &detail); requestError != nil {
		return nil, requestError
	}
	return &detail, nil
}

// ApproveTask says yes to the task's pending approval, or to a proposal
// as a whole, with an optional note for the record.
func (client *Client) ApproveTask(requestContext context.Context, taskID string, note string) (*TaskDetail, error) {
	return client.decide(requestContext, taskID, "approve", note)
}

// DeclineTask says no, with an optional note for the record.
func (client *Client) DeclineTask(requestContext context.Context, taskID string, note string) (*TaskDetail, error) {
	return client.decide(requestContext, taskID, "decline", note)
}

// GetAISettings reads how much Tael may do in the workspace.
func (client *Client) GetAISettings(requestContext context.Context) (*AISettings, error) {
	var settings AISettings
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/workspace/ai-settings", nil, &settings); requestError != nil {
		return nil, requestError
	}
	return &settings, nil
}

// SetPaused pauses or resumes Tael for the whole workspace.
func (client *Client) SetPaused(requestContext context.Context, paused bool) (*AISettings, error) {
	var settings AISettings
	body := map[string]bool{"paused": paused}
	if requestError := client.doJSON(requestContext, http.MethodPut, "/api/v1/workspace/ai-settings", body, &settings); requestError != nil {
		return nil, requestError
	}
	return &settings, nil
}

// Event is one entry on the workspace stream. Payload is whatever the
// event carries; task events carry task_id, title, status and the app.
type Event struct {
	ID        int64           `json:"id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"created_at"`
}

// FollowEvents tails the workspace stream, calling handle for each event
// until handle returns false, the stream ends, or the context is
// cancelled. Frames are `id:` then `data:` with a JSON event; comment
// lines are heartbeats.
func (client *Client) FollowEvents(requestContext context.Context, handle func(event Event) bool) error {
	request, buildError := client.newRequest(requestContext, http.MethodGet, "/api/v1/events?stream=true", nil)
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
