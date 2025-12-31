package app

import (
	"context"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

type authLoginFlags struct {
	username      string
	passwordStdin bool
	tokenName     string
	expiresAt     string
}

type authSetFlags struct {
	token      string
	tokenStdin bool
	apiURL     string
}

type authLogoutFlags struct {
	revoke bool
}

func authSetFlagSet(out io.Writer) (*flag.FlagSet, *authSetFlags) {
	opts := &authSetFlags{}
	flags := newFlagSet("auth set", out, printAuthSetUsage)
	flags.StringVar(&opts.token, "token", "", "Personal access token")
	flags.BoolVar(&opts.tokenStdin, "token-stdin", false, "Read token from stdin")
	flags.StringVar(&opts.apiURL, "api-url", "", "API base URL override")
	return flags, opts
}

func authLoginFlagSet(out io.Writer) (*flag.FlagSet, *authLoginFlags) {
	opts := &authLoginFlags{}
	flags := newFlagSet("auth login", out, printAuthLoginUsage)
	flags.StringVar(&opts.username, "username", "", "Username for login")
	flags.BoolVar(&opts.passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&opts.tokenName, "token-name", "cookctl", "Name for the new PAT")
	flags.StringVar(&opts.expiresAt, "expires-at", "", "Token expiration (RFC3339)")
	return flags, opts
}

func authStatusFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("auth status", out, printAuthStatusUsage)
}

func authWhoAmIFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("auth whoami", out, printAuthWhoAmIUsage)
}

func authLogoutFlagSet(out io.Writer) (*flag.FlagSet, *authLogoutFlags) {
	opts := &authLogoutFlags{}
	flags := newFlagSet("auth logout", out, printAuthLogoutUsage)
	flags.BoolVar(&opts.revoke, "revoke", false, "Revoke stored token before clearing credentials")
	return flags, opts
}

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
		usageErrorf(a.stderr, "unknown auth command: %s", args[0])
		printAuthUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runAuthSet(args []string) int {
	if hasHelpFlag(args) {
		printAuthSetUsage(a.stdout)
		return exitOK
	}

	flags, opts := authSetFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if opts.tokenStdin && strings.TrimSpace(opts.token) != "" {
		return usageError(a.stderr, "token and token-stdin cannot be combined")
	}
	if opts.tokenStdin {
		var err error
		opts.token, err = readToken(a.stdin)
		if err != nil {
			return usageError(a.stderr, err.Error())
		}
	}
	opts.token = strings.TrimSpace(opts.token)
	if opts.token == "" {
		return usageError(a.stderr, "token is required (use --token or --token-stdin)")
	}
	if strings.TrimSpace(opts.apiURL) == "" {
		opts.apiURL = a.cfg.APIURL
	}

	if err := a.store.Save(credentials.Credentials{
		Token:  strings.TrimSpace(opts.token),
		APIURL: strings.TrimSpace(opts.apiURL),
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

	flags, opts := authLoginFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if opts.username == "" {
		return usageError(a.stderr, "username is required")
	}
	if !opts.passwordStdin {
		return usageError(a.stderr, "password-stdin is required for auth login")
	}
	opts.tokenName = strings.TrimSpace(opts.tokenName)
	if opts.tokenName == "" {
		return usageError(a.stderr, "token-name is required")
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		return usageError(a.stderr, "password is required")
	}

	var expiresAtTime *time.Time
	if opts.expiresAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, opts.expiresAt)
		if parseErr != nil {
			return usageError(a.stderr, "expires-at must be RFC3339")
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

	resp, err := sessionClient.BootstrapToken(ctx, opts.username, password, opts.tokenName, expiresAtTime)
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

	flags := authStatusFlagSet(a.stderr)
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

	flags := authWhoAmIFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
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

	flags, opts := authLogoutFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if opts.revoke {
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
