package app

import (
	"context"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

func (a *App) runAuth(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printAuthUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printAuthUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case "login":
		return a.runAuthLogin(args[1:])
	case "set":
		return a.runAuthSet(args[1:])
	case "status":
		return a.runAuthStatus(args[1:])
	case "whoami":
		return a.runAuthWhoAmI(args[1:])
	case "logout":
		return a.runAuthLogout(args[1:])
	default:
		writef(a.stderr, "unknown auth command: %s\n", args[0])
		printAuthUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runAuthSet(args []string) int {
	if hasHelpFlag(args) {
		printAuthSetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth set", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var token string
	var tokenStdin bool
	var apiURL string
	flags.StringVar(&token, "token", "", "Personal access token")
	flags.BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin")
	flags.StringVar(&apiURL, "api-url", "", "API base URL override")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if tokenStdin && strings.TrimSpace(token) != "" {
		writeLine(a.stderr, "token and token-stdin cannot be combined")
		return exitUsage
	}
	if tokenStdin {
		var err error
		token, err = readToken(a.stdin)
		if err != nil {
			writeLine(a.stderr, err)
			return exitUsage
		}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		writeLine(a.stderr, "token is required (use --token or --token-stdin)")
		return exitUsage
	}
	if strings.TrimSpace(apiURL) == "" {
		apiURL = a.cfg.APIURL
	}

	if err := a.store.Save(credentials.Credentials{
		Token:  strings.TrimSpace(token),
		APIURL: strings.TrimSpace(apiURL),
	}); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token saved"})
}

func (a *App) runAuthLogin(args []string) int {
	if hasHelpFlag(args) {
		printAuthLoginUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var username string
	var passwordStdin bool
	var tokenName string
	var expiresAt string

	flags.StringVar(&username, "username", "", "Username for login")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&tokenName, "token-name", "cookctl", "Name for the new PAT")
	flags.StringVar(&expiresAt, "expires-at", "", "Token expiration (RFC3339)")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if username == "" {
		writeLine(a.stderr, "username is required")
		return exitUsage
	}
	if !passwordStdin {
		writeLine(a.stderr, "password-stdin is required for auth login")
		return exitUsage
	}
	tokenName = strings.TrimSpace(tokenName)
	if tokenName == "" {
		writeLine(a.stderr, "token-name is required")
		return exitUsage
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		writeLine(a.stderr, "password is required")
		return exitUsage
	}

	var expiresAtTime *time.Time
	if expiresAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, expiresAt)
		if parseErr != nil {
			writeLine(a.stderr, "expires-at must be RFC3339")
			return exitUsage
		}
		expiresAtTime = &parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	if healthErr := a.ensureHealthyURL(ctx, a.cfg.APIURL); healthErr != nil {
		writeLine(a.stderr, healthErr)
		return exitError
	}

	sessionClient, err := client.NewSessionClient(a.cfg.APIURL, a.cfg.Timeout, a.cfg.Debug, a.stderr)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := sessionClient.BootstrapToken(ctx, username, password, tokenName, expiresAtTime)
	if err != nil {
		return a.handleAPIError(err)
	}

	creds := credentials.Credentials{
		Token:     resp.Token,
		TokenID:   resp.ID,
		TokenName: resp.Name,
		CreatedAt: &resp.CreatedAt,
		ExpiresAt: expiresAtTime,
		APIURL:    a.cfg.APIURL,
	}
	if err := a.store.Save(creds); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	return writeOutput(a.stdout, a.cfg.Output, authLoginResult{
		ID:        resp.ID,
		Name:      resp.Name,
		Token:     resp.Token,
		CreatedAt: resp.CreatedAt,
		ExpiresAt: expiresAtTime,
	})
}

func (a *App) runAuthStatus(args []string) int {
	if hasHelpFlag(args) {
		printAuthStatusUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	token, source, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	apiURL := a.cfg.APIURL
	var creds credentials.Credentials
	if source == tokenSourceCredentials {
		loadedCreds, ok, err := a.store.Load()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		if ok {
			creds = loadedCreds
			if !a.apiURLOverride {
				if storedURL := strings.TrimSpace(creds.APIURL); storedURL != "" {
					apiURL = storedURL
				}
			}
		}
	}

	status := authStatus{
		Source:       string(source),
		TokenPresent: token != "",
		MaskedToken:  maskToken(token),
		APIURL:       apiURL,
	}
	if source == tokenSourceCredentials {
		status.TokenID = creds.TokenID
		status.TokenName = creds.TokenName
		status.CreatedAt = creds.CreatedAt
		status.ExpiresAt = creds.ExpiresAt
	}
	return writeOutput(a.stdout, a.cfg.Output, status)
}

func (a *App) runAuthWhoAmI(args []string) int {
	if hasHelpFlag(args) {
		printAuthWhoAmIUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth whoami", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if token == "" {
		writeLine(a.stderr, "no token found; run `cookctl auth set --token <pat>`")
		return exitAuth
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := api.Me(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runAuthLogout(args []string) int {
	if hasHelpFlag(args) {
		printAuthLogoutUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var revoke bool
	flags.BoolVar(&revoke, "revoke", false, "Revoke stored token before clearing credentials")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if revoke {
		return a.runAuthLogoutRevoke()
	}

	if err := a.store.Clear(); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if env := os.Getenv("COOKING_PAT"); env != "" {
		return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "credentials cleared; COOKING_PAT is still set"})
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "credentials cleared"})
}

func (a *App) runAuthLogoutRevoke() int {
	creds, ok, err := a.store.Load()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if !ok || creds.Token == "" {
		writeLine(a.stderr, "no stored token found to revoke")
		return exitAuth
	}
	if creds.TokenID == "" {
		writeLine(a.stderr, "stored token id is missing; cannot revoke")
		return exitError
	}

	apiURL := a.cfg.APIURL
	if !a.apiURLOverride {
		if storedURL := strings.TrimSpace(creds.APIURL); storedURL != "" {
			apiURL = storedURL
		}
	}

	api, err := client.New(apiURL, creds.Token, a.cfg.Timeout, a.cfg.Debug, a.stderr)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	if err := a.ensureHealthy(ctx, api, apiURL); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if err := api.RevokeToken(ctx, creds.TokenID); err != nil {
		return a.handleAPIError(err)
	}

	if err := a.store.Clear(); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if env := os.Getenv("COOKING_PAT"); env != "" {
		return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token revoked and credentials cleared; COOKING_PAT is still set"})
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token revoked and credentials cleared"})
}
