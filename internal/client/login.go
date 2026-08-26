package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LoginInitRequest starts a browser login. The code challenge is the S256
// hash of a PKCE verifier that never leaves this machine.
type LoginInitRequest struct {
	CodeChallenge string `json:"code_challenge"`
	MachineName   string `json:"machine_name"`
}

type LoginInitResponse struct {
	AuthURL string `json:"auth_url"`
	Ticket  string `json:"ticket"`
}

// LoginPollRequest proves possession of the PKCE code verifier: presenting
// it is what authorises collecting the token the browser login parked on
// the ticket.
type LoginPollRequest struct {
	Ticket       string `json:"ticket"`
	CodeVerifier string `json:"code_verifier"`
}

type LoginPollResponse struct {
	Token         string `json:"token"`
	WorkspaceSlug string `json:"workspace_slug"`
	Status        string `json:"status"`
}

// LoginInit begins a CLI login session on the platform.
func (client *Client) LoginInit(requestContext context.Context, initRequest LoginInitRequest) (*LoginInitResponse, error) {
	var initResponse LoginInitResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/cli/login/init", initRequest, &initResponse); requestError != nil {
		return nil, requestError
	}
	if initResponse.Ticket == "" {
		return nil, fmt.Errorf("the platform did not start a login session")
	}
	return &initResponse, nil
}

// LoginPoll collects the token parked on the login ticket. While the
// browser side of the login is still in progress the platform answers 202
// with {"status": "pending"}; that is reported as a pending response, not
// an error, so the caller keeps polling.
func (client *Client) LoginPoll(requestContext context.Context, pollRequest LoginPollRequest) (*LoginPollResponse, error) {
	request, buildError := client.newRequest(requestContext, http.MethodPost, "/api/v1/cli/login/poll", pollRequest)
	if buildError != nil {
		return nil, buildError
	}

	response, sendError := client.HTTP.Do(request)
	if sendError != nil {
		return nil, fmt.Errorf("could not reach %s: %w", client.BaseURL, sendError)
	}
	defer closeBody(response)

	responseBody, readError := readResponseBody(response)
	if readError != nil {
		return nil, fmt.Errorf("read response: %w", readError)
	}
	if statusError := errorFromResponse(response.StatusCode, responseBody); statusError != nil {
		return nil, statusError
	}

	var pollResponse LoginPollResponse
	if unmarshalError := json.Unmarshal(responseBody, &pollResponse); unmarshalError != nil {
		return nil, fmt.Errorf("parse response: %w", unmarshalError)
	}
	if response.StatusCode == http.StatusAccepted && pollResponse.Status == "" {
		pollResponse.Status = "pending"
	}
	return &pollResponse, nil
}
