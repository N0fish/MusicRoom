# Security overview

This document describes the main security mechanisms implemented in the **Music Room** backend,
with a focus on the **API Gateway** service (entry point for web and mobile clients).

The goal is **not** to be unbreakable, but to:
- protect users from obvious attacks (bruteforce, basic abuse, simple token theft),
- ensure that an authenticated user can only access their own data,
- be able explain current protections and known limitations.

---

## 1. Architecture and trust model

- The system is split into multiple backend services:
  - `auth-service` (authentication, tokens),
  - `user-service` (profiles, friends),
  - `playlist-service` (playlists and tracks),
  - `vote-service` (events, invites, voting),
  - `realtime-service` (WebSocket / realtime features),
  - `music-provider-service` (searching tracks in external providers),
  - `api-gateway` (single public entry point).
- All traffic from frontend and mobile apps goes **only through the API Gateway**.  
  Direct calls to internal services (auth, user, playlist, etc.) are not exposed.

The gateway is responsible for:
- validating JWT access tokens (authentication),
- enforcing per-route access rules (who can call what),
- rate limiting, body size limiting and basic CORS,
- logging information required by the subject (platform, device, app version).

Internal services trust the gateway to:
- forward the authenticated user id (`X-User-Id`),
- forward the original client IP (for logging / geo checks),
- shield them from basic HTTP abuse.

---

## 2. Authentication (JWT)

- Clients authenticate using **JWT access tokens**, issued by `auth-service`.
- Tokens are sent via the standard HTTP header:

```http
Authorization: Bearer <access_token>
```
The api-gateway validates tokens with jwtAuthMiddleware:
- checks that the Authorization header exists,
- requires the "Bearer <token>" format,
- verifies the signature using JWT_SECRET,
- parses structured claims (uid, email, emailVerified, typ, etc.),
- rejects tokens where TokenType is not "access".

If validation fails, the gateway returns:
```json
HTTP 401 Unauthorized
{ "error": "invalid token" }
```
In addition:
- On startup, the gateway refuses to run if `JWT_SECRET` is empty:
```text
api-gateway: JWT_SECRET is empty, cannot start without JWT validation
```
This guarantees that in any realistic environment (dev/staging/prod) the gateway
always runs with token validation enabled.

The `auth-service` exposes `/auth/refresh` to issue new access tokens using refresh tokens.

---


## 3. Authorization
Authorization is enforced at two levels:
1. **At the gateway level** using route groups and JWT middleware:
- all sensitive endpoints are protected with `security: bearerAuth` (OpenAPI) and
`jwtAuthMiddleware` (code),
- examples:
- - `/users/me`, `/users/me/avatar/random`, `/users/me/friends/...`,
- - `/playlists` (create/update/delete, invite management),
- - `/events/...` and `/events/{id}/vote`,
- - `/music/search`.

2. **At the service level (auth / user / playlist / vote)**:
- services receive `X-User-Id` and `X-User-Email` headers from the gateway,
- they check ownership, visibility and invite rules,
- examples:
- - playlist ownership and edit rights,
- - private playlists and events,
- - invite-only access.

This combination ensures that:
- unauthenticated users cannot access protected routes (gateway side),
- authenticated users can only access **their own** data or data explicitly shared with them (service side).

---

## 4. Rate limiting and bruteforce protection
The gateway implements several layers of rate limiting to protect against bruteforce
and generic abuse:

### 4.1 Global per-IP rate limit

Middleware: `rateLimitMiddleware(RATE_LIMIT_RPS)`
- Applies to all requests, based on client IP.
- Sliding window of 1 second; counts requests per IP.
- Defaults to RATE_LIMIT_RPS=20 if not overridden via environment variable.
- If the limit is exceeded, the gateway returns:
```json
HTTP 429 Too Many Requests
{ "error": "too many requests" }
```
- Also sets the `Retry-After` header to help clients back off.

### 4.2 Login-specific rate limit
Middleware: `loginRateLimitMiddleware`
- Applied only to `/auth/login`.
- Limits how often a single IP can attempt to log in.
- If requests are too frequent, the gateway returns:
```json
HTTP 429 Too Many Requests
{ "error": "too many login attempts" }
```
This helps mitigate password bruteforce attacks on the login endpoint.

### 4.3 Playlist creation rate limit
Middleware: `playlistCreateRateLimitMiddleware`
- Applied only to `POST /playlists`.
- Limits how frequently a single IP can create new playlists.
- If requests are too frequent, the gateway returns:
```json
HTTP 429 Too Many Requests
{ "error": "too many playlist creations" }
```
This protects the backend from abuse (e.g. automatically creating thousands of playlists).

---

## 5. Request size limits
Middleware: `bodySizeLimitMiddleware(maxBytes)`
- Used on endpoints that accept JSON bodies, for example:
- - `PATCH /users/me`,
- - friend request endpoints,
- - avatar generation,
- - other POST/PATCH routes where large bodies are not expected.
- - If the request body is too large, the gateway returns:
```json
HTTP 413 Request Entity Too Large
{ "error": "request body too large" }
```
This prevents clients from accidentally or maliciously sending huge payloads
to the backend.

---

## 6. CORS and browser security
Middleware: `corsMiddleware`
- Controls which origins are allowed to call the API from a browser.
- Uses the `CORS_ALLOWED_ORIGIN` environment variable:
- - in development: `CORS_ALLOWED_ORIGIN=http://localhost:5175` (Go frontend),
- - in production: should be set to the real frontend / mobile web origin.

The middleware:
- sets `Access-Control-Allow-Origin` to the configured origin,
- allows required headers (`Authorization`, `Content-Type`),
- allows standard methods (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`),
- handles `OPTIONS` preflight requests with `HTTP 204 No Content`,
- enables credentials with `Access-Control-Allow-Credentials: true`.
This ensures that only trusted frontend origins can call the API from the browser.

---

## 7. Logging and audit
Middleware: `requestLogMiddleware`
- For every request coming through the gateway, we log:
- - HTTP method and path,
- - client platform (`X-Client-Platform` header),
- - device model (`X-Client-Device` header),
- - application version (`X-Client-App-Version` header),
- - client IP address (based on `X-Real-IP`, `X-Forwarded-For` or `RemoteAddr`).

Example log format:
```text
req: GET /users/me platform=iOS device=iPhone13 app=1.0.3 ip=192.168.1.10
```
This matches the subject requirement:
  Any action on the mobile application must generate logs on the back-end:
  - Platform
  - Device
  - Application Version
Frontends (web and mobile) are responsible for setting these headers.

---

## 8. Secrets and environment variables
All sensitive configuration is stored in environment variables, not in the codebase:

- `JWT_SECRET` — HMAC secret for signing and verifying JWT tokens.
- `AUTH_SERVICE_URL`, `USER_SERVICE_URL`, `PLAYLIST_SERVICE_URL`, etc. — internal service URLs.
- `CORS_ALLOWED_ORIGIN` — allowed browser origin for CORS.
- `RATE_LIMIT_RPS`, `USER_PATCH_BODY_LIMIT`, `AVATAR_RPS`, `FRIEND_REQUEST_RPS`, etc. — rate limits and size limits.
- `.env` files are:
  - stored locally on each developer’s machine,
  - ignored by git via `.gitignore`, as required by the subject:
    - For obvious security reasons, any credentials, API keys, env variables etc...
    must be saved locally in a `.env` file and ignored by git.
No secrets are hardcoded in the repository.

---

## 9. Known limitations and future improvements - NO
This project is not intended to be perfectly secure, but to demonstrate reasonable protections.

Possible improvements (not all implemented yet):
- Per-user login lockout after several failed attempts for the same account.
- Using HTTPS/TLS termination in front of the gateway (reverse proxy / load balancer).
- Storing refresh tokens with rotation and blacklist support (revocation on logout).
- More granular rate limiting per endpoint and per user.
- Structured logging and forwarding logs to a central log collector.
- Additional validation on incoming payloads (e.g. stricter length checks).

These points can be discussed during the defense as identified hazards and practicable protections.
