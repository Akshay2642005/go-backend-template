# Go Backend Template

This is a starting point for building production Go APIs without reinventing
the same plumbing every time. It gives you a layered codebase with real
authentication, a database, caching, background jobs, and observability —
plus a fully working Posts feature you can read, run, and copy when you
build your own resources.

Everything is wired up and tested. Clone it, configure a few environment
variables, and you have an API you can deploy.

## What's Inside

The HTTP layer runs on **Echo**, which is fast and unobtrusive — it stays
out of your way once routes are registered. Data lives in **Postgres**,
accessed through `pgx` (the best Postgres driver in the Go ecosystem), with
schema changes managed by **tern** migrations. **Redis** does double duty:
it's the cache and the backbone of the background job queue (**asynq**).

Authentication is handled by **Clerk**, so you never store passwords or
manage sessions yourself — you validate a JWT and trust the user ID inside
it. Outgoing mail goes through **Resend**.

For configuration, the app reads environment variables through **koanf**,
which maps them onto typed structs and validates them at startup. If you
misconfigure something, the app refuses to boot and tells you exactly which
variable is wrong — much better than a nil pointer three requests in.

Logging is structured JSON via **zerolog**, and distributed tracing works
through **OpenTelemetry** if you point it at a collector. Both are designed
to degrade gracefully: if tracing is off or Redis is down, requests still
succeed. Nothing in the observability or caching path can take down your
API.

Errors follow **RFC 7807 Problem Details**, so every failure — validation,
not-found, database constraint — comes back in the same predictable shape
with a request ID you can grep for in the logs.

Tests spin up real Postgres and Redis containers with **testcontainers**,
so you're testing against the actual infrastructure instead of mocks that
drift from reality.

## Getting Started

You'll need Go 1.26 or newer, Docker, and
[Task](https://taskfile.dev) (a simpler alternative to Make).

First, grab the dependencies and set up your environment:

```bash
go mod download
cp .env.example .env
```

Open `.env` and look through it. For local development most defaults just
work, but there's one thing you must change: `BOILERPLATE_AUTH.SECRET_KEY`
needs a real Clerk secret key. Without it, every authenticated endpoint
will reject you. Grab one from the
[Clerk dashboard](https://dashboard.clerk.com) — a test-mode key is fine
for local work.

Next, start Postgres and Redis, run the migrations, and boot the server:

```bash
task compose:up      # starts Postgres 16 + Redis 8 in Docker
task migrations:up   # creates the tables
task run             # starts the API on :8080
```

Visit `http://localhost:8080/docs` and you'll see the interactive API
explorer. `/healthz` should return 200 — that means you're up.

A few other commands you'll use daily:

```bash
task test                          # run the full test suite (needs Docker)
task migrations:new name=add_x     # scaffold a new migration
task tidy                          # format code, tidy modules, verify deps
task compose:down                  # stop infra, keep data
task compose:down-v                # stop infra and wipe data (fresh start)
```

## How the Code Is Organized

Everything the app does lives under `internal/`, which means Go won't let
outside modules import it — this is your private codebase. Here's the tour:

- `cmd/go-boilerplate/main.go` — the entry point. It loads config, builds
  the server, wires dependencies, starts HTTP, and handles shutdown. There
  is no business logic here, only wiring.
- `handler/` — thin HTTP adapters. A handler takes a request, calls a
  service, and returns a response. It knows about HTTP status codes and
  nothing else.
- `service/` — where your business rules live. "Can this user publish this
  post?" is answered here. Services never import Echo or touch HTTP.
- `repository/` — SQL and nothing but SQL. Each repository wraps queries
  for one resource and maps rows onto models.
- `model/` — domain structs plus the request/response shapes your API
  speaks. Validation rules live alongside the types they protect.
- `router/` — route registration. `router.go` builds the Echo instance and
  the middleware chain; `*_routes.go` files bind URLs to handlers.
- `middleware/` — everything that runs around your handlers: auth, rate
  limiting, request IDs, logging, tracing, caching headers.
- `cache/` — typed Redis helpers with stampede protection and metrics.
- `lib/` — shared libraries that don't belong to a layer: the circuit
  breaker, the email client, the job queue, and context propagation.
- `errs/` — error types and constructors. Every error your API returns is
  built here.
- `config/`, `database/`, `server/`, `logger/`, `otel/`, `validation/` —
  infrastructure. You'll configure these more than you'll change them.
- `testing/` — helpers for tests: throwaway databases, transaction
  wrappers, assertions.
- `static/openapi.json` — the API contract. The `/docs` page renders it.
- `templates/emails/` — HTML email templates.

## How a Request Travels Through the System

Say a client sends `POST /api/v1/posts` with a Clerk JWT. Here's what
happens, in order:

1. **Echo routes the request** to the posts group, which requires
   authentication. The Clerk middleware validates the token and records
   the user ID.
2. **The middleware chain runs.** Rate limiting checks the Redis sliding
   window. A request ID is assigned. The `ContextEnhancer` bundles the
   request ID and user ID into a `PropagatedValues` struct and tucks it
   into the request's `context.Context`, alongside a logger already
   tagged with who is asking and what they're asking for.
3. **The handler takes over.** The `Handle` helper binds the JSON body
   onto a `CreatePostRequest`, validates it, and — only if everything
   checks out — calls your closure with `(ctx, req)`. If validation
   fails, the client gets a 400 with per-field details and your closure
   never runs.
4. **The service applies business logic.** It pulls the user ID out of
   the context (`propagation.UserIDFrom(ctx)`), builds the post, and
   hands it to the repository. It has no idea Echo exists.
5. **The repository runs SQL** and returns the row. The handler converts
   it to a response shape and `Handle` serializes it with status 201.

```
Client ──► Echo ──► middleware ──► ContextEnhancer ──► Handler ──► Service ──► Repository ──► Postgres
                              (user ID into ctx)       (ctx out)    (ctx in)      (ctx in)
```

The key idea: `echo.Context` never travels past the handler. Everything
below it speaks plain `context.Context`, which is why services stay
testable and reusable from background jobs.

## How Dependencies Are Wired

There is no DI framework — just constructors, called once in `main.go`
from the top down:

```go
repos    := repository.NewRepositories(srv)      // repos.Post (from s.DB.Pool)
services := service.NewServices(srv, repos)      // services.Post (from repos.Post)
handlers := handler.NewHandlers(srv, services)   // handlers.Post (from services.Post)
r        := router.NewRouter(srv, handlers)
```

Each layer receives what it needs from the layer above it. Repositories
get the database pool. Services get repositories. Handlers get services.
Nobody reaches sideways or constructs their own dependencies — if you
catch yourself calling `NewPostService(pool)` inside a handler, something
has gone wrong.

When you add a new resource, you follow the same rhythm: add a field to
`Repositories`, `Services`, and `Handlers`, and wire each constructor to
the previous layer. The compiler will guide you — miss a step and
something won't build.

## Where the User ID Comes From

This deserves its own section because it surprises people. Handlers never
extract the user ID. Services never import Clerk or Echo. Instead:

1. The Clerk middleware validates the JWT and stores the user ID on the
   Echo context (`c.Set("user_id", ...)`).
2. The `ContextEnhancer` — which runs on every request — copies that ID
   into `PropagatedValues` inside the standard `context.Context`.
3. Services read it with `propagation.UserIDFrom(ctx)`. If nobody is
   logged in (or the value wasn't set), they get an empty string and
   decide what that means — for posts, it means `"anonymous"`.

Because the user ID travels in `context.Context` rather than Echo state,
it survives the trip into background jobs too. When you enqueue work with
`job.Envelope(ctx, payload)`, the request ID, user ID, and trace IDs are
serialized into the task. When the worker picks it up,
`job.ExtractMetadata` puts them back. A job that creates a post records
the same author as the HTTP request that triggered it, with no extra code.

This is safe under load. Every request builds a fresh context and a fresh
`PropagatedValues` struct — `context.WithValue` never mutates, it wraps.
Ten thousand concurrent requests means ten thousand isolated chains with
nothing shared between them. The service, repository, and connection pool
are all stateless with respect to the request; they only ever read what
was handed to them.

## Writing Handlers

A handler is one function per operation. You describe what a valid request
looks like, what to do with it, and what status to return — `Handle` takes
care of binding, validation, logging, and serialization:

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

Notice the closure receives `ctx`, not Echo's context. If you need
Echo-specific things — a path parameter, a raw JSON response, an empty
204 — you skip the wrapper and use Echo directly, the way `GetByID` and
`Delete` do. Both styles live side by side; pick whichever fits the
endpoint.

There are three wrappers for three kinds of responses: `Handle` returns a
JSON body, `HandleNoContent` returns an empty 204, and `HandleFile`
streams a download. Validation failures, panics in logging, and unexpected
errors are all handled before your code ever sees them — or after it
returns them, in the case of errors.

## What Runs Around Your Handlers

Middleware executes in a fixed order, defined in `router.go`. Each piece
does one job:

- **Rate limiting** checks a Redis-backed sliding window before anything
  expensive happens. If Redis is unreachable, requests pass through
  rather than failing — limiting must never become an outage.
- **Security headers, CORS, and request timeouts** set the baseline: XSS
  and clickjacking protection, allowed origins, and a deadline after
  which slow requests get a 504.
- **Request ID, tracing, and context enhancement** give every request an
  identity. The request ID shows up in logs, error responses, and traces,
  so you can follow one request across all three.
- **Metrics and request logging** record what happened: how long it took,
  what status went back, which route served it.
- **Compression and body logging** are opt-in. Gzip is on by default;
  request/response body logging exists for debugging and stays off in
  production (it truncates at 1 KB when enabled).

On top of the globals, the `/api/v1` group adds HTTP caching semantics
(ETags, `304 Not Modified`) and idempotency: if a client retries a POST
with the same `Idempotency-Key`, it gets the original response replayed
instead of a duplicate side effect. And any route group can require
authentication with a single line — that's how `/posts` is protected.

## When Things Go Wrong

Every error your API returns looks the same:

```json
{
  "type": "https://api.example.com/problems/validation",
  "title": "Validation failed",
  "status": 400,
  "detail": "title is required",
  "instance": "/api/v1/posts (req: 01J...)",
  "errors": [{ "field": "title", "error": "is required" }]
}
```

This is RFC 7807, and clients can rely on it: `status` matches the HTTP
code, `instance` always contains the request ID, and validation failures
list each bad field. The constructors live in `internal/errs` —
`ProblemNotFound`, `ProblemValidation`, `NewBadRequestError` — and
database errors are translated into plain language by `internal/sqlerr`,
so clients never see raw Postgres messages.

## Configuration

All settings come from the environment. Variable names start with
`BOILERPLATE_` and use dots to mirror the config structs, so
`BOILERPLATE_SERVER.PORT` fills `Config.Server.Port`. On startup the app
validates everything and refuses to boot with a clear message if
something is missing or malformed. `.env.example` documents every
variable; copy it to `.env` and adjust.

A few things worth knowing about specific areas:

**Server.** Ports, timeouts, CORS origins, and the graceful-shutdown
budgets live here. Rate limiting and idempotency each have their own
subsection and are off by default in local development — flip
`RATE_LIMIT.ENABLED` when you're ready to exercise them.

**Database.** Connection details plus pool sizing. `AUTO_MIGRATE` runs
migrations on boot, which is handy in containers but something you'll
want to think about deliberately in production. The app builds its own
connection string from these fields; `BOILERPLATE_DB_DSN` exists only for
the `tern` CLI used by `task migrations:up`.

**Auth.** Just the Clerk secret key. This is the one value that has no
usable default — the placeholder `"secret"` in the example file will boot
but reject every authenticated request. Use a test key locally and a live
key in production.

**Redis.** Address, topology mode (`single`, `cluster`, or `sentinel`),
pool sizes, timeouts, and retry behavior, plus TLS flags you'll need for
managed offerings like Upstash or ElastiCache. Anything you leave unset
falls back to sensible defaults, so local development needs almost
nothing here.

**Integrations.** Third-party keys — currently just Resend for email.

**Observability.** Log level and format, OTel tracing and metrics
endpoints (both disabled by default), and which checks the readiness
probe runs. The service name and environment are filled in automatically.

## Data, Caching, and Jobs

**Database access** goes through repositories that take either the pool
or a transaction — both satisfy the same `Querier` interface, so the same
repository code works inside and outside a transaction. When you need
atomicity across multiple writes, `database.WithTx` runs your function
inside a transaction and commits only if it returns nil. Migrations are
plain SQL files applied in order; `task migrations:new` scaffolds the
up/down pair for you.

**Redis** is wrapped in typed helpers (`Get`, `Set`, `GetOrSet`,
`Increment`, ...) that handle serialization, key namespacing, and
metrics. `GetOrSet` implements the cache-aside pattern with singleflight
deduplication, so a hundred simultaneous misses for the same key produce
one origin call instead of a hundred. Expirations get random jitter so
keys don't stampede at the same second. And if Redis goes away, a circuit
breaker trips and everything falls through to the origin — slower, but
working.

**Background jobs** run on asynq with critical, default, and low queues
plus cron scheduling. The welcome-email flow shows the whole lifecycle:
enqueue with context via `job.Envelope`, process on a worker with
`job.ExtractMetadata`, send through Resend. Add your own task types the
same way.

## Testing

Run `task test` and every package's tests execute against real
infrastructure — Postgres and Redis containers that the test helpers
create, migrate, and tear down automatically. Docker needs to be running;
there's no other setup.

The helpers in `internal/testing` do the heavy lifting. `SetupTestDB`
gives you a fresh database with migrations applied. Transaction wrappers
let each test case run inside a transaction that rolls back afterward, so
tests never leak data into each other. Middleware and handler tests use
`httptest` with a real Echo instance rather than mocks.

When you write tests, follow the existing shape: a `*_test.go` file next
to the code, table-driven cases with `testify/require`, a real database
for anything touching storage, and no dependence on `.env` or shared
state. If your test needs the database, take it from `SetupTestDB`; if it
doesn't, don't start one.

## The API

The contract lives in `static/openapi.json` and renders interactively at
`/docs`. Protected routes expect `Authorization: Bearer <clerk-jwt>`.

| Method | Path | Auth | What it does |
|---|---|---|---|
| GET | `/healthz` | — | Liveness: is the process alive? |
| GET | `/readyz` | — | Readiness: can it serve? checks DB + Redis |
| GET | `/status` | — | Legacy health details |
| GET | `/docs` | — | Interactive API explorer |
| GET | `/api/v1/posts` | Clerk | List posts (`page`, `limit`, `status` filter) |
| POST | `/api/v1/posts` | Clerk | Create a post (send `Idempotency-Key` to dedupe retries) |
| GET | `/api/v1/posts/:id` | Clerk | Fetch one post |
| PUT | `/api/v1/posts/:id` | Clerk | Update a post |
| DELETE | `/api/v1/posts/:id` | Clerk | Delete a post (empty 204) |

## Adding Your Own Resource

The fastest way to learn the template is to add something. Say you want
a `comments` resource. You'll touch six files, each in the layer it
belongs to, then wire three constructors:

Start with the **model** — the struct, what create/update requests look
like, and what responses go back. Put validation next to the types: struct
tags for the simple rules (`required`, `min`), a `Validate()` method for
anything cross-field.

Then the **migration**. Run `task migrations:new name=comments`, write
the `CREATE TABLE` in the up section and the `DROP TABLE` in the down
section, and apply it with `task migrations:up`. Look at `002_posts.sql`
if you want an example of the file format.

The **repository** is pure SQL — a `Create`, a `GetByID`, a `List`, an
`Update`, a `Delete`, each taking `context.Context` first. Register it by
adding a `Comment` field to the `Repositories` struct and constructing it
in `NewRepositories` from the pool.

The **service** holds anything the database shouldn't decide: defaults,
state transitions, permission checks. It takes your repository in its
constructor. Same dance — field on `Services`, line in `NewServices`.

The **handler** is the thinnest piece: one function per endpoint calling
`Handle` with a closure that calls the service. Field on `Handlers`, line
in `NewHandlers`, and you're nearly there.

Finally, **routes**: a small `comment_routes.go` that creates the group,
attaches `RequireAuth`, and binds methods to handler functions. Register
it from `router.go`, add the paths to `static/openapi.json`, and your
resource is live with auth, validation, logging, tracing, rate limiting,
and error handling — all inherited, none reimplemented.

## Going to Production

When you're ready to deploy, work through these:

- Swap in the real Clerk and Resend keys — the example values boot but
  don't do anything useful.
- Set `BOILERPLATE_PRIMARY.ENV` to your environment name. Anything other
  than `local` makes the app migrate on boot, which is what you want if
  nothing else runs migrations.
- Decide who runs migrations: the app (`AUTO_MIGRATE=true`, simplest in
  containers) or your pipeline (`task migrations:up` in CI, more
  control). Pick one, not both.
- Point Redis config at your real topology — mode, address, credentials,
  and TLS for managed providers.
- Turn on rate limiting and idempotency. They default off because they're
  noise locally; in production they're protection.
- Configure the OTel endpoints and sample rate, and point your log
  shipper at stdout. Alert on `/readyz`, not just the process.
- Terminate TLS in front of the app (or set `SECURITY_HSTS` if the app
  terminates it). Keep secrets in your platform's secret store, never in
  the image.
- Make sure Postgres is backed up and Redis persists (AOF or RDB). The
  compose file is for development — production infrastructure is yours
  to provision.

On shutdown the app drains in-flight HTTP requests first, then closes the
database pool, Redis, and the job server in that order. `DRAIN_TIMEOUT`
bounds the drain, `SHUTDOWN_TIMEOUT` bounds everything. Send SIGTERM and
it does the right thing.

## License

MIT — see [LICENSE](LICENSE).
