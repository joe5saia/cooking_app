# cookctl CLI

## Summary
`cookctl` is a companion CLI for the Cooking App API. It uses Personal Access Tokens (PATs) for authenticated requests and supports human-friendly tables or JSON output for scripting.

## Install (from this repo)
From the repo root:

```bash
make -C backend cookctl-build
./backend/bin/cookctl --help
```

Direct build (no Makefile):

```bash
go build -o /tmp/cookctl ./backend/cmd/cookctl
```

From `backend/`:

```bash
go build -o /tmp/cookctl ./cmd/cookctl
```

Run help:

```bash
/tmp/cookctl --help
/tmp/cookctl -h
```

CLI usage help:

```bash
/tmp/cookctl help
/tmp/cookctl help auth
```

Check the CLI version:

```bash
/tmp/cookctl version
```

Or use the global flag:

```bash
/tmp/cookctl --version
```

## Authentication
Bootstrap a PAT with your username and password:

```bash
printf '%s' 'password' | /tmp/cookctl auth login --username alice --password-stdin --token-name cookctl
```

Set an existing PAT:

```bash
/tmp/cookctl auth set --token pat_abc
```

Set a PAT from stdin (avoids shell history):

```bash
printf '%s' 'pat_abc' | /tmp/cookctl auth set --token-stdin
```

Set a PAT for a specific API URL:

```bash
/tmp/cookctl auth set --token pat_abc --api-url http://localhost:8080
```

Check the active token source:

```bash
/tmp/cookctl auth status
```

`auth status` prints the active source, resolved `api_url`, and stored token metadata when available.

Use an environment override for CI:

```bash
export COOKING_PAT=pat_abc
```

API URL resolution notes:
- `--api-url` always overrides stored credentials.
- If `COOKING_PAT` is set, the API URL comes from flags/config (stored URLs are ignored).
- If using stored credentials, the stored `api_url` is used when present.

## Configuration
Show the resolved config:

```bash
/tmp/cookctl config view
```

Update config values:

```bash
/tmp/cookctl config set --api-url http://localhost:8080 --output json --timeout 45s
```

Locate the config file:

```bash
/tmp/cookctl config path
```

## Common Commands
List recipes:

```bash
/tmp/cookctl recipe list --limit 25
```

Filter recipes by tag, book name, or servings:

```bash
/tmp/cookctl recipe list --tag "Dinner"
/tmp/cookctl recipe list --book "Weeknight"
/tmp/cookctl recipe list --servings 4
```

Include ingredient/step counts:

```bash
/tmp/cookctl recipe list --with-counts
```

Get a recipe:

```bash
/tmp/cookctl recipe get recipe-123
```

Get a recipe by title (fuzzy match):

```bash
/tmp/cookctl recipe get "Red Pasta"
```

Create a recipe from JSON:

```bash
/tmp/cookctl recipe create --file recipe.json
```

Create a recipe interactively:

```bash
/tmp/cookctl recipe create --interactive
```

Allow duplicate titles:

```bash
/tmp/cookctl recipe create --interactive --allow-duplicate
```

Create a recipe from stdin:

```bash
cat recipe.json | /tmp/cookctl recipe create --stdin
```

Update a recipe from stdin:

```bash
cat recipe.json | /tmp/cookctl recipe update recipe-123 --stdin
```

Generate a recipe template:

```bash
/tmp/cookctl recipe template > recipe.json
```

Export a recipe to JSON:

```bash
/tmp/cookctl recipe export recipe-123 > recipe.json
```

Import recipes from JSON:

```bash
/tmp/cookctl recipe import --file recipe.json
cat recipe.json | /tmp/cookctl recipe import --stdin
```

Tag recipes:

```bash
/tmp/cookctl recipe tag recipe-123 Dinner Quick
```

By default, missing tags are created. Use `--no-create-missing` to require existing tags.

Clone a recipe:

```bash
/tmp/cookctl recipe clone recipe-123
```

Edit a recipe in your editor:

```bash
/tmp/cookctl recipe edit recipe-123
```

Manage tags and books:

```bash
/tmp/cookctl tag list
/tmp/cookctl book create --name "Weeknight"
```

Meal plan commands:

```bash
/tmp/cookctl meal-plan list --start 2025-01-01 --end 2025-01-31
/tmp/cookctl meal-plan create --date 2025-01-03 --recipe-id recipe-123
/tmp/cookctl meal-plan delete --date 2025-01-03 --recipe-id recipe-123 --yes
```

## Output Modes
`--output table` (default) prints aligned columns for humans. `--output json` prints JSON suitable for scripting.
Successful responses are written to stdout; non-API errors are written to stderr.
When `--output json` is set, API errors are emitted to stdout as JSON:

```json
{
  "error": {
    "status": 403,
    "code": "forbidden",
    "message": "csrf missing",
    "details": [
      {
        "field": "csrf",
        "message": "required"
      }
    ]
  }
}
```
For `recipe list`, table output includes `next_cursor=<value>` when pagination is available.
For `recipe get`, table output includes ingredients and steps in readable sections.

## Shell Completion
Generate completion scripts:

```bash
/tmp/cookctl completion bash > /tmp/cookctl.bash
/tmp/cookctl completion zsh > /tmp/_cookctl
/tmp/cookctl completion fish > /tmp/cookctl.fish
```

## Environment Variables
- `COOKING_API_URL`
- `COOKING_PAT`
- `COOKING_TIMEOUT`
- `COOKING_OUTPUT`

## Notes
- Destructive commands require `--yes`.
- `--debug` logs request metadata and redacts secrets.
- Global flags (like `--output`, `--api-url`, `--timeout`, `--debug`) can appear before or after the command token (example: `/tmp/cookctl recipe list --output json`).
- Use `--skip-health-check` to bypass the API preflight when needed.
- Use `cookctl help <topic>` for per-command usage; subcommands also accept `--help` or `-h`.
