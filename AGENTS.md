## Project Overview

Monorepo:
- `backend/`: Go API + CLI (Go `1.25.5` via `go.work`)
- `frontend/`: React + TypeScript + Vite
- `compose.yaml`: local dev stack (Postgres + backend + frontend)

## Fast Commands (Use These)

- Full pre-push/PR gate: `make ci`
- Dev stack: `make dev-up` / `make dev-down` / `make dev-logs`
- Backend checks: `make backend-ci` (fmt + vet + golangci-lint + tests)
- Frontend checks: `make frontend-ci` (prettier check + eslint + build/typecheck)

## Docker Compose Development

### Start/Stop

- Configure ports/creds (optional): `cp .env.example .env` then edit as needed.
- Start stack (build + run): `make dev-up`
- Stop stack + **delete volumes** (resets Postgres data and wipes the named volumes): `make dev-down`
  - If you want to stop without deleting volumes, use `docker compose down` instead.
- Status: `docker compose ps`

### URLs and APIs

- Frontend: `http://localhost:5173/`
- Backend (direct): `http://localhost:8080/`
- Health checks:
  - Backend: `curl -fsS http://localhost:8080/healthz`
  - API: `curl -fsS http://localhost:8080/api/v1/healthz`
  - Via Vite proxy (recommended when testing frontend integration): `curl -fsS http://localhost:5173/api/v1/healthz`

### Logs and Debugging

- Tail all logs: `make dev-logs`
- Tail a single service: `docker compose logs -f --tail=200 backend` (or `frontend`, `db`)
- Shell into containers (for debugging only; prefer running linters/tests on the host via `make ci`):
  - `docker compose exec backend sh`
  - `docker compose exec frontend sh`

### Database

- PSQL shell: `make dev-psql`
- Reset local DB (destructive): `make dev-down && make dev-up`

## Linting / Type Checking / Formatting (Strict)

### Backend (Go)

- Format: `make backend-fmt` (runs `go fmt` + `goimports -w`)
- Lint: `make backend-lint` (uses `backend/.golangci.yml`; tool is installed to `backend/bin/`)
- Tests: `make backend-test`
- When changing deps: `make backend-tidy` (then re-run `make backend-ci`)

Guidelines:
- Do not hand-format Go; always run `make backend-fmt` before considering a change “done”.
- Treat `golangci-lint` findings as failures (fix or explicitly justify and configure in `backend/.golangci.yml`).

### Frontend (TypeScript/React)

- Format (write): `make frontend-format` (Prettier)
- Format (check): `make frontend-format-check`
- Lint: `make frontend-lint` (ESLint, `--max-warnings=0`)
- Typecheck/build: `make frontend-build` (runs `tsc -b` + `vite build`)

Guidelines:
- Prettier is the source of truth for style (see `frontend/prettier.config.mjs`); do not fight the formatter.
- TypeScript compiler options are strict (see `frontend/tsconfig.app.json` and `frontend/tsconfig.node.json`); fix type errors instead of weakening types.

### Lint Suppressions (Last Resort)

- Go: use `//nolint:<linter> // reason` on the smallest possible scope (prefer a single line over file/package disables).
- TS/TSX: use `// eslint-disable-next-line <rule> -- reason` on the smallest possible scope.
- If a suppression is needed repeatedly, prefer adjusting the rule/config once (and track it in bd as a deliberate decision).

## Generated / Local-Only Files

- Never commit or edit generated artifacts: `frontend/dist/`, `frontend/node_modules/`, `backend/bin/`, `backend/.air/`.
- bd database: never commit `.beads/beads.db`; do commit `.beads/issues.jsonl` alongside relevant code changes.

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**
```bash
bd ready --json
```

**Create new issues:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
bd create "Subtask" --parent <epic-id> --json  # Hierarchical subtask (gets ID like epic-id.1)
```

**Claim and update:**
```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### Auto-Sync

bd automatically syncs with git:
- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### GitHub Copilot Integration

If using GitHub Copilot, also create `.github/copilot-instructions.md` for automatic instruction loading.
Run `bd onboard` to get the content, or see step 2 of the onboard instructions.

### MCP Server (Recommended)

If using Claude or MCP-compatible clients, install the beads MCP server:

```bash
pip install beads-mcp
```

Add to MCP config (e.g., `~/.config/claude/config.json`):
```json
{
  "beads": {
    "command": "beads-mcp",
    "args": []
  }
}
```

Then use `mcp__beads__*` functions instead of CLI commands.

### Managing AI-Generated Planning Documents

AI assistants often create planning and design documents during development:
- PLAN.md, IMPLEMENTATION.md, ARCHITECTURE.md
- DESIGN.md, CODEBASE_SUMMARY.md, INTEGRATION_PLAN.md
- TESTING_GUIDE.md, TECHNICAL_DESIGN.md, and similar files

**Best Practice: Use a dedicated directory for these ephemeral files**

**Recommended approach:**
- Create a `history/` directory in the project root
- Store ALL AI-generated planning/design docs in `history/`
- Keep the repository root clean and focused on permanent project files
- Only access `history/` when explicitly asked to review past planning

**Example .gitignore entry (optional):**
```
# AI planning documents (ephemeral)
history/
```

**Benefits:**
- ✅ Clean repository root
- ✅ Clear separation between ephemeral and permanent documentation
- ✅ Easy to exclude from version control if desired
- ✅ Preserves planning history for archeological research
- ✅ Reduces noise when browsing the project

### CLI Help

Run `bd <command> --help` to see all available flags for any command.
For example: `bd create --help` shows `--parent`, `--deps`, `--assignee`, etc.

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ✅ Store AI planning docs in `history/` directory
- ✅ Run `bd <cmd> --help` to discover available flags
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems
- ❌ Do NOT clutter repo root with planning documents

For more details, see README.md.

## Extra Tools
- Use npx -y @steipete/oracle -p "Walk through the UI smoke test" --file "src/**/*.ts" to consult the oracle when you need a second opinion from a very senior engineer. 
- Use the oracle whenever you get stuck. Provide it with a prompt and a file with the relevant context. Please make sure to explain your problem clearly and provide all the context needed to help solve it. 
