# UI primitives

These are token-driven UI primitives intended to keep styling consistent across pages.

## Conventions

- Prefer primitives for **controls** and **surfaces**:
  - `Button` / `ButtonLink` for actions
  - `Input` / `Select` for form controls
  - `Card` for glass surfaces
  - `FormField` for label + help/error copy and `aria-describedby` wiring
- Prefer page CSS modules for **layout only** (grid/flex, spacing), not per-page re-styling of controls.
- Avoid `style={{ ... }}` for one-off styling; add tokens, primitives, or small page-level CSS module rules instead.

## Accessibility

- Global `:focus-visible` styles live in `frontend/src/index.css`.
- `FormField` provides `id` + `describedBy` so controls can set `aria-describedby` and `aria-invalid`.

## Example

```tsx
<FormField label="Username" error={error}>
  {({ id, describedBy, invalid }) => (
    <Input
      id={id}
      aria-describedby={describedBy}
      invalid={invalid}
      autoComplete="username"
    />
  )}
</FormField>
```
