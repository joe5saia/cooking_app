# Product Document: cookctl (Cooking App Companion CLI)

## Summary
`cookctl` is a companion CLI for the Cooking App API. It targets developers, power users, and automation pipelines that need a fast, scriptable interface to manage recipes, tags, recipe books, and users. All post-bootstrap authenticated requests use Personal Access Tokens (PATs) via the HTTP bearer auth scheme.

## Problem Statement
The current web UI and raw API are useful but slow for power users and automation. A CLI enables quick inspection, batch edits, and integrations in scripts and CI without building custom clients. The CLI must be secure, consistent with the API contract, and friendly to both humans and automation.

## Goals
- Provide a stable, discoverable command set aligned with the API contract.
- Use PATs for all authenticated requests, with a safe bootstrap flow to create and store a PAT.
- Offer human-friendly output by default with a structured JSON mode for scripting.
- Support pagination, filtering, and safe destructive actions.
- Be easy to install across macOS, Linux, and Windows.

## Non-Goals
- Replacing the web UI or building a full TUI.
- Introducing new API behaviors or server-side features.
- Persisting local caches of server data.
- Managing user sessions beyond the initial PAT bootstrap flow.

## Target Users
- Developers integrating the API into scripts or automation.
- Admins and testers managing users, tags, and recipe books quickly.
- Power users who prefer terminal workflows.

## Key Use Cases
- List, filter, and paginate recipes quickly from the terminal.
- Create or update recipes from JSON or templated input.
- Manage tags and recipe books in bulk.
- Administer users (create, deactivate).
- Manage PATs (list, revoke), and bootstrap a new PAT.

## Authentication (PAT-First)
All post-bootstrap authenticated requests use `Authorization: Bearer <PAT>`. The CLI uses a temporary session cookie only to mint a PAT, then discards it.

CSRF-aware bootstrap flow (cookie auth):
1. `POST /api/v1/auth/login` with `{username,password}` using an in-memory cookie jar.
2. Read the `csrf_token` cookie value and include `X-CSRF-Token: <value>` on unsafe session-authenticated requests.
3. `POST /api/v1/tokens` with the cookie jar + `X-CSRF-Token` to mint the PAT.
4. `POST /api/v1/auth/logout` with the same CSRF header (best-effort cleanup).
5. Discard the cookie jar from memory.

Bootstrap options:
1. `cookctl auth login` performs the CSRF-aware bootstrap flow and stores the PAT.
2. `cookctl auth set --token` stores a user-provided PAT.
3. Environment variable `COOKING_PAT` overrides stored credentials for CI or scripts.

Token storage:
- Linux: `$XDG_CONFIG_HOME/cookctl/credentials.json` (fallback `~/.config/cookctl/credentials.json`).
- macOS: `~/Library/Application Support/cookctl/credentials.json`.
- Windows: `%AppData%\\cookctl\\credentials.json`.
- Config is stored alongside credentials as `config.toml`.
- POSIX permissions: directory 0700, file 0600. Windows: best-effort per-user ACL.
- Optional future enhancement: OS keychain integration.

## Configuration
Precedence order:
1. Flags
2. Environment variables
3. Config file
4. Built-in defaults

API URL nuance:
- When a stored PAT is used, the stored `api_url` is preferred unless `--api-url` is provided.
- When `COOKING_PAT` is set, stored `api_url` is ignored and flags/config apply.

Defaults:
- API base URL: `http://localhost:8080`
- Request timeout: `30s`

Environment variables:
- `COOKING_API_URL`
- `COOKING_PAT`
- `COOKING_TIMEOUT`
- `COOKING_OUTPUT` (`table` or `json`)

## Command Design
Command groups map to API resources and behaviors. All commands support `--output` (`table`|`json`) and `--api-url`.

Global flags:
- `--api-url <url>`
- `--output <table|json>`
- `--timeout <duration>`
- `--debug` (prints request/response metadata, redacts secrets)
- `--version` (prints build metadata and exits)
- `--help` / `-h` (prints CLI usage and exits)
Note: global flags may be provided before or after the command token (example: `cookctl recipe list --output json`).

Auth:
- `cookctl auth login --username <u> --password-stdin --token-name <name> [--expires-at <rfc3339>]`
- `cookctl auth set --token <pat> [--api-url <url>]`
- `cookctl auth set --token-stdin [--api-url <url>]`
- `cookctl auth status`
- `cookctl auth whoami` (calls `GET /api/v1/auth/me`)
- `cookctl auth logout [--revoke]` (clears local credentials; optional revoke if token id is known)

Tokens:
- `cookctl token list`
- `cookctl token create --name <name> [--expires-at <rfc3339>]`
- `cookctl token revoke <id> [--yes]`
  - When `--expires-at` is omitted, the CLI warns that the token will not expire.

Users:
- `cookctl user list`
- `cookctl user create --username <u> --password-stdin [--display-name <name>]`
- `cookctl user deactivate <id> [--yes]`

Recipe books:
- `cookctl book list`
- `cookctl book create --name <name>`
- `cookctl book update <id> --name <name>`
- `cookctl book delete <id> [--yes]`

Tags:
- `cookctl tag list`
- `cookctl tag create --name <name>`
- `cookctl tag update <id> --name <name>`
- `cookctl tag delete <id> [--yes]`

Recipes:
- `cookctl recipe list [--q <text>] [--book-id <id>] [--tag-id <id>] [--include-deleted] [--limit <n>] [--cursor <c>] [--all]`
- `cookctl recipe get <id>`
- `cookctl recipe create [--file <json> | --stdin]`
- `cookctl recipe update <id> [--file <json> | --stdin]`
- `cookctl recipe init [<id>]` (prints a valid upsert payload template)
- `cookctl recipe edit <id>` (opens an upsert payload in `$EDITOR`, then submits)
- `cookctl recipe delete <id> [--yes]`
- `cookctl recipe restore <id> [--yes]`

Health:
- `cookctl health` (calls `GET /api/v1/healthz`)

Version:
- `cookctl version` (prints build metadata)

Completion:
- `cookctl completion <bash|zsh|fish>` (prints shell completion scripts)

Help:
- `cookctl help [command]` (prints usage for the CLI or a specific command; subcommands do not accept `--help`)

Config:
- `cookctl config view`
- `cookctl config set [--api-url <url>] [--output <table|json>] [--timeout <duration>]`
- `cookctl config unset [--api-url] [--output] [--timeout]`
- `cookctl config path`

## Output Formats
- `table` (default): aligned columns, concise fields, paging disabled by default.
- `json`: exact API response, suitable for `jq` or scripts; for `recipe list` this is the `{items,next_cursor}` object.
- Errors are human-readable and written to stderr; successful JSON responses are written to stdout.
- When `--output json` is set, API errors are emitted to stdout in an `{error:{status,code,message,details}}` envelope.

## Error Handling
The CLI maps API Problem responses to actionable messages:
- Primary line: `<code>: <message>`
- Field errors (validation): `field=<name> message=<msg>`
- Exit codes:
  - `1` for generic errors
  - `2` for usage/flag errors (including invalid JSON input)
  - `3` for unauthorized
  - `4` for not found
  - `5` for conflict
  - `6` for rate limited (with retry advice)
  - `7` for forbidden (CSRF or policy failures)
  - `8` for request too large
- Forbidden errors should include guidance to re-run `auth login` and verify CSRF handling.

## Safety and Destructive Actions
- Commands that delete or revoke require confirmation unless `--yes` is set.
- `--yes` is accepted only on destructive commands, not globally.

## Pagination and Filtering
- `recipe list --all` will follow `next_cursor` until all items are collected.
- `--limit` and `--cursor` map directly to the API.
- Other list endpoints return full arrays in v1; the CLI may truncate table output via `--limit` and should adopt server pagination if it is added later.
- When `--output table` is used, `recipe list` prints `next_cursor=<value>` if a next page is available.

## Data Input
- JSON payloads are accepted via `--file` or `--stdin`.
- `recipe init` and `recipe edit` emit valid upsert payloads (no server-managed fields, `tag_ids` instead of `tags`).
- `recipe update` uses replace semantics; the input file must include all required fields.
- The CLI validates JSON syntax and ensures the payload is non-empty; the API validates request schema.
- Inputs are not logged when `--debug` is enabled.

## Security Considerations
- PATs are never echoed in logs.
- POSIX credentials file permissions are 0600; Windows uses best-effort per-user ACLs.
- `--debug` redacts `Authorization`, `Cookie`, and `Set-Cookie` headers.
- `--debug` redacts response fields named `token` (PAT secrets).
- Prefer `auth set --token-stdin` or `COOKING_PAT` to avoid leaking tokens into shell history.
- The CLI never writes to `.env` or reads secrets from it.

## API Mapping
| CLI Command | Endpoint |
| --- | --- |
| `cookctl auth whoami` | `GET /api/v1/auth/me` |
| `cookctl token list` | `GET /api/v1/tokens` |
| `cookctl token create` | `POST /api/v1/tokens` |
| `cookctl token revoke` | `DELETE /api/v1/tokens/{id}` |
| `cookctl user list` | `GET /api/v1/users` |
| `cookctl user create` | `POST /api/v1/users` |
| `cookctl user deactivate` | `PUT /api/v1/users/{id}/deactivate` |
| `cookctl book list` | `GET /api/v1/recipe-books` |
| `cookctl book create` | `POST /api/v1/recipe-books` |
| `cookctl book update` | `PUT /api/v1/recipe-books/{id}` |
| `cookctl book delete` | `DELETE /api/v1/recipe-books/{id}` |
| `cookctl tag list` | `GET /api/v1/tags` |
| `cookctl tag create` | `POST /api/v1/tags` |
| `cookctl tag update` | `PUT /api/v1/tags/{id}` |
| `cookctl tag delete` | `DELETE /api/v1/tags/{id}` |
| `cookctl recipe list` | `GET /api/v1/recipes` |
| `cookctl recipe get` | `GET /api/v1/recipes/{id}` |
| `cookctl recipe create` | `POST /api/v1/recipes` |
| `cookctl recipe update` | `PUT /api/v1/recipes/{id}` |
| `cookctl recipe delete` | `DELETE /api/v1/recipes/{id}` |
| `cookctl recipe restore` | `PUT /api/v1/recipes/{id}/restore` |
| `cookctl health` | `GET /api/v1/healthz` |

## HTTP Semantics
- `POST /api/v1/recipes` returns `201` with a response body.
- `PUT /api/v1/recipes/{id}/restore` returns `204` with no body.
- Create endpoints for users, tags, recipe books, and tokens return `200` with a response body.
- The client should treat any 2xx as success and only decode JSON when a body is expected.

## Implementation Notes (Go)
- Use Go `net/http` with a shared client and context timeouts.
- Keep command handlers thin; share request/response logic in a client package.
- Validate JSON syntax before sending request bodies.
- Use Go's `flag` package with explicit command trees.
- Provide shell completion scripts.

## Testing Strategy
- Unit tests for command parsing and request building.
- Golden tests for `table` output rendering.
- Integration tests against a local server via `make dev-up` (optional).
- Contract checks against `backend/openapi.yaml` to keep CLI mapping in sync.

## Release and Distribution
- Publish a versioned `cookctl` binary for macOS/Linux/Windows.
- Provide Homebrew tap and GitHub Releases.
- Document installation in `README.md`.

## Open Questions
- Should `auth login` be allowed in CI, or require `--non-interactive` for clarity?
- Should `recipe create/update` accept YAML as an input format?

## Future Enhancements
- OS keychain support for PAT storage.
- `--watch` mode for recipe list filtering.
- Templated interactive recipe creation.
