# cooking_app

Monorepo containing a Go backend API and a React + TypeScript + Vite frontend, both with linting and formatting wired up.

## Layout
- `backend/` — Go module (`github.com/saiaj/cooking_app/backend`) with API server + admin CLI, linting via `golangci-lint` + `goimports`.
- `frontend/` — Vite React+TS app with ESLint + Prettier.
- `go.work` — workspace tying the backend module to the repo root.

## Prerequisites
- Go 1.25.5+
- Node.js 18+ and npm

## Top-level workflows
- `make ci` — backend fmt/vet/lint/test + frontend format check/lint/build.
- Use `make backend-*` or `make frontend-*` targets for focused tasks (see below).

## Local development (Docker Compose)
- Create a local dev env file: `cp .env.example .env.dev` (optional; defaults are fine).
  - `.env.dev` is intentionally local-only and ignored by git.
  - `make dev-up`/`make dev-down` use `.env.dev` by default; override with `DEV_ENV_FILE=path`.
  - Playwright loads `.env.dev` for `COOKING_APP_E2E_USERNAME`/`COOKING_APP_E2E_PASSWORD`.
- Start stack: `make dev-up`
- URLs:
  - Frontend: `http://localhost:5173`
  - Backend health: `http://localhost:8080/healthz`
  - API health: `http://localhost:5173/api/v1/healthz` (proxied by Vite)
- Stop stack: `make dev-down`

## Backend (from repo root or `backend/`)
- Run API: `make backend-run-api` (requires `DATABASE_URL`).
- Run CLI: `go run ./cmd/cli bootstrap-user --username alice --password '...'` (from `backend/`; requires `DATABASE_URL`).
- Format: `make backend-fmt`
- Lint: `make backend-lint` (installs `golangci-lint`/`goimports` into `backend/bin/`)
- Generate sqlc code: `make -C backend sqlc-generate`
- Tests: `make backend-test`
- Full check: `make backend-ci`

## Frontend (from repo root or `frontend/`)
- Install deps: `npm install --prefix frontend` (`make frontend-install` uses `npm ci` when `CI` is set)
- Dev server: `npm run dev --prefix frontend`
- Format: `npm run format --prefix frontend`
- Lint: `npm run lint --prefix frontend`
- Build: `npm run build --prefix frontend`
- Full check: `make frontend-ci`
