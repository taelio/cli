package client

import (
	"context"
	"net/http"
	"net/url"
)

// The people half of a workspace: who is in it, who has been invited, and
// how the workspace admits people.

type Member struct {
	UserID      string  `json:"user_id"`
	Name        *string `json:"name"`
	Email       *string `json:"email"`
	GithubLogin *string `json:"github_login"`
	AvatarURL   *string `json:"avatar_url"`
	Role        string  `json:"role"`
	JoinedAt    string  `json:"joined_at"`
	InvitedVia  *string `json:"invited_via"`
}

// ListMembersResponse carries the members, how the workspace admits
// people (invite_only or github_repo_access) and the caller's own role.
type ListMembersResponse struct {
	Members    []Member `json:"members"`
	JoinPolicy string   `json:"join_policy"`
	YourRole   string   `json:"your_role"`
}

type Invitation struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Role        string  `json:"role"`
	Email       *string `json:"email"`
	GithubLogin *string `json:"github_login"`
	CreatedBy   string  `json:"created_by"`
	ExpiresAt   string  `json:"expires_at"`
	MaxUses     *int    `json:"max_uses"`
	Uses        int     `json:"uses"`
	RevokedAt   *string `json:"revoked_at"`
	CreatedAt   string  `json:"created_at"`
	Status      string  `json:"status"`
}

type ListInvitationsResponse struct {
	Invitations []Invitation `json:"invitations"`
}

// NewInvitation is one to create: by link (anyone with it), by email or by
// GitHub login. MaxUses bounds a link; zero is unlimited.
type NewInvitation struct {
	Kind        string `json:"kind"`
	Role        string `json:"role,omitempty"`
	Email       string `json:"email,omitempty"`
	GithubLogin string `json:"github_login,omitempty"`
	MaxUses     int    `json:"max_uses,omitempty"`
}

// CreatedInvitation carries the join URL, the only time the secret exists
// in readable form.
type CreatedInvitation struct {
	Invitation Invitation `json:"invitation"`
	JoinURL    string     `json:"join_url"`
}

type JoinPolicyResponse struct {
	JoinPolicy string `json:"join_policy"`
}

// ListMembers returns who is in the workspace.
func (client *Client) ListMembers(requestContext context.Context) (*ListMembersResponse, error) {
	var listResponse ListMembersResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/members", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// RemoveMember takes a person out of the workspace. Owners and admins
// only; the API refuses to remove the last owner.
func (client *Client) RemoveMember(requestContext context.Context, userID string) error {
	return client.doJSON(requestContext, http.MethodDelete, "/api/v1/members/"+url.PathEscape(userID), nil, nil)
}

// ListInvitations returns the workspace's invitations. Owners and admins only.
func (client *Client) ListInvitations(requestContext context.Context) (*ListInvitationsResponse, error) {
	var listResponse ListInvitationsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/invitations", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// CreateInvitation makes one and returns its join URL, once.
func (client *Client) CreateInvitation(requestContext context.Context, invitation NewInvitation) (*CreatedInvitation, error) {
	var created CreatedInvitation
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/invitations", invitation, &created); requestError != nil {
		return nil, requestError
	}
	return &created, nil
}

// RevokeInvitation makes an invitation stop working.
func (client *Client) RevokeInvitation(requestContext context.Context, invitationID string) error {
	return client.doJSON(requestContext, http.MethodDelete, "/api/v1/invitations/"+url.PathEscape(invitationID), nil, nil)
}

// SetJoinPolicy sets how the workspace admits people: invite_only, or
// github_repo_access for anyone with access to its repositories.
func (client *Client) SetJoinPolicy(requestContext context.Context, joinPolicy string) (*JoinPolicyResponse, error) {
	var response JoinPolicyResponse
	body := map[string]string{"join_policy": joinPolicy}
	if requestError := client.doJSON(requestContext, http.MethodPut, "/api/v1/workspace/join-policy", body, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}
