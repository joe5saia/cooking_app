# UI conventions (Midnight Glass)

This repo uses a lightweight, token-driven UI system to keep the frontend consistent and avoid per-page style drift.

## Tokens

- Tokens live in `frontend/src/ui/tokens.css`.
- CSS modules should reference tokens via `var(--...)` (avoid raw hex colors in `*.module.css`).

Common tokens:

- Colors: `--color-bg`, `--color-surface-1`, `--color-border`, `--color-text`, `--color-muted`, `--color-accent`, `--color-danger`
- Spacing: `--space-1` … `--space-6`
- Radii: `--radius-sm`, `--radius-md`, `--radius-lg`
- Typography: `--font-sans`, `--text-sm`, `--text-md`, `--text-lg`

## Primitives vs page styles

Primitives live in `frontend/src/ui/components/`:

- `Button` / `ButtonLink` (variants + sizes)
- `Input` / `Select`
- `Card`
- `FormField`

Guideline:

- Use primitives for controls/surfaces and keep page CSS modules focused on layout (grid/flex, spacing).
- Prefer updating/adding primitives or small shared patterns over copy/pasting one-off styles per page.

See also: `frontend/src/ui/components/README.md`.

## Accessibility expectations

- Global `:focus-visible` styles are defined in `frontend/src/index.css`.
- `FormField` provides `id` + `describedBy` so controls can set `aria-describedby` and `aria-invalid`.
- Prefer `role="alert"` for user-visible errors and ensure error copy is discoverable by screen readers.
- Respect `prefers-reduced-motion` for non-essential transitions (e.g., header nav pills).

## Common patterns

### Forms

Use `FormField` to wire label/help/error text and `Input` for styling:

```tsx
<FormField label="Username" error={usernameError ?? undefined} required>
  {({ id, describedBy, invalid }) => (
    <Input
      id={id}
      autoComplete="username"
      aria-describedby={describedBy}
      invalid={invalid}
      value={username}
      onChange={(e) => setUsername(e.target.value)}
    />
  )}
</FormField>
```

### CRUD lists

- Wrap pages in `Page` (shared title + card chrome)
- Use `ButtonLink` for “create” actions, `Button` for mutations
- Use `Card` for list rows when you need per-row surfaces

Reference implementations:

- `frontend/src/ui/pages/RecipeListPage.tsx`
- `frontend/src/ui/pages/BookListPage.tsx`
- `frontend/src/ui/pages/TagListPage.tsx`

## Guardrails

`make frontend-ci` enforces:

- no JSX inline style objects (`style={{...}}`)
- no raw hex colors in TS/TSX
- no raw hex colors in `*.module.css`

Details: `frontend/docs/style-guard.md`.
