package tokenissuer

import "context"

type IssueRequest struct {
	Subject string
	Roles   []string
}

type IssueResponse struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
}

type Issuer interface {
	Name() string
	Issue(ctx context.Context, req IssueRequest) (IssueResponse, error)
	Validate(ctx context.Context, accessToken string) (IssueRequest, error)
}
