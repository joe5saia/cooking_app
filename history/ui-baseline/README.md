# UI baseline screenshots

This folder captures the pre-token / pre-global-CSS-change baseline screenshots referenced in `docs/site_design.md` Phase 0.

## Prereqs

- Local stack running: `make dev-up`
- A valid session for an existing user:
  - Provide `COOKING_APP_E2E_STORAGE_STATE` pointing at a Playwright storage state JSON, **or**
  - Provide `COOKING_APP_E2E_USERNAME` and `COOKING_APP_E2E_PASSWORD` so the script can log in and create `storageState.json`.

## One-time setup (Playwright browser)

From `frontend/`:

```bash
npx playwright install chromium
```

## Capture baseline screenshots

From `frontend/`:

```bash
COOKING_APP_BASE_URL=http://127.0.0.1:5173 \
COOKING_APP_E2E_USERNAME="..." \
COOKING_APP_E2E_PASSWORD="..." \
npm run ui:baseline
```

Outputs (overwritten on each run):

- `history/ui-baseline/login-desktop.png`
- `history/ui-baseline/login-mobile.png`
- `history/ui-baseline/recipes-desktop.png`
- `history/ui-baseline/recipes-mobile.png`
- `history/ui-baseline/storageState.json` (local-only auth helper; do not add real secrets here)
