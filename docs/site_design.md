# RFC: Site Design Review & Design Language

Status: Draft  
Last updated: 2025-12-15  
Scope: UI/UX and styling for the web app (primarily `frontend/`)

## Summary

The current Cooking App UI is a small authenticated SPA with a consistent “dark glass” look: a deep navy shell (`#0b1220`), translucent card surfaces, and pill-style navigation. Layout is centered with a max-width content column on desktop and a stacked header + 4-wide tab grid on mobile.

This RFC documents the current layout (as observed via Playwright navigation and a quick code review) and proposes a lightweight design language (tokens + a few reusable components) along with a migration plan that reduces style drift and removes leftover Vite starter CSS that currently fights the app’s intended look.

## Goals / Non-goals

Goals:

- Summarize the app’s current information architecture (routes/pages) and layout primitives.
- Propose a cohesive design language that matches the app’s existing direction (dark, modern, glassy).
- Provide a pragmatic migration plan that can be executed incrementally without a redesign rewrite.
- Establish a Playwright-based “visual snapshot” workflow for layout review.

Non-goals:

- Pixel-perfect final designs for every screen.
- Introducing a heavy UI framework (MUI/Chakra/etc.) as a requirement.
- Reworking product flows or adding new features beyond what is necessary to improve layout consistency.

## Current App Overview (Observed)

Authentication:

- Unauthenticated users land on `/login`.
- Successful login redirects to `/recipes` (or the originally requested route).
- `/login` is routed outside the authenticated app shell (`AppShell`).

Primary routes (top navigation):

- `/recipes` (list + filters; create entrypoint)
- `/books` (recipe books list; create)
- `/tags` (tags list; create)
- `/settings` (links to token/user management)
  - `/settings/tokens`
  - `/settings/users`

## Current Layout & Visual Structure

### App shell (global)

Implemented in `frontend/src/ui/AppShell.tsx` with styles in `frontend/src/ui/AppShell.module.css`.

- Sticky header with:
  - Brand (“Cooking App”)
  - Primary nav (4 tabs/pills)
  - Current user label (“Signed in as …”)
- Main content area:
  - Padding of `16px` (mobile) / `24px` (≥ 640px)
  - Desktop max width `980px` centered

### Page wrapper pattern

Implemented in `frontend/src/ui/pages/Page.tsx` + `frontend/src/ui/pages/Page.module.css`.

- Page = title row + “body card”
- Card treatment:
  - `border-radius: 14px`
  - translucent background `rgba(231, 234, 240, 0.04)`
  - subtle border `rgba(231, 234, 240, 0.08)`

### CRUD list patterns

Many list pages use `frontend/src/ui/pages/CrudList.module.css`:

- Inputs and buttons share a cohesive glass treatment (rounded, translucent, subtle borders).
- Explicit “danger” styling exists for destructive actions.
- Layout uses grid for filters/forms and flex for action rows.

### Notable inconsistencies / design risks

These issues are visible in screenshots and reflected in styles:

- Global starter CSS is still present (`frontend/src/index.css`):
  - `body` uses `display: flex; place-items: center; min-height: 100vh;` which centers the entire app like a demo page.
  - Default `h1` sizing and `button` styles conflict with the app’s CSS modules.
  - Global `a:hover` styling can leak into navigation (NavLink renders as `<a>`).
  - Light color-scheme can yield a light outer canvas around a dark centered app shell, which reads like the app is “floating in a white page”.
- Login page uses inline styles (`frontend/src/ui/pages/LoginPage.tsx`) rather than the `Page` + CSS-module patterns.
- Components are visually consistent within pages that use `CrudList.module.css`, but global HTML element styles can still leak in (buttons, headings, body centering).

### Wayfinding / navigation behavior

- Desktop header background is full-bleed, but the content column is constrained; without a shared “inner container” the header and main content edges won’t align visually.
- Primary nav uses active-state pills; however, “Recipes” is configured with `NavLink end`, which means nested routes like `/recipes/new` or `/recipes/:id` won’t show the Recipes tab as active. This weakens wayfinding on detail/editor pages.

### Responsive behavior (current + risks)

- Mobile header stacks brand → nav → user label; the nav is a 4-column grid, which works now but is brittle if more items are added.
- The app should treat the shell background as full-bleed; avoid body-level centering and prefer `min-height: 100dvh` so the UI behaves correctly on mobile address-bar resizing.
- Consider safe-area padding on iOS (`env(safe-area-inset-*)`) for the sticky header.

## Proposed Design Language: “Midnight Glass”

The current UI already points toward a coherent direction: a deep, calm background with softly elevated glass surfaces. The proposal is to formalize this into tokens and a small component set so the app remains consistent as features grow.

### Principles

- Calm, content-first UI: avoid heavy borders and high-contrast noise.
- One primary accent color for interactive focus and selected states.
- Soft elevation: use translucent surfaces and subtle borders rather than drop-shadow-heavy cards.
- Strong accessibility by default: clear focus rings, sufficient contrast, predictable spacing.

### Design tokens (CSS variables)

Define a single source of truth for colors/spacing/typography, consumed by CSS modules:

- Color
  - `--color-bg`: deep navy background
  - `--color-surface-1`: primary glass surface
  - `--color-surface-2`: hover/raised surface
  - `--color-border`: subtle border
  - `--color-text`: primary text
  - `--color-text-muted`: secondary text
  - `--color-accent`: action/selection (current `#a7c7ff`)
  - `--color-danger`: destructive
  - `--color-focus`: focus ring (usually derived from accent)
- Radius
  - `--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-pill`
- Spacing
  - `--space-1..--space-6` (4px-based scale)
- Typography
  - `--font-sans` (system stack is fine)
  - `--text-sm`, `--text-md`, `--text-lg`, `--text-xl`
  - `--leading-normal`, `--leading-tight`

### Core components (minimum viable “design system”)

Create or standardize a small set of primitives that match existing patterns:

- `Button`: variants `primary`, `secondary`, `ghost`, `danger`, sizes `sm/md`
- `Input` + `Select`: consistent padding, border, focus ring, disabled state
- `Card`: the glass surface used by `Page` body and list containers
- `FormField`: label + control + help/error text (keeps spacing consistent)
- `TopNav` (or keep existing `NavLink` rendering but standardize tokens)

This can remain “just React components + CSS modules” to avoid framework lock-in.

### Component boundaries (to prevent style drift)

Define and enforce boundaries so patterns don’t fork per-page over time:

- Primitives: low-level, reusable, style-token-driven building blocks (`Button`, `Input`, `Card`, `FormField`).
- Patterns: small compositions for common UI shapes (filters row, CRUD list item, empty states, confirmation dialogs).
- Pages: orchestration, data fetching, and layout composition; avoid page-specific one-off styles when a primitive/pattern can be reused.

### Accessibility requirements (definition of “strong by default”)

Minimum acceptance criteria for the design language and primitives:

- Focus visibility: all interactive elements have a clear `:focus-visible` indicator (including nav pills).
- Keyboard: tab order is logical; primary navigation is reachable; add a “Skip to content” link in the app shell.
- Forms: field-level errors use `role="alert"` or `aria-live`, plus `aria-invalid` and `aria-describedby` for controls.
- Contrast: text, borders, and focus rings meet contrast expectations on dark surfaces.
- Motion: respect `prefers-reduced-motion` (disable non-essential transitions/animations).

### Layout guidance

- Full-bleed shell background should cover the full viewport; avoid body-level centering.
- Content width:
  - Keep `max-width: 980px` for primary pages
  - Consider widening to `1100–1200px` for data-heavy pages (recipes detail/editor) if needed
- Header:
  - Mobile: brand + tab bar is good; consider moving “Signed in as …” into a menu (or smaller, single-line treatment) to reduce header height.

## Migration Plan (Incremental)

Phase 0: Baseline + guardrails

- Capture baseline screenshots (desktop + mobile) before making global CSS changes.
- Decide and document route boundaries (auth routes stay outside the app shell; keep shell chrome out of `/login`).
- Add a convention check: new UI styles should use tokens rather than raw hex values (enforced by review or tooling).

Phase 1a: Introduce tokens + minimal reset

- Add a theme/tokens stylesheet (CSS variables) imported from the app entrypoint.
- Replace Vite demo defaults in `frontend/src/index.css`:
  - Remove `body` flex-centering and demo `h1` sizing.
  - Set `body` background to `--color-bg` and text to `--color-text`.
  - Remove global `a:hover` coloring that can leak into app navigation.
  - Avoid `prefers-color-scheme: light` overrides for the app shell (keep a single theme by default).

Phase 1b: Tokenize the shell and page chrome

- Update `AppShell.module.css` and `Page.module.css` to use `var(--...)` tokens.
- Align header and content column by introducing a shared “inner container” for header content (visual polish that benefits every page).
- Add hover/focus-visible styles for nav pills.

Phase 2: Normalize auth layout

- Convert `LoginPage` away from inline styles to a CSS module and reuse the same form primitives as the rest of the app.
- Ensure the login screen visually matches the “Midnight Glass” direction (same background, same card treatment).

Phase 3: Introduce primitives and migrate high-churn pages

- Add `Button`, `Input`, `Card`, `FormField` primitives.
- Migrate list pages (`/recipes`, `/books`, `/tags`) first because they share the most patterns and will benefit most from consistency.

Phase 4: Visual regression workflow with Playwright

- Prefer a deterministic Playwright snapshot suite (rather than ad-hoc screenshots):
  - Fixed viewports (e.g., `390x844` and `1280x720`), `prefers-reduced-motion`, and stable waits.
  - Login via environment-provided credentials (never hard-coded), or use a `storageState` captured once per run.
  - Mask dynamic areas (e.g., “Signed in as …”) to avoid snapshot churn.
  - Capture `/login`, `/recipes`, `/books`, `/tags`, `/settings` at both viewports.

## Playwright notes (how this RFC was validated)

The layout/route observations were confirmed by automating login and navigation with Playwright and capturing full-page screenshots at desktop and mobile viewports.

Recommended commands for manual exploration:

- Interactive exploration + DevTools: `npx -y playwright open -b chromium --devtools http://HOST/`
- Generate selectors and scripts: `npx -y playwright codegen -b chromium http://HOST/`
- Full-page snapshots: `npx -y playwright screenshot --full-page http://HOST/path /tmp/page.png`
