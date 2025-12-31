package app

import (
	"context"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

const tokenMissingMessage = "no token found; run `cookctl auth set --token <pat>`"

// requireToken resolves an auth token and handles standard error messaging.
func (a *App) requireToken() (string, int) {
	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return "", exitError
	}
	if token == "" {
		writeLine(a.stderr, tokenMissingMessage)
		return "", exitAuth
	}
	return token, exitOK
}

// authedClient resolves an auth token and constructs a client for API calls.
func (a *App) authedClient(ctx context.Context) (*client.Client, int) {
	token, exitCode := a.requireToken()
	if exitCode != exitOK {
		return nil, exitCode
	}

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return nil, exitError
	}

	return api, exitOK
}
