package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// What Tael says without being asked: the digest of a window, and the
// suggestions its watcher posts.

type DigestFailedDeploy struct {
	App   string `json:"app"`
	When  string `json:"when"`
	Error string `json:"error"`
}

type DigestOpenIncident struct {
	App      string `json:"app"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	OpenFor  string `json:"open_for"`
}

type DigestOpenSuggestion struct {
	Message string `json:"message"`
	Offer   string `json:"offer"`
}

// DigestOpenApproval is one decision waiting on a person.
type DigestOpenApproval struct {
	Task       string `json:"task"`
	App        string `json:"app"`
	Ask        string `json:"ask"`
	Category   string `json:"category"`
	WaitingFor string `json:"waiting_for"`
}

// DigestFacts is what the database says happened; every number is exact.
type DigestFacts struct {
	WindowDays        int                    `json:"window_days"`
	From              string                 `json:"from"`
	To                string                 `json:"to"`
	DeploysTotal      int                    `json:"deploys_total"`
	DeploysSucceeded  int                    `json:"deploys_succeeded"`
	DeploysFailed     int                    `json:"deploys_failed"`
	AppsDeployed      []string               `json:"apps_deployed"`
	FailedDeploys     []DigestFailedDeploy   `json:"failed_deploys"`
	IncidentsOpened   int                    `json:"incidents_opened"`
	IncidentsResolved int                    `json:"incidents_resolved"`
	OpenIncidents     []DigestOpenIncident   `json:"open_incidents"`
	OpenSuggestions   []DigestOpenSuggestion `json:"open_suggestions"`
	NeedsYou          []DigestOpenApproval   `json:"needs_you"`
	NewMembers        int                    `json:"new_members"`
}

// DigestProse is what Tael wrote about the facts.
type DigestProse struct {
	Headline    string   `json:"headline"`
	WhatChanged string   `json:"what_changed"`
	WhatBroke   string   `json:"what_broke"`
	NeedsYou    []string `json:"needs_you"`
}

// Digest is facts plus prose. Prose is nil when nothing wrote any;
// Writing says a reading is being composed right now.
type Digest struct {
	Facts   DigestFacts  `json:"facts"`
	Prose   *DigestProse `json:"prose"`
	Model   string       `json:"model"`
	Writing bool         `json:"writing"`
}

type Suggestion struct {
	ID         string  `json:"id"`
	AppID      *string `json:"app_id"`
	Kind       string  `json:"kind"`
	Message    string  `json:"message"`
	Offer      string  `json:"offer"`
	CreatedAt  string  `json:"created_at"`
	ResolvedAt *string `json:"resolved_at"`
}

type ListSuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

// GetDigest reads what happened over the last days (the API's default
// window when zero).
func (client *Client) GetDigest(requestContext context.Context, days int) (*Digest, error) {
	path := "/api/v1/digest"
	if days > 0 {
		path += "?days=" + strconv.Itoa(days)
	}
	var digest Digest
	if requestError := client.doJSON(requestContext, http.MethodGet, path, nil, &digest); requestError != nil {
		return nil, requestError
	}
	return &digest, nil
}

// ListSuggestions returns what Tael noticed, newest first; open ones
// only unless includeResolved.
func (client *Client) ListSuggestions(requestContext context.Context, includeResolved bool) (*ListSuggestionsResponse, error) {
	path := "/api/v1/suggestions"
	if includeResolved {
		path += "?include_resolved=true"
	}
	var listResponse ListSuggestionsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, path, nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// ResolveSuggestion marks a suggestion as dealt with.
func (client *Client) ResolveSuggestion(requestContext context.Context, suggestionID string) error {
	return client.doJSON(requestContext, http.MethodPost, "/api/v1/suggestions/"+url.PathEscape(suggestionID)+"/resolve", struct{}{}, nil)
}
