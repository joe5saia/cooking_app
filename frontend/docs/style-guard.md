# Style guardrails (tokens + no inline styles)

The frontend enforces a few conventions to prevent UI drift:

- No JSX inline style objects (`style={{ ... }}`)
- No raw hex colors in TS/TSX (use tokens instead)
- No raw hex colors in `*.module.css` (use `var(--color-...)` tokens)

## Run locally

From `frontend/`:

```bash
npm run lint:style
```

This runs:

- `ast-grep` rules (see `frontend/ast-grep/rules/`)
- A CSS module scan for `#[0-9a-fA-F]{3,8}`

## How to fix violations

- Inline style: move styles into a CSS module or a shared primitive/pattern component.
- Hex colors: add or reuse a token in `frontend/src/ui/tokens.css`, then reference it via `var(--color-...)`.
