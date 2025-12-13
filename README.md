# cooking_app

Monorepo containing a Go backend API and a React + TypeScript + Vite frontend, both with linting and formatting wired up.

## Layout
- `backend/` — Go module (`github.com/saiaj/cooking_app/backend`) with CLI entry point and linting via `golangci-lint` + `goimports`.
- `frontend/` — Vite React+TS app with ESLint + Prettier.
- `go.work` — workspace tying the backend module to the repo root.

## Prerequisites
- Go 1.25.5+
- Node.js 18+ and npm

## Top-level workflows
- `make ci` — backend fmt/vet/lint/test + frontend format check/lint/build.
- Use `make backend-*` or `make frontend-*` targets for focused tasks (see below).

## Backend (from repo root or `backend/`)
- Run: `go run ./cmd/cooking_app` (from `backend/`).
- Format: `make backend-fmt`
- Lint: `make backend-lint` (installs `golangci-lint`/`goimports` into `backend/bin/`)
- Tests: `make backend-test`
- Full check: `make backend-ci`

## Frontend (from repo root or `frontend/`)
- Install deps: `npm install --prefix frontend` (`make frontend-install` uses `npm ci` when `CI` is set)
- Dev server: `npm run dev --prefix frontend`
- Format: `npm run format --prefix frontend`
- Lint: `npm run lint --prefix frontend`
- Build: `npm run build --prefix frontend`
- Full check: `make frontend-ci`
