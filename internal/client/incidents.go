package client

import (
	"context"
	"net/http"
)

type Incident struct {
	ID        string `json:"id"`
	AppID     string `json:"app_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Severity  string `json:"severity"`
	CreatedAt string `json:"created_at"`
}

type ListIncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
}

// ListIncidents returns the workspace's incidents, newest first.
func (client *Client) ListIncidents(requestContext context.Context) (*ListIncidentsResponse, error) {
	var listResponse ListIncidentsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/incidents", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}
