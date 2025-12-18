# Backend Security Model

Last updated: 2025-12-18  
Scope: `backend/` (HTTP API)

This document defines the backend security model and handler-facing rules for:

- authentication types (session cookie vs Personal Access Token),
- authorization expectations (what an authenticated principal may do),
- CSRF protections for cookie-authenticated unsafe requests,
- rate limiting for login + token issuance,
- token/session lifecycle semantics.

It is intended to be the source-of-truth for backend behavior and OpenAPI documentation.

## Principals

- **Anonymous**: no authentication.
- **Session**: browser-style cookie session created by `POST /api/v1/auth/login`.
- **PAT (Bearer)**: Personal Access Token provided via `Authorization: Bearer <token>`.

## Authentication model

### Session cookies

- A successful login creates a server-side session row and sets a session cookie.
- Session cookies are:
  - `HttpOnly`
  - `SameSite=Lax`
  - `Secure` depends on environment (`SESSION_COOKIE_SECURE`)
  - `Expires` is fixed at creation time (`SESSION_TTL_HOURS`)
- Session expiration is **fixed** (no sliding expiry).

### Personal Access Tokens (PAT)

- PATs are created via `POST /api/v1/tokens`.
- The raw token secret is returned **only once** at creation time.
- Only a hash is stored in the database.
- PATs can be created with an optional `expires_at`.
- PAT usage updates `last_used_at` (best-effort).

## Authorization model

The current product is a **shared, single-tenant workspace**:

- Any authenticated active user may manage all first-class resources (recipes, tags, recipe-books).
- There is no per-resource ownership enforcement beyond `created_by` metadata.
- Any authenticated active user may manage users and PATs (subject to endpoint behavior).

If/when roles (admin vs member) or ownership enforcement are introduced, this document must be updated and enforced consistently in handlers and OpenAPI.

### Authorization matrix

Legend:

- ✅ allowed
- ❌ denied
- ⚠️ conditional (see notes)

| Resource / action | Anonymous | Session | PAT |
| --- | --- | --- | --- |
| `POST /auth/login` | ✅ | ✅ | ✅ |
| `POST /auth/logout` | ❌ | ✅ | ✅ (no-op) |
| `GET /auth/me` | ❌ | ✅ | ✅ |
| `GET /tokens` | ❌ | ✅ | ✅ |
| `POST /tokens` | ❌ | ✅ | ✅ |
| `DELETE /tokens/{id}` | ❌ | ✅ | ✅ |
| `GET /users` | ❌ | ✅ | ✅ |
| `POST /users` | ❌ | ✅ | ✅ |
| `PUT /users/{id}/deactivate` | ❌ | ✅ | ✅ |
| `GET/POST/PUT/DELETE` `recipe-books/*` | ❌ | ✅ | ✅ |
| `GET/POST/PUT/DELETE` `tags/*` | ❌ | ✅ | ✅ |
| `GET/POST/PUT/DELETE` `recipes/*` | ❌ | ✅ | ✅ |

Notes:

- “Authenticated active user” means the user account is `is_active=true`, and the presented session/PAT is valid and not expired.
- `GET /auth/me` reports the current authenticated principal (session or PAT).
- `POST /auth/logout` clears the session cookie. For bearer PAT, the operation does not revoke the PAT; clients should instead delete the token via `/tokens/{id}`.

## CSRF model (cookie-auth unsafe methods)

Cookie authentication is vulnerable to CSRF unless unsafe methods require a request-bound secret.

Decision:

- For requests authenticated via **session cookie**, all unsafe methods (`POST`, `PUT`, `PATCH`, `DELETE`) require a CSRF token.
- For requests authenticated via **bearer PAT**, CSRF protection is not applied (bearer tokens are not automatically attached by browsers cross-site).

Mechanism (double-submit):

- On successful login, the server sets an additional `csrf_token` cookie (not `HttpOnly`).
- Unsafe requests authenticated via session cookie MUST send the header `X-CSRF-Token` with the same value as the `csrf_token` cookie.
- If missing/mismatched, return `403` with a standard problem response (`code=forbidden`).

## Rate limiting

Decision:

- Add rate limiting for:
  - `POST /api/v1/auth/login` (to mitigate credential stuffing and brute force)
  - `POST /api/v1/tokens` (token issuance can be abused for resource exhaustion and auditing blind spots)

Expected properties:

- Configurable limits (via env) with sensible defaults for local dev.
- Deterministic behavior suitable for integration tests (e.g., small limits under test).
- When exceeded, return `429` with a standard problem response (`code=rate_limited`).

## Request parsing hardening

Decision:

- JSON request bodies are limited to a maximum size (default `2 MiB`).
  - If exceeded, return `413` with `code=request_too_large`.
- JSON decoding is **strict** by default:
  - Unknown fields are rejected (handlers use `DisallowUnknownFields`).
  - Trailing non-whitespace after the JSON value is rejected.

Configuration:

- `MAX_JSON_BODY_BYTES` (defaults to `2097152`)
- `STRICT_JSON` (defaults to `true`)

## Logout semantics

Decision:

- `POST /api/v1/auth/logout` requires authentication (session or bearer).
- The endpoint always returns `204` and clears the session cookie.
- If a session cookie is present, its server-side session row is deleted (best-effort).
- If a bearer PAT is present, no token is revoked. Revocation is done via `DELETE /api/v1/tokens/{id}`.
