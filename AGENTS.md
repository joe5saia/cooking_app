# Code Style

Be OCD about constient code style. You reconigize that code will be read houndreds of times, but written only once. Always perform a second pass on your code after you have editted it to make sure that is well organized and clear. Always write idiomatic code for the language you using.

When writting Go code, reference https://go.dev/doc/effective_go when reviewing code style.

- Always document your code
- Always write unit tests for your code
- Use AST Grep and suggest rules that can be applied to a project when you see an opportunity to do so
- Always run linters and formatters frequently
- Follow SOLID principles
- For typed langagues, use the strictest type checking possible. Avoid Any types or lazy uses of Nulls

# General Guidelines

- Delete unused or obsolete files when your changes make them irrelevant (refactors, feature removals, etc.), and revert files only when the change is yours or explicitly requested. If a git operation leaves you unsure about other agents' in-flight work, stop and coordinate instead of deleting.
- **Before attempting to delete a file to resolve a local type/lint failure, stop and ask the user.** Other agents are often editing adjacent files; deleting their work to silence an error is never acceptable without explicit approval.
- NEVER edit `.env` or any environment variable files--only the user may change them.
- Coordinate with other agents before removing their in-progress edits--don't revert or delete work you didn't author unless everyone agrees.
- Moving/renaming and restoring files is allowed.
- ABSOLUTELY NEVER run destructive git operations (e.g., `git reset --hard`, `rm`, `git checkout`/`git restore` to an older commit) unless the user gives an explicit, written instruction in this conversation. Treat these commands as catastrophic; if you are even slightly unsure, stop and ask before touching them. *(When working within Cursor or Codex Web, these git limitations do not apply; use the tooling's capabilities as needed.)*
- Never use `git restore` (or similar commands) to revert files you didn't author--coordinate with other agents instead so their in-progress work stays intact.
- Always double-check git status before any commit
- Keep commits atomic: commit only the files you touched and list each path explicitly. For tracked files run `git commit -m "<scoped message>" -- path/to/file1 path/to/file2`. For brand-new files, use the one-liner `git restore --staged :/ && git add "path/to/file1" "path/to/file2" && git commit -m "<scoped message>" -- path/to/file1 path/to/file2`.
- Quote any git paths containing brackets or parentheses (e.g., `src/app/[candidate]/**`) when staging or committing so the shell does not treat them as globs or subshells.
- When running `git rebase`, avoid opening editors--export `GIT_EDITOR=:` and `GIT_SEQUENCE_EDITOR=:` (or pass `--no-edit`) so the default messages are used automatically.
- Never amend commits unless you have explicit written approval in the task thread.
- Use `npx -y @steipete/oracle -p "Walk through the UI smoke test" --file "src/**/*.ts"` to consult the oracle when you need a second opinion from a very senior engineer.
- Use the oracle whenever you get stuck. Provide it with a prompt and a file with the relevant context. Please make sure to explain your problem clearly and provide all the context needed to help solve it.

# Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

## Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

## Quick Start

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

## Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

## Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

## Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

## Using bd Effectively

### Daily flow (recommended)

1. Find work: `bd ready --json` (or `bd list --status open --sort priority`)
2. De-dupe: `bd search "keywords" --json` before creating anything new
3. Claim: `bd update <id> --status in_progress --json` (set assignee if you use them)
4. Keep it accurate: update priority/status as reality changes (and add comments when helpful)
5. Finish: `bd close <id> --reason "<what changed>" --json`

### Writing high-signal issues

- Title: start with a verb and include scope when helpful (e.g., `Backend: ...`, `Frontend: ...`, `Deploy: ...`).
- Description: include the "why", links to files/symbols, and any constraints/assumptions.
- Acceptance criteria: make it objectively verifiable (commands/tests, observable behavior, specific outputs).

### Dependencies: when to use what

- `parent-child`: epic -> child/subtask hierarchy (use `--parent <epic-id>` when creating).
- `blocks`: hard ordering (issue B cannot start/finish until A is done).
- `discovered-from`: track newly found work back to the issue/context that surfaced it.
- `related`: useful context, but not a blocker.

Examples:
```bash
bd create "Epic title" --type epic -p 1 --json
bd create "Child task" --parent <epic-id> -p 2 --json
bd dep add <issue-id> <depends-on-id> -t blocks --json
bd dep add <new-issue-id> <source-issue-id> -t discovered-from --json
```

### Navigating and reviewing work

```bash
bd show <id> --json
bd dep tree <id>
bd dep tree <id> --direction=up
bd epic status --json
```

### Troubleshooting / hygiene

- If `bd` prints sync/hash warnings, run `bd doctor` (health) or `bd validate` (integrity) and re-run the command.
- If `.beads/issues.jsonl` conflicts during merges, resolve the conflict and then run `bd clean` to remove temporary merge artifacts.
- Prefer `--no-daemon` in scripts/CI to avoid daemon-related flakiness.

## CLI Help

Run `bd <command> --help` to see all available flags for any command.
For example: `bd create --help` shows `--parent`, `--deps`, `--assignee`, etc.

## Important Rules

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

# Project Overview

Monorepo:
- `backend/`: Go API + CLI (Go `1.25.5` via `go.work`)
- `frontend/`: React + TypeScript + Vite
- `compose.yaml`: local dev stack (Postgres + backend + frontend)

# Fast Commands (Use These)

- Full pre-push/PR gate: `make ci`
- Dev stack: `make dev-up` / `make dev-down` / `make dev-logs`
- Backend checks: `make backend-ci` (fmt + vet + golangci-lint + tests)
- Frontend checks: `make frontend-ci` (prettier check + eslint + build/typecheck)

# Docker Compose Development

## Start/Stop

- Configure ports/creds (optional): `cp .env.example .env` then edit as needed.
- Start stack (build + run): `make dev-up`
- Stop stack + **delete volumes** (resets Postgres data and wipes the named volumes): `make dev-down`
  - If you want to stop without deleting volumes, use `docker compose down` instead.
- Status: `docker compose ps`

## URLs and APIs

- Frontend: `http://localhost:5173/`
- Backend (direct): `http://localhost:8080/`
- Health checks:
  - Backend: `curl -fsS http://localhost:8080/healthz`
  - API: `curl -fsS http://localhost:8080/api/v1/healthz`
  - Via Vite proxy (recommended when testing frontend integration): `curl -fsS http://localhost:5173/api/v1/healthz`

## Logs and Debugging

- Tail all logs: `make dev-logs`
- Tail a single service: `docker compose logs -f --tail=200 backend` (or `frontend`, `db`)
- Shell into containers (for debugging only; prefer running linters/tests on the host via `make ci`):
  - `docker compose exec backend sh`
  - `docker compose exec frontend sh`

## Database

- PSQL shell: `make dev-psql`
- Reset local DB (destructive): `make dev-down && make dev-up`

# Linting / Type Checking / Formatting (Strict)

## Backend (Go)

- Format: `make backend-fmt` (runs `go fmt` + `goimports -w`)
- Lint: `make backend-lint` (uses `backend/.golangci.yml`; tool is installed to `backend/bin/`)
- Tests: `make backend-test`
- When changing deps: `make backend-tidy` (then re-run `make backend-ci`)

Guidelines:
- Do not hand-format Go; always run `make backend-fmt` before considering a change "done".
- Treat `golangci-lint` findings as failures (fix or explicitly justify and configure in `backend/.golangci.yml`).

## Frontend (TypeScript/React)

- Format (write): `make frontend-format` (Prettier)
- Format (check): `make frontend-format-check`
- Lint: `make frontend-lint` (ESLint, `--max-warnings=0`)
- Typecheck/build: `make frontend-build` (runs `tsc -b` + `vite build`)

Guidelines:
- Prettier is the source of truth for style (see `frontend/prettier.config.mjs`); do not fight the formatter.
- TypeScript compiler options are strict (see `frontend/tsconfig.app.json` and `frontend/tsconfig.node.json`); fix type errors instead of weakening types.

## Lint Suppressions (Last Resort)

- Go: use `//nolint:<linter> // reason` on the smallest possible scope (prefer a single line over file/package disables).
- TS/TSX: use `// eslint-disable-next-line <rule> -- reason` on the smallest possible scope.
- If a suppression is needed repeatedly, prefer adjusting the rule/config once (and track it in bd as a deliberate decision).

# Generated / Local-Only Files

- Never commit or edit generated artifacts: `frontend/dist/`, `frontend/node_modules/`, `backend/bin/`, `backend/.air/`.
- bd database: never commit `.beads/beads.db`; do commit `.beads/issues.jsonl` alongside relevant code changes.

# Playwright (UI Exploration)

Use `npx playwright` for quick, ad-hoc navigation and screenshots against a running environment.

## One-time setup

- Install a browser (first time on a machine): `npx -y playwright install chromium`

## Quick screenshots

- Desktop full page: `npx -y playwright screenshot --full-page http://HOST/ /tmp/page.png`
- Mobile-ish viewport: `npx -y playwright screenshot --viewport-size=390,844 --full-page http://HOST/ /tmp/page-mobile.png`

## Interactive exploration (best for learning layout/selectors)

- Open with DevTools: `npx -y playwright open -b chromium --devtools http://HOST/`
- Record clicks and generate code: `npx -y playwright codegen -b chromium http://HOST/`

## Current app navigation notes (observed)

- Landing route redirects to `/login` when not authenticated.
- Login form uses labeled fields `Username` and `Password` and a `Sign in` button (good for `page.getByLabel(...)` / `page.getByRole(...)` selectors).
- Successful login redirects to `/recipes`; top navigation includes `/recipes`, `/books`, `/tags`, `/settings`.

# Extra Tools

- Use `npx -y @steipete/oracle -p "Walk through the UI smoke test" --file "src/**/*.ts"` to consult the oracle when you need a second opinion from a very senior engineer.
- Use the oracle whenever you get stuck. Provide it with a prompt and a file with the relevant context. Please make sure to explain your problem clearly and provide all the context needed to help solve it.

# Skills

These skills are discovered at startup from multiple local sources. Each entry includes a name, description, and file path so you can open the source for full instructions.

- skill-creator: Guide for creating effective skills. This skill should be used when users want to create a new skill (or update an existing skill) that extends Codex's capabilities with specialized knowledge, workflows, or tool integrations. (file: /Users/saiaj/.codex/skills/.system/skill-creator/SKILL.md)
- skill-installer: Install Codex skills into $CODEX_HOME/skills from a curated list or a GitHub repo path. Use when a user asks to list installable skills, install a curated skill, or install a skill from another repo (including private repos). (file: /Users/saiaj/.codex/skills/.system/skill-installer/SKILL.md)
- Discovery: Available skills are listed in project docs and may also appear in a runtime "## Skills" section (name + description + file path). These are the sources of truth; skill bodies live on disk at the listed paths.
- Trigger rules: If the user names a skill (with `$SkillName` or plain text) OR the task clearly matches a skill's description, you must use that skill for that turn. Multiple mentions mean use them all. Do not carry skills across turns unless re-mentioned.
- Missing/blocked: If a named skill isn't in the list or the path can't be read, say so briefly and continue with the best fallback.
- How to use a skill (progressive disclosure):
  1) After deciding to use a skill, open its `SKILL.md`. Read only enough to follow the workflow.
  2) If `SKILL.md` points to extra folders such as `references/`, load only the specific files needed for the request; don't bulk-load everything.
  3) If `scripts/` exist, prefer running or patching them instead of retyping large code blocks.
  4) If `assets/` or templates exist, reuse them instead of recreating from scratch.
- Description as trigger: The YAML `description` in `SKILL.md` is the primary trigger signal; rely on it to decide applicability. If unsure, ask a brief clarification before proceeding.
- Coordination and sequencing:
  - If multiple skills apply, choose the minimal set that covers the request and state the order you'll use them.
  - Announce which skill(s) you're using and why (one short line). If you skip an obvious skill, say why.
- Context hygiene:
  - Keep context small: summarize long sections instead of pasting them; only load extra files when needed.
  - Avoid deeply nested references; prefer one-hop files explicitly linked from `SKILL.md`.
  - When variants exist (frameworks, providers, domains), pick only the relevant reference file(s) and note that choice.
- Safety and fallback: If a skill can't be applied cleanly (missing files, unclear instructions), state the issue, pick the next-best approach, and continue.
