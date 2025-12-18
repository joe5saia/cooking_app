# Visual snapshot suite (Playwright)

This suite captures deterministic-ish UI screenshots to catch layout/style drift.

## Prereqs

- A running stack (`make dev-up`) reachable via `COOKING_APP_BASE_URL` (recommended: `http://127.0.0.1:5173`)
- A valid user for login (provided via env vars; never hard-code credentials)

One-time browser install (from `frontend/`):

```bash
npx playwright install chromium
```

## Run

From `frontend/`:

```bash
COOKING_APP_BASE_URL=http://127.0.0.1:5173 \
COOKING_APP_E2E_USERNAME="..." \
COOKING_APP_E2E_PASSWORD="..." \
npm run e2e
```

## Update snapshots

```bash
COOKING_APP_BASE_URL=http://127.0.0.1:5173 \
COOKING_APP_E2E_USERNAME="..." \
COOKING_APP_E2E_PASSWORD="..." \
npm run e2e:update
```

Notes:

- Auth storage state is generated into `frontend/playwright/.auth/` (gitignored).
- Test artifacts (traces/videos) go to `test-results/playwright/` (gitignored).
