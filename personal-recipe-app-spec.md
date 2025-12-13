# Personal Recipe App (Umami-like) — Product & Technical Specification (MVP v1)

**Document owner:** You  
**Audience:** Engineering / AI Agent implementation team  
**Status:** Ready for build handoff  
**Last updated:** 2025-12-13

---

## 1. Executive summary

This document specifies an MVP implementation of a **multi-user, shared recipe management** web application inspired by Umami, optimized for **API access** and future **interoperability**. The system is designed for **self-hosting** on an Ubuntu server and runs via **Docker Compose**, fronted by **Caddy** with **Let’s Encrypt** TLS. The MVP scope focuses on **saving and browsing recipes**, with **recipe books** and **tags** for organization. Meal planning, grocery lists, timers, event streams, exports/importers, and offline support are explicitly out of scope for v1.

Key constraints:
- Backend: **Golang** (REST API)
- Frontend: **React + TypeScript + Vite**, installable **PWA** (no offline support in v1)
- Database: **PostgreSQL 18**
- Multi-user: **>= 3 users**, all “admin” with full permissions; **recipes are fully shared**
- Authentication: **username + password**
- API access: **Personal Access Tokens (PAT) required from day one**
- Auditing on all mutable entities: `created_at`, `created_by`, `updated_at`, `updated_by` (non-null; FK to `users`)
- Soft delete for recipes
- Search: **title only**
- Sorting: **recently updated**

---

## 2. Goals and non-goals

### 2.1 Goals (MVP)
1. Allow authenticated users to **create, edit, view, and soft-delete recipes**.
2. Provide recipe organization via:
   - **Recipe Books** (one recipe belongs to at most one book in v1)
   - **Tags** (many-to-many)
3. Provide:
   - **Recipe listing** (sorted by `updated_at DESC`)
   - **Title search** (`q=` parameter)
4. Provide a **well-defined REST API** suitable for scripting and future integrations.
5. Provide **PAT-based API access** from day one.
6. Self-hostable deployment on Ubuntu with Docker Compose + Caddy + Postgres 18.
7. Basic, structured **JSONL logging**.

### 2.2 Non-goals (explicitly out of scope for v1)
- Meal planning / calendar / scheduling
- Grocery lists
- Timers / cooking mode stepper
- Offline mode / IndexedDB caching
- URL-based recipe import or parsing
- Data export endpoints and bulk endpoints
- Event streaming (SSE/WebSockets) and webhooks
- Fine-grained authorization, roles, per-user recipe ownership (everything shared)

---

## 3. Users and permissions

### 3.1 Users
- The system must support **at least 3 users**.
- All users have equal privileges; treat all as “admin”.

### 3.2 Provisioning model
- **Admin users provision new accounts**.
- There is no public signup.
- The system must support a **bootstrap** path to create the first user.

**Bootstrap requirement:** On a fresh database, there must be a safe method to create the first user, such as:
- A one-time CLI/admin command (`umami-api bootstrap-user ...`)
- Or a guarded “bootstrap” endpoint enabled only when `users` table is empty and protected by a one-time secret in env (`UMAMI_BOOTSTRAP_SECRET`).

Recommendation: implement a CLI for bootstrap to avoid any long-lived web bootstrap endpoint.

---

## 4. Product requirements

### 4.1 Navigation and primary screens (v1)
Mobile-first layout with top bar + simple navigation:
- **Recipes** (default)
- **Recipe Books**
- **Tags**
- **Settings** (profile, PAT management, user management)

### 4.2 Recipe requirements
Each recipe must support:
- Title (required)
- Servings (integer; required)
- Prep time (minutes; integer; required)
- Total time (minutes; integer; required)
- Source URL (optional)
- Notes (optional; markdown allowed or plain text in v1)

Ingredients must be stored as **structured fields** (see data model). Steps are a numbered list 1..N (no subsections).

### 4.3 Recipe books
- CRUD for recipe books
- Recipe belongs to **zero or one** book (v1)
- Future possibility: many-to-many (not in v1)

### 4.4 Tags
- CRUD for tags
- Tags are applied to recipes (many-to-many)

### 4.5 Search and sort
- Search is **title-only**.
- Listing is sorted by `updated_at DESC` (most recently updated first).
- Soft-deleted recipes are excluded by default.

### 4.6 Soft delete
- Deleting a recipe sets `deleted_at` (and updates `updated_at/updated_by`).
- Deleted recipes should not appear in normal listing/search.
- Optional endpoint for admin restore may be implemented (recommended), but can be postponed.

---

## 5. System architecture

### 5.1 High-level components
1. **Web client** (React + TS + Vite)
2. **API service** (Go)
3. **Database** (PostgreSQL 18)
4. **Reverse proxy / TLS** (Caddy)
5. **(Optional)** One-shot migration job or migrations built into API startup.

### 5.2 Proposed request flow
Client (PWA) → Caddy → API (`/api/v1/*`) → Postgres  
Client (static assets) → Caddy file server (or API static server)

Recommendation: Caddy serves static assets directly; API is separate container.

---

## 6. Technology stack (recommended)

### 6.1 Backend (Go)
- HTTP router: `chi` (lightweight, stable) or `echo` (acceptable)
- DB driver: `pgx`
- Query layer: `sqlc` (type-safe queries, explicit SQL)
- Migrations: `goose` or `atlas` (choose one; document and standardize)
- Password hashing: **Argon2id** (preferred) or bcrypt (acceptable)
- Authentication:
  - Browser: cookie-based session (HTTP-only, Secure)
  - API scripts: PATs (Bearer token)
- Structured logging: `slog` (Go standard) to JSONL, or `zap` JSON encoder

### 6.2 Frontend
- React 18+
- TypeScript
- Vite
- Routing: React Router
- API calls: generated client from OpenAPI (preferred) or typed fetch wrapper
- State management:
  - Server state: TanStack Query (recommended)
  - UI state: local state + minimal context
- PWA: `vite-plugin-pwa` for manifest/install (offline disabled for now)

---

## 7. Data model (PostgreSQL 18)

### 7.1 Conventions
- Primary keys: `uuid` (v4)
- Time fields: `timestamptz`
- Required audit fields on all mutable tables:
  - `created_at timestamptz not null`
  - `created_by uuid not null references users(id)`
  - `updated_at timestamptz not null`
  - `updated_by uuid not null references users(id)`
- Soft delete pattern:
  - `deleted_at timestamptz null` (for recipes only in v1)

### 7.2 Tables

#### 7.2.1 `users`
Stores authentication credentials and identity.

Fields:
- `id uuid pk`
- `username text not null unique` (case-insensitive uniqueness recommended)
- `password_hash text not null`
- `display_name text null`
- `is_active boolean not null default true`
- `created_at timestamptz not null`
- `created_by uuid not null references users(id)` *(bootstrap special case; see below)*
- `updated_at timestamptz not null`
- `updated_by uuid not null references users(id)`

Bootstrap note:
- For the first user, `created_by/updated_by` cannot reference an existing user. Options:
  1) Allow `created_by/updated_by` to be nullable on `users` only (not preferred given requirements), OR
  2) Use a special “system user” row created first, OR
  3) Insert the first user with `created_by = id` (self-reference), same for updated_by.

**Chosen approach (recommended):** create the first user with `created_by = <new_user_id>` and `updated_by = <new_user_id>`. This satisfies non-null + FK constraints.

#### 7.2.2 `recipe_books`
- `id uuid pk`
- `name text not null unique` (or unique per deployment)
- Audit fields

Indexes:
- unique index on `name` (consider `citext` extension to make it case-insensitive)

#### 7.2.3 `tags`
- `id uuid pk`
- `name text not null unique`
- Audit fields

Indexes:
- unique index on `name` (case-insensitive recommended)

#### 7.2.4 `recipes`
- `id uuid pk`
- `title text not null`
- `servings int not null check (servings > 0)`
- `prep_time_minutes int not null check (prep_time_minutes >= 0)`
- `total_time_minutes int not null check (total_time_minutes >= 0)`
- `source_url text null` (validate format in app)
- `notes text null`
- `recipe_book_id uuid null references recipe_books(id)`
- `deleted_at timestamptz null`
- Audit fields

Indexes:
- `recipes_updated_at_idx (updated_at desc)`
- `recipes_deleted_at_idx (deleted_at)`
- Title search index:
  - Option A (simple): `index on lower(title)`
  - Option B (better contains search): `pg_trgm` + `gin` index on title
    - `create extension if not exists pg_trgm;`
    - `create index recipes_title_trgm_idx on recipes using gin (title gin_trgm_ops);`

Given “title-only” search, trigram index is a good low-effort upgrade.

#### 7.2.5 `recipe_ingredients`
Stores structured ingredient lines.

- `id uuid pk`
- `recipe_id uuid not null references recipes(id) on delete cascade`
- `position int not null` (0-based or 1-based; decide and standardize)
- `quantity numeric null` *(optional; see below)*
- `quantity_text text null` *(optional)*  
- `unit text null`
- `item text not null` *(ingredient main item; e.g., "onion")*
- `prep text null` *(e.g., "diced")*
- `notes text null` *(e.g., "to taste")*
- `original_text text null` *(optional but recommended for fidelity)*
- Audit fields

**Quantity representation decision (recommended):**
- Store both:
  - `quantity_text` for fidelity (“1 1/2”, “a pinch”)
  - `quantity` numeric for numeric values when parseable
- The app may accept `quantity_text` always and populate `quantity` when numeric.

Indexes:
- `recipe_ingredients_recipe_id_position_idx (recipe_id, position)`

#### 7.2.6 `recipe_steps`
- `id uuid pk`
- `recipe_id uuid not null references recipes(id) on delete cascade`
- `step_number int not null check (step_number >= 1)`
- `instruction text not null`
- Audit fields

Unique constraint:
- `(recipe_id, step_number)` unique

Index:
- `(recipe_id, step_number)`

#### 7.2.7 `recipe_tags`
- `recipe_id uuid not null references recipes(id) on delete cascade`
- `tag_id uuid not null references tags(id) on delete cascade`
- Audit fields

PK / unique:
- primary key `(recipe_id, tag_id)`

Indexes:
- `(tag_id)` to query by tag

#### 7.2.8 `personal_access_tokens`
- `id uuid pk`
- `user_id uuid not null references users(id) on delete cascade`
- `name text not null` (label)
- `token_hash text not null` (hash of token secret)
- `last_used_at timestamptz null`
- `expires_at timestamptz null` *(optional; recommended)*
- Audit fields

Indexes:
- `(user_id)`
- `(expires_at)`

**Security requirement:** token secret is displayed **only once** at creation; only hash stored.

---

## 8. API specification (REST)

### 8.1 API standards
- Base path: `/api/v1`
- Content type: `application/json; charset=utf-8`
- Auth:
  - Browser: cookie session
  - API: `Authorization: Bearer <PAT>`
- Errors: JSON problem-style (simplified)

Example error:
```json
{
  "error": {
    "code": "validation_error",
    "message": "title is required",
    "details": {"field": "title"}
  }
}
```

### 8.2 Authentication endpoints

#### 8.2.1 Login (browser)
`POST /api/v1/auth/login`

Request:
```json
{ "username": "joe", "password": "..." }
```

Response: `204 No Content` and sets session cookie.

#### 8.2.2 Logout
`POST /api/v1/auth/logout` → clears cookie.

#### 8.2.3 Current user
`GET /api/v1/auth/me`

Response:
```json
{
  "id": "uuid",
  "username": "joe",
  "display_name": "Joe"
}
```

### 8.3 PAT endpoints (required day one)

#### 8.3.1 Create PAT
`POST /api/v1/tokens`

Request:
```json
{
  "name": "laptop-cli",
  "expires_at": "2026-12-31T00:00:00Z"
}
```

Response (secret only returned once):
```json
{
  "id": "uuid",
  "name": "laptop-cli",
  "token": "umami_pat_XXXXXXXXXXXXXXXX",
  "created_at": "..."
}
```

#### 8.3.2 List PATs (no secrets)
`GET /api/v1/tokens`

Response:
```json
[
  { "id":"uuid", "name":"laptop-cli", "last_used_at":null, "expires_at":null, "created_at":"..." }
]
```

#### 8.3.3 Revoke PAT
`DELETE /api/v1/tokens/{id}` → `204`

### 8.4 User management (admin provisions)
Because all users are admin, any authenticated user may manage users (v1).

#### 8.4.1 List users
`GET /api/v1/users`

#### 8.4.2 Create user
`POST /api/v1/users`

Request:
```json
{ "username": "shannon", "password": "...", "display_name": "Shannon" }
```

#### 8.4.3 Deactivate user
`PUT /api/v1/users/{id}/deactivate` *(optional; recommended)*

### 8.5 Recipe Books endpoints

- `GET /api/v1/recipe-books`
- `POST /api/v1/recipe-books`
- `PUT /api/v1/recipe-books/{id}`
- `DELETE /api/v1/recipe-books/{id}` *(consider preventing delete if recipes exist or set null)*

### 8.6 Tags endpoints

- `GET /api/v1/tags`
- `POST /api/v1/tags`
- `PUT /api/v1/tags/{id}`
- `DELETE /api/v1/tags/{id}`

### 8.7 Recipes endpoints

#### 8.7.1 List recipes
`GET /api/v1/recipes?q=&book_id=&tag_id=&include_deleted=`

- Default: `deleted_at is null`
- Sort: `updated_at desc`
- Search: title only when `q` present

Response:
```json
{
  "items": [
    {
      "id":"uuid",
      "title":"Chicken Soup",
      "servings":4,
      "prep_time_minutes":15,
      "total_time_minutes":60,
      "source_url":null,
      "notes":null,
      "recipe_book_id":"uuid",
      "tags":[{"id":"uuid","name":"soup"}],
      "updated_at":"2025-12-13T12:34:56Z"
    }
  ],
  "next_cursor": null
}
```

Pagination:
- Cursor pagination recommended (stable, maintainable); offset acceptable for personal scale.

#### 8.7.2 Get recipe detail
`GET /api/v1/recipes/{id}`

Response includes ingredients and steps:
```json
{
  "id":"uuid",
  "title":"Chicken Soup",
  "servings":4,
  "prep_time_minutes":15,
  "total_time_minutes":60,
  "source_url":"https://example.com",
  "notes":"Family favorite",
  "recipe_book_id":"uuid",
  "tags":[{"id":"uuid","name":"soup"}],
  "ingredients":[
    {"id":"uuid","position":1,"quantity":1.0,"quantity_text":"1","unit":"lb","item":"chicken","prep":null,"notes":null,"original_text":"1 lb chicken"}
  ],
  "steps":[
    {"id":"uuid","step_number":1,"instruction":"Boil the chicken."}
  ],
  "created_at":"...",
  "created_by":"uuid",
  "updated_at":"...",
  "updated_by":"uuid",
  "deleted_at": null
}
```

#### 8.7.3 Create recipe
`POST /api/v1/recipes`

Request:
```json
{
  "title":"Chicken Soup",
  "servings":4,
  "prep_time_minutes":15,
  "total_time_minutes":60,
  "source_url":"https://example.com",
  "notes":"Family favorite",
  "recipe_book_id":"uuid",
  "tag_ids":["uuid","uuid"],
  "ingredients":[
    {"position":1,"quantity":1.0,"quantity_text":"1","unit":"lb","item":"chicken","prep":null,"notes":null,"original_text":"1 lb chicken"}
  ],
  "steps":[
    {"step_number":1,"instruction":"Boil the chicken."}
  ]
}
```

Response: `201` with recipe detail.

Validation rules:
- Title required, trimmed, min length 1
- Servings > 0
- Prep/total >= 0
- Steps numbered consecutively 1..N (enforced in app; DB unique constraint prevents duplicates)
- Ingredients positions unique within recipe

#### 8.7.4 Update recipe (replace semantics)
`PUT /api/v1/recipes/{id}`

- Last write wins; no versioning required in v1.
- Update should be transactional: update recipe + replace ingredients + replace steps + tags.

Response: updated recipe detail.

#### 8.7.5 Soft delete recipe
`DELETE /api/v1/recipes/{id}`  
Behavior:
- Set `deleted_at = now()`
- Update `updated_at/updated_by`
Response: `204`

#### 8.7.6 Restore recipe (recommended)
`PUT /api/v1/recipes/{id}/restore`  
Behavior: set `deleted_at = null` and update audit fields.

---

## 9. Backend implementation details (Go)

### 9.1 Service layout (recommended)
```
/backend
  /cmd/api
  /internal
    /auth
    /db
    /http
    /models
    /recipes
    /users
    /tokens
  /migrations
  openapi.yaml (optional but recommended)
```

### 9.2 Database access
- Prefer `sqlc` for CRUD and list queries.
- Use explicit SQL queries for complex operations (replace ingredients/steps).

Transactional update pattern (recipe update):
1. `BEGIN`
2. Update `recipes` row (ensure `deleted_at is null` unless include_deleted).
3. Delete existing ingredients/steps for recipe.
4. Insert new ingredients/steps.
5. Replace tag relations in `recipe_tags`.
6. `COMMIT`

### 9.3 Audit field enforcement
- App sets `created_at/created_by/updated_at/updated_by` on insert/update.
- DB validates:
  - `NOT NULL`
  - `FOREIGN KEY` references `users(id)`
  - Optional check constraints:
    - `updated_at >= created_at`
- Consider default values for timestamps (`now()`) to reduce foot-guns, but keep app authoritative.

### 9.4 Authentication
- Password hashing: Argon2id
- Sessions:
  - `sessions` table in Postgres or signed cookies.
  - Recommended: server-side sessions to simplify revocation.

### 9.5 PAT authentication
- PAT token format:
  - Prefix: `umami_pat_` + random base64url secret
- Store only a hash:
  - `token_hash = sha256(secret)` or Argon2 hash for stronger resistance
- Validate:
  - Lookup by hash index (store `sha256` and index it) and compare
- Update `last_used_at` on successful auth (best effort)

### 9.6 Logging (JSONL)
Log each request with:
- `ts`
- `level`
- `msg`
- `request_id`
- `remote_ip`
- `method`
- `path`
- `status`
- `duration_ms`
- `user_id` (if authenticated)
- `auth_type` (`session` | `pat`)

---

## 10. Frontend specification (React + TS + Vite)

### 10.1 UX principles (MVP)
- Mobile-first, readable, minimal UI
- Recipes list is the primary entry point
- Recipe detail page optimized for reading

### 10.2 App routes (recommended)
- `/login`
- `/recipes`
- `/recipes/new`
- `/recipes/:id`
- `/recipes/:id/edit`
- `/books`
- `/tags`
- `/settings`
  - `/settings/tokens`
  - `/settings/users`

### 10.3 Component structure (suggested)
- `AppShell`
- `TopNav`
- `RecipeList`
- `RecipeDetail`
- `RecipeEditor` (used for create and edit)
- `BookList`, `TagList`
- `TokenManager`, `UserManager`

### 10.4 API integration approach (maintainability)
Recommendation:
- Maintain an **OpenAPI** document in the backend.
- Generate a typed TS client (`openapi-typescript` or similar).
- Wrap client calls in TanStack Query hooks:
  - `useRecipes({q, tagId, bookId})`
  - `useRecipe(id)`
  - `useCreateRecipe()`, `useUpdateRecipe()`

### 10.5 Forms and validation
- Use a form library (React Hook Form) + runtime validation (zod) to keep correctness high.
- Validation mirrors backend rules (required fields, positive integers).

### 10.6 PWA requirements (v1)
- Provide `manifest.webmanifest` with:
  - name, short_name
  - icons (192/512)
  - display: standalone
- Register service worker for installability.
- Offline is not required; keep caching limited to static assets.

---

## 11. Deployment specification (Ubuntu + Docker Compose + Caddy)

### 11.1 Containers
- `db`: postgres:18
- `api`: custom Go API image
- `web`: static build served by Caddy (or Caddy serves files via mounted volume)
- `caddy`: caddy:latest

### 11.2 Network and ports
- Expose only Caddy ports:
  - 80 (HTTP → redirect to HTTPS)
  - 443 (HTTPS)
- Keep Postgres port internal (not exposed on host).

### 11.3 TLS for “LAN-only” with Let’s Encrypt
**Important constraint:** Let’s Encrypt HTTP-01 challenges require public reachability on 80/443.
To keep LAN-only while using a public domain + Let’s Encrypt, use **DNS-01 challenge**.

Implementation approach:
- Configure Caddy with DNS provider plugin (or use Caddy with supported DNS module image).
- Provide DNS API credentials via `.env`.

### 11.4 Example `docker-compose.yml` (skeleton)
```yaml
services:
  db:
    image: postgres:18
    environment:
      POSTGRES_DB: umami
      POSTGRES_USER: umami
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks: [internal]

  api:
    build: ./backend
    environment:
      DATABASE_URL: postgres://umami:${POSTGRES_PASSWORD}@db:5432/umami?sslmode=disable
      UMAMI_SESSION_SECRET: ${UMAMI_SESSION_SECRET}
      UMAMI_LOG_FORMAT: json
    depends_on: [db]
    networks: [internal]

  caddy:
    image: caddy:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./deploy/Caddyfile:/etc/caddy/Caddyfile:ro
      - ./frontend/dist:/srv:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on: [api]
    networks: [internal]

networks:
  internal:

volumes:
  pgdata:
  caddy_data:
  caddy_config:
```

### 11.5 Example `Caddyfile` (skeleton)
```caddy
your.domain.example {
  encode gzip zstd

  # Static frontend
  root * /srv
  try_files {path} /index.html
  file_server

  # API
  handle_path /api/* {
    reverse_proxy api:8080
  }

  log {
    output file /data/access.log
    format json
  }
}
```

### 11.6 Backups
- Nightly `pg_dump` stored locally on the Ubuntu host.
- Provide a host cron job or systemd timer that runs:
  - `docker exec <db_container> pg_dump ... > /backups/umami_YYYYMMDD.sql`
- Retention: decide (e.g., 14 daily + 8 weekly). (TBD)

---

## 12. Security requirements

- Passwords must be hashed (Argon2id) with sane parameters.
- Session cookies must be:
  - `HttpOnly`
  - `Secure` (requires TLS)
  - `SameSite=Lax`
- PAT secrets displayed once; store hashes only.
- Rate limiting is optional for LAN MVP; consider basic login rate limiting.
- Caddy TLS with Let’s Encrypt via DNS-01 for LAN-only setup.

---

## 13. Testing requirements

### Backend
- Unit tests for:
  - ingredient parsing/validation (if any)
  - recipe create/update validation
  - PAT auth
- Integration tests:
  - API endpoints against a test Postgres
  - recipe update transaction correctness

### Frontend
- Component tests for RecipeEditor validations
- Smoke tests for routing and list/detail rendering

---

## 14. Acceptance criteria (MVP)

1. Three distinct user accounts can log in.
2. Any user can:
   - create a recipe with structured ingredients and numbered steps
   - view recipe detail
   - edit recipe
   - soft delete and (if implemented) restore recipe
3. Recipe listing:
   - sorts by recently updated
   - supports title-only search
4. Recipe books and tags:
   - CRUD works
   - recipes can be assigned to one book and multiple tags
5. PATs:
   - a user can create a PAT and use it to call `/api/v1/recipes`
   - tokens are listed and can be revoked
6. Deployment:
   - runs via docker compose on Ubuntu
   - served via Caddy with TLS
7. Logs:
   - API emits JSONL structured logs for each request.

---

## 15. Future roadmap (explicitly post-MVP)
- Meal planning
- Grocery lists
- Exports (JSON, Markdown)
- URL import/parsing (schema.org)
- Offline support (IndexedDB caching)
- Event stream (SSE)
- Conflict detection (ETags / if-match) if needed

---

## 16. Glossary
- **PAT**: Personal Access Token used for API authentication.
- **Soft delete**: mark as deleted without removing records from DB.
- **PWA**: Progressive Web App, installable web experience.

---
