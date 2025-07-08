package gorp

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// passwordGrantTokenSource is a custom implementation of oauth2.TokenSource that uses the
// Password Credentials grant to automatically fetch and refresh tokens.
type passwordGrantTokenSource struct {
	ctx    context.Context
	config *oauth2.Config

	user string
	pass string
}

// Token fetches a new token or returns the existing token if it is still valid.
func (p *passwordGrantTokenSource) Token() (*oauth2.Token, error) {
	// Fetch a new token using the Password Grant.
	token, err := p.config.PasswordCredentialsToken(p.ctx, p.user, p.pass)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch token: %w", err)
	}

	return token, nil
}

// NewPasswordGrantTokenSource creates an instance of passwordGrantTokenSource.
func NewPasswordGrantTokenSource(
	ctx context.Context,
	config *oauth2.Config,
	username, password string,
) oauth2.TokenSource {
	ts := &passwordGrantTokenSource{
		ctx:    ctx,
		config: config,
		user:   username,
		pass:   password,
	}
	return oauth2.ReuseTokenSource(nil, ts)
}
