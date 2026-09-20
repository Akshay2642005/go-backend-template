# Go Backend Template

A production-ready Go backend template using Echo, Postgres, Redis, and Clerk.
Clean layered architecture with constructor dependency injection, typed context
propagation, RFC 7807 errors, and a working Posts CRUD example to copy from.

## Stack

| Concern      | Choice                                              |
|--------------|-----------------------------------------------------|
| HTTP         | Echo v4                                             |
| Database     | Postgres 16 via pgx/v5, migrations via tern         |
| Cache/queue  | Redis 8 via go-redis (single, cluster, or sentinel) |
| Auth         | Clerk (JWT via `clerkhttp`)                         |
| Jobs         | asynq (Redis-backed task queue)                     |
| Email        | Resend                                              |
| Config       | koanf, `BOILERPLATE_` env prefix                    |
| Logging      | zerolog (structured)                                |
| Tracing      | OpenTelemetry OTLP/gRPC (disabled by default)       |
| Validation   | validator/v10 + custom `Validate()`                 |
| Errors       | RFC 7807 Problem Details                            |
| Tests        | testcontainers (real Postgres/Redis in Docker)      |

## Quick Start

Prerequisites: Go 1.26+, Docker, [Task](https://taskfile.dev).

```bash
# 1. Install deps
go mod download

# 2. Configure
cp .env.example .env
# Edit .env — at minimum set BOILERPLATE_AUTH.SECRET_KEY to a real Clerk key

# 3. Start Postgres + Redis
task compose:up        # or: docker compose up -d

# 4. Migrate
task migrations:up

# 5. Run
task run
```

Server starts on `:8080` (configurable). Docs at `http://localhost:8080/docs`.

```bash
task test              # run all tests (needs Docker running)
task tidy              # go fmt + go mod tidy + verify
task migrations:new name=add_things   # scaffold a migration
```

## Project Layout

```
cmd/go-boilerplate/     # main.go — wiring only, no logic
internal/
  config/               # koanf structs, BOILERPLATE_ prefix, validation
  server/               # Server struct: Config, Logger, DB, Cache, Job
  database/             # pgx pool, tern migrator, tx helpers
  database/migrations/  # 001_setup.sql, 002_posts.sql, ...
  repository/           # data access, one file per resource
  service/              # business logic, one file per resource
  handler/              # HTTP adapters, one file per resource
  model/                # domain types + request/response DTOs
  router/               # route registration (router.go + *_routes.go)
  middleware/           # cross-cutting HTTP concerns
  cache/                # Redis operations, cache-aside, singleflight
  lib/                  # shared libs: breaker, email, job, propagation, utils
  errs/                 # RFC 7807 problem types + helpers
  validation/           # BindAndValidate, Validatable interface
  otel/                 # OpenTelemetry init (no-op when disabled)
  logger/               # zerolog setup
  testing/              # testcontainers helpers for tests
static/                 # openapi.json (source of truth), Scalar UI
templates/emails/       # HTML email templates
```

## Architecture

### Request Flow

```
Client
  │
  ▼
Echo router (/api/v1/...)
  │
  ▼  middleware pipeline (see below)
ContextEnhancer ── injects PropagatedValues (requestID, userID)
  │                  into context.Context
  ▼
Handler (thin adapter)
  │  Handle() → bind + validate + log, then closure(ctx, req)
  ▼
Service (business logic)
  │  reads userID via propagation.UserIDFrom(ctx)
  ▼
Repository (SQL via pgx)
  │
  ▼
Postgres
```

`echo.Context` never leaves the HTTP layer. Everything below the handler
takes `context.Context`, which carries request-scoped values.

### Dependency Injection

Constructor DI through `*server.Server`. Wired once in `main.go`:

```go
repos    := repository.NewRepositories(srv)      // repos.Post (from s.DB.Pool)
services := service.NewServices(srv, repos)      // services.Post (from repos.Post)
handlers := handler.NewHandlers(srv, services)   // handlers.Post (from services.Post)
r        := router.NewRouter(srv, handlers)
```

Rules:

- Each layer receives dependencies from the layer above. Never construct
  them ad-hoc (no `NewPostService(pool)` inside a handler).
- Repositories take `*pgxpool.Pool` (or the `Querier` interface for tx support).
- Services take `*repository.XxxRepository`.
- Handlers take `*service.XxxService` plus `*server.Server` (for `Handler` base).
- To add a resource: add a field to `Repositories`, `Services`, `Handlers`,
  and wire it in each `New*` constructor. See "Adding a Resource" below.

### Context Propagation

`ContextEnhancer` middleware runs on every request and injects:

```go
type PropagatedValues struct {
    RequestID string
    UserID    string   // from Clerk JWT (via auth middleware)
    TraceID   string
    SpanID    string
}
```

Services read these without touching Echo:

```go
authorID := propagation.UserIDFrom(ctx)  // "" if absent
```

The same values flow into background jobs: `lib/job` serializes them into
the task payload envelope on enqueue and re-injects them on dequeue. A job
calling `PostService.Create` sees the original request's user ID with no
extra code. This is concurrency-safe — every request gets a fresh context
and a fresh `PropagatedValues` struct; nothing is shared or mutated.

### Handler Pattern

One function per operation. Closures receive `(ctx context.Context, req *Req)`
— no `echo.Context`, no shadowing:

```go
func (h *PostHandler) Create(c echo.Context) error {
    return Handle(h.Handler, func(ctx context.Context, req *model.CreatePostRequest) (interface{}, error) {
        post, err := h.postService.Create(ctx, *req)
        if err != nil {
            return nil, err
        }
        return model.PostFromModel(post), nil
    }, 201, &model.CreatePostRequest{})(c)
}
```

`Handle` runs the pipeline: bind → validate → execute → structured log →
JSON response with the given status. Variants:

- `Handle` — returns a JSON body (200/201/...)
- `HandleNoContent` — 204, no body
- `HandleFile` — file download (`[]byte` + filename + content type)

Endpoints needing path params or raw Echo features (`c.Param`, `c.JSON`,
`c.NoContent`) skip the wrapper — see `GetByID`/`Delete` in
`post_handler.go`.

### Middleware Pipeline

Global (in `router.go`, in order):

1. RateLimit — Redis sliding window, fail-open
2. CORS, Secure (security headers), RequestTimeout
3. RequestID, Tracing (OTel spans), ContextEnhancer (logger + propagation)
4. Metrics, RequestLogger, Recover
5. Compression (optional), BodyLogger (optional, debug)

Per-group (`/api/v1`): CacheControl (ETag / 304), Idempotency
(`Idempotency-Key` replay for POST/PUT/PATCH).

Per-route: `RequireAuth` (Clerk) on protected groups like `/posts`.

### Errors (RFC 7807)

All errors are Problem Details with `application/problem+json`:

```json
{
  "type": "https://api.example.com/problems/validation",
  "title": "Validation failed",
  "status": 400,
  "detail": "...",
  "instance": "/api/v1/posts (req: abc-123)",
  "errors": [{ "field": "title", "error": "is required" }]
}
```

Helpers in `internal/errs`: `ProblemNotFound`, `ProblemValidation`,
`NewBadRequestError`, etc. pgx errors are mapped to human-readable
problems via `internal/sqlerr`.

### Configuration

koanf with `BOILERPLATE_` prefix, `.` delimiter, validated on load.
Every key in `.env.example` maps to a struct field:

| Prefix | Struct | Notes |
|---|---|---|
| `BOILERPLATE_PRIMARY.ENV` | `Primary` | `local` skips forced migrate |
| `BOILERPLATE_SERVER.*` | `ServerConfig` | ports, timeouts, rate limit, idempotency, shutdown |
| `BOILERPLATE_DATABASE.*` | `DatabaseConfig` | pgx pool, retry, `AUTO_MIGRATE` |
| `BOILERPLATE_DB_DSN` | — | CLI only (tern via task); app builds its own DSN |
| `BOILERPLATE_AUTH.SECRET_KEY` | `AuthConfig` | Clerk key — must be real, not `"secret"` |
| `BOILERPLATE_REDIS.*` | `RedisConfig` | mode: single/cluster/sentinel, pool, TLS, TTL |
| `BOILERPLATE_INTEGRATION.*` | `IntegrationConfig` | Resend API key |
| `BOILERPLATE_OBSERVABILITY.*` | `ObservabilityConfig` | logging, tracing, metrics, health checks |

Redis and observability configs merge over defaults — unset fields get
sane values, so local dev works with a minimal `.env`.

### Database & Migrations

- Migrations live in `internal/database/migrations/`, applied by tern.
- `task migrations:new name=foo` scaffolds `NNN_foo.sql` (up + down).
- `task migrations:up` applies via `BOILERPLATE_DB_DSN`.
- `AUTO_MIGRATE=true` runs migrations on startup (useful in containers).
- `database.WithTx(ctx, pool, fn)` runs a function in a transaction with
  auto commit/rollback. Repositories accept the `Querier` interface so the
  same code works inside or outside a tx.
- Connection retry with backoff on startup (`CONNECT_RETRIES`).

### Redis & Caching

`internal/cache` provides typed helpers (`Get`, `Set`, `GetOrSet`,
`Delete`, `Increment`) over `redis.UniversalClient`:

- Topologies: single, cluster, sentinel (`REDIS.MODE`).
- Cache-aside `GetOrSet` with singleflight stampede protection.
- TTL jitter prevents synchronized expiry stampedes.
- Fail-open: Redis errors log and fall through to origin — requests never block.
- Key prefix namespacing per environment; long segments SHA-256 hashed.
- Hit/miss/error counters + slow-op logging.
- Circuit breaker (`lib/breaker`) trips on repeated Redis failures.

### Background Jobs

asynq over Redis. `lib/job` owns the client, server, mux, and handlers:

- Queues: critical, default, low.
- Cron schedules registered in `job.go`.
- Context propagation via payload envelope: enqueue with `job.Envelope(ctx,
  payload)`, extract on the worker with `job.ExtractMetadata(ctx, task)`.
  Request ID, user ID, and trace IDs survive the round trip.
- Welcome-email task + Resend integration included as an example.

### Auth

Clerk JWTs. `AuthMiddleware.RequireAuth` validates the token and sets
`user_id` / `user_role` on the echo context. `ContextEnhancer` (which runs
on all routes) then copies the user ID into `PropagatedValues`, so services
read it from `context.Context` without importing Echo or Clerk.

Posts use `author_id` from the Clerk user — no foreign key to a users
table (Clerk owns identity).

### Observability

- Logs: zerolog, JSON in prod / console locally. Request logger +
  per-request child loggers with request ID, method, path, user.
- Tracing: OTel OTLP/gRPC export, `otelecho` spans. Disabled by default
  (`TRACING.ENABLED=false` = no-op).
- Metrics: request duration histogram + count, cache hit/miss counters.
- Health: `/healthz` (liveness), `/readyz` (readiness: DB + Redis checks),
  `/status` (legacy full details).
- Body logging: opt-in request/response logging, 1 KB truncation.

### Graceful Shutdown

Ordered teardown on SIGINT/SIGTERM: HTTP drain → DB pool → Redis →
job server. `DRAIN_TIMEOUT` (default 15s) bounds in-flight requests;
`SHUTDOWN_TIMEOUT` (default 30s) bounds the whole sequence. OTel flushes
first so no traces are lost.

## Adding a Resource

Copy the Posts stack. Six files, then two wiring edits:

1. **Model** (`internal/model/widget.go`) — domain struct + request/response
   DTOs. Requests implement `Validate() error` (the `Validatable`
   interface) for custom rules beyond struct tags.
2. **Migration** (`internal/database/migrations/003_widgets.sql`) —
   `task migrations:new name=widgets`, write up/down SQL.
3. **Repository** (`internal/repository/widget_repository.go`) — SQL via
   `s.DB.Pool` (or `Querier`). Add `Widget *WidgetRepository` to
   `Repositories` and wire in `NewRepositories`.
4. **Service** (`internal/service/widget_service.go`) — business logic,
   takes `*repository.WidgetRepository`. Add `Widget *WidgetService` to
   `Services`, wire in `NewServices`.
5. **Handler** (`internal/handler/widget_handler.go`) — one function per
   operation with `Handle`. Add `Widget *WidgetHandler` to `Handlers`,
   wire in `NewHandlers`.
6. **Routes** (`internal/router/widget_routes.go`) — group + auth +
   method bindings. Register from `router.go`.
7. **OpenAPI** (`static/openapi.json`) — add paths + schemas (served at
   `/docs` via Scalar).

## Testing

Tests use testcontainers — real Postgres/Redis containers, no mocks for
infrastructure:

```bash
task test            # go test ./... (Docker must be running)
go test ./internal/cache/... -run TestGetOrSet -v
```

Helpers in `internal/testing`: `SetupTestDB` (fresh Postgres per package +
migrations applied), transaction helpers (`WithTransaction`,
`WithRollbackTransaction` for test isolation), assertion helpers.

Conventions:

- `*_test.go` next to the code it tests.
- Table-driven tests with `testify/require`.
- Repository/service tests get a real DB from `SetupTestDB`; wrap each
  case in a rollback transaction so tests don't leak state.
- Middleware/handler tests use `httptest` + Echo directly (see
  `body_logger_test.go`, `timeout_test.go`).
- Keep tests hermetic: no reliance on `.env`, no shared containers
  across packages.

## API Reference

Full spec in `static/openapi.json`, interactive explorer at `/docs`.

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/healthz` | — | Liveness |
| GET | `/readyz` | — | Readiness (DB + Redis) |
| GET | `/status` | — | Legacy health details |
| GET | `/docs` | — | Scalar API explorer |
| GET | `/api/v1/posts` | Clerk | List (page, limit, status) |
| POST | `/api/v1/posts` | Clerk | Create (`Idempotency-Key` supported) |
| GET | `/api/v1/posts/:id` | Clerk | Get one |
| PUT | `/api/v1/posts/:id` | Clerk | Update |
| DELETE | `/api/v1/posts/:id` | Clerk | Delete (204) |

Protected routes take `Authorization: Bearer <clerk-jwt>`.

## Deployment Checklist

- [ ] Real `BOILERPLATE_AUTH.SECRET_KEY` (Clerk live key)
- [ ] Real `BOILERPLATE_INTEGRATION.RESEND_API_KEY`
- [ ] `BOILERPLATE_PRIMARY.ENV` set (non-`local` forces migrate on boot)
- [ ] TLS in front (HSTS via `SECURITY_HSTS` if terminating here)
- [ ] Redis reachable; `REDIS.MODE` matches topology; TLS flags for managed Redis
- [ ] Rate limiting + idempotency enabled
- [ ] OTel endpoints + sample rate configured
- [ ] `AUTO_MIGRATE` decision made (on for containers, off if migrating via CI)
- [ ] Log aggregation + alerts on `/readyz`
- [ ] Backups for Postgres; persistence for Redis (AOF/RDB)

## License

MIT — see [LICENSE](LICENSE).
