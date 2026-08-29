package client

import (
	"context"
	"net/http"
	"net/url"
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

// PreApproval is the monthly allowance for a category Tael may act on
// without asking.
type PreApproval struct {
	PerMonth int `json:"per_month"`
}

// QuietHours is when scheduled work keeps quiet, as "HH:MM" in a zone.
type QuietHours struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// AISettings is how much Tael may do in the workspace and who decides.
type AISettings struct {
	Plan                    string                 `json:"plan"`
	Paused                  bool                   `json:"paused"`
	PausedAt                *string                `json:"paused_at"`
	PausedBy                *string                `json:"paused_by"`
	PreApproved             map[string]PreApproval `json:"pre_approved"`
	QuietHours              *QuietHours            `json:"quiet_hours"`
	Approvers               string                 `json:"approvers"`
	AllowanceExhaustedUntil *string                `json:"allowance_exhausted_until"`
	Budget                  int                    `json:"budget"`
	ActionsThisMonth        int                    `json:"actions_this_month"`
}

// AISettingsUpdate is what a PUT may change; a nil field keeps its value.
// PreApproved replaces the whole map, so callers merge first.
type AISettingsUpdate struct {
	Paused          *bool                   `json:"paused,omitempty"`
	PreApproved     *map[string]PreApproval `json:"pre_approved,omitempty"`
	QuietHours      *QuietHours             `json:"quiet_hours,omitempty"`
	ClearQuietHours bool                    `json:"clear_quiet_hours,omitempty"`
	Approvers       *string                 `json:"approvers,omitempty"`
}

// NeedsYouItem is one thing waiting on a person: the approval and the
// task it belongs to (no approval for a proposal as a whole).
type NeedsYouItem struct {
	Task     Task      `json:"task"`
	Approval *Approval `json:"approval"`
}

type NeedsYouResponse struct {
	Items []NeedsYouItem `json:"items"`
	Count int            `json:"count"`
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
	return client.UpdateAISettings(requestContext, AISettingsUpdate{Paused: &paused})
}

// UpdateAISettings changes how much Tael may do. Owners and admins only;
// the API refuses a bad value with a sentence.
func (client *Client) UpdateAISettings(requestContext context.Context, update AISettingsUpdate) (*AISettings, error) {
	var settings AISettings
	if requestError := client.doJSON(requestContext, http.MethodPut, "/api/v1/workspace/ai-settings", update, &settings); requestError != nil {
		return nil, requestError
	}
	return &settings, nil
}

// AddTaskComment leaves a comment on a task, for Tael and the team.
func (client *Client) AddTaskComment(requestContext context.Context, taskID string, body string) (*TaskComment, error) {
	var response struct {
		Comment TaskComment `json:"comment"`
	}
	path := "/api/v1/tasks/" + url.PathEscape(taskID) + "/comments"
	if requestError := client.doJSON(requestContext, http.MethodPost, path, map[string]string{"body": body}, &response); requestError != nil {
		return nil, requestError
	}
	return &response.Comment, nil
}

// NeedsYou lists what is waiting on a person: pending approvals and open
// proposals, with their tasks.
func (client *Client) NeedsYou(requestContext context.Context) (*NeedsYouResponse, error) {
	var response NeedsYouResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/needs-you", nil, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}
