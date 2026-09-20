# Go Boilerplate Backend

A production-ready Go backend service built with Echo framework, featuring clean architecture, comprehensive middleware, and modern DevOps practices.

## Architecture Overview

This backend follows clean architecture principles with clear separation of concerns:

```
backend/
├── cmd/go-boilerplate/        # Application entry point
├── internal/                  # Private application code
│   ├── cache/                # Redis cache control (operations, keys, metrics, fail-open)
│   ├── config/               # Configuration management
│   ├── database/             # Database connections and migrations
│   ├── handler/              # HTTP request handlers
│   ├── service/              # Business logic layer
│   ├── repository/           # Data access layer
│   ├── model/                # Domain models
│   ├── middleware/           # HTTP middleware (auth, cache control, rate limit, etc.)
│   ├── lib/                  # Shared libraries
│   └── validation/           # Request validation
├── static/                   # Static files (OpenAPI spec)
├── templates/                # Email templates
└── Taskfile.yml              # Task automation
```

## Features

### Core Framework
- **Echo v4**: High-performance, minimalist web framework
- **Clean Architecture**: Handlers → Services → Repositories → Models
- **Dependency Injection**: Constructor-based DI for testability

### Database
- **PostgreSQL**: Primary database with pgx/v5 driver
- **Migration System**: Tern for schema versioning with auto-migrate option
- **Connection Pooling**: Optimized for production workloads
- **Transaction Helpers**: `WithTx` for automatic commit/rollback with error handling
- **Read-Only Transactions**: Support for read-only transaction isolation
- **Querier Interface**: Abstracts pool vs transaction for testable repositories
- **Context Propagation**: Transactions injectable via `context.Context`

### Error Handling (RFC 7807)
- **Problem Details**: All errors conform to RFC 7807 with `type`, `title`, `detail`, `instance`
- **Content-Type**: `application/problem+json` for all error responses
- **Request Tracing**: Every error includes the request ID in `instance` field
- **Field-Level Errors**: Validation errors include per-field details
- **Database Error Mapping**: pgx errors automatically converted to human-readable problems

### Authentication & Security
- **Clerk Integration**: Modern authentication service
- **JWT Validation**: Secure token verification
- **Role-Based Access**: Configurable permission system
- **Rate Limiting**: Redis-backed sliding window with per-IP/per-user strategies
- **Security Headers**: XSS, CSRF, clickjacking, HSTS, CSP, and Referrer-Policy protection
- **Request Validation**: Structured input validation with `validator/v10` and RFC 7807 error responses

### Resilience
- **Circuit Breaker**: State machine (Closed → Open → Half-Open) preventing cascading failures when Redis or database is unhealthy
- **Per-Request Timeout**: Configurable context deadline per request (returns 504 on timeout)
- **Database Connection Retry**: Exponential backoff retry for initial database connection
- **Fail-Open**: Circuit breaker, cache, and rate limiter all fail open — never block requests

### Observability
- **OpenTelemetry**: Distributed tracing and metrics with OTLP/gRPC export (disabled by default)
- **Structured Logging**: JSON logs with Zerolog
- **Request Tracing**: Automatic span creation and context propagation for HTTP requests
- **Request Metrics**: `http.server.request.duration` histogram, `http.server.request.count` counter with method/status/route labels
- **Cache Metrics**: `cache.operation.count` with hit/miss status
- **Health Checks**: `/healthz` (liveness), `/readyz` (readiness), `/status` (full details)
- **Request/Response Body Logging**: Optional debug-level logging with 1 KB truncation (disabled by default)
- **Fail-Open**: Observability failures never block requests

### Rate Limiting
- **Redis-backed Sliding Window**: Distributed rate limiting across all instances
- **Per-User or Per-IP**: Configurable key strategy (by user ID or client IP)
- **Lua Script**: Atomic rate limit checks via Redis EVALSHA for performance
- **Fail-Open**: Redis failures allow requests through (never block)
- **Standard Headers**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`

### Idempotency
- **Redis-backed**: Idempotent request handling for POST/PUT/PATCH endpoints
- **Client-Provided Keys**: Clients send `Idempotency-Key` header
- **Response Caching**: Successful responses stored and replayed for duplicate requests
- **Scoped by User**: Keys are scoped to user + method + path for isolation
- **Configurable TTL**: Default 24h, adjustable per deployment

### Background Jobs
- **Asynq**: Redis-based distributed task queue
- **Priority Queues**: Critical, default, and low priority
- **Job Scheduling**: Cron-like task scheduling
- **Retry Logic**: Exponential backoff for failed jobs
- **Job Monitoring**: Real-time job status tracking
- **Context Propagation**: Request metadata (request ID, user ID, trace ID) automatically propagated from HTTP handlers to background jobs via payload envelope

### Cache Control
- **Enterprise Redis**: Single, cluster, and sentinel topology support via `redis.UniversalClient`
- **Cache-Aside Pattern**: `GetOrSet` with automatic origin loading and cache storage
- **Stampede Protection**: Singleflight deduplication of concurrent cache misses
- **Fail-Open**: Redis failures never block requests — falls through to origin
- **TTL Jitter**: Randomized expiration to prevent synchronized cache stampedes
- **HTTP Caching**: `CacheControl` middleware with ETag, `If-None-Match` (304), and `Cache-Control` headers
- **Key Namespacing**: Configurable prefix per environment with automatic SHA-256 hashing of long segments
- **Observability**: Atomic hit/miss/error counters, slow-op logging, structured zerolog metrics

### Email Service
- **Resend Integration**: Reliable email delivery
- **HTML Templates**: Beautiful transactional emails
- **Preview Mode**: Test emails in development

### API Documentation
- **OpenAPI 3.0**: Complete API specification
- **Scalar UI**: Interactive API explorer (served at `/docs`)

### CRUD Example (Posts)
- **Full Stack**: Handler → Service → Repository → Model pattern demonstrated with a Posts resource
- **Clerk User Integration**: `author_id` extracted from Clerk JWT (no foreign key to users table)
- **Pagination**: Configurable page/limit with total count
- **Status Filtering**: Draft/published/archived post states
- **Migration Example**: `002_posts.sql` shows the migration pattern

### Response Compression
- **Gzip**: Configurable gzip compression for HTTP responses (enabled by default)
- **Balanced**: Level 5 compression for optimal speed/ratio tradeoff

### Graceful Shutdown
- **Ordered Teardown**: HTTP drain → database → Redis cache → background jobs
- **Drain Period**: Configurable timeout for in-flight requests to complete (default 15s)
- **Shutdown Timeout**: Total deadline for the entire shutdown sequence (default 30s)
- **Structured Logging**: Each shutdown step logged for observability

## Getting Started

### Prerequisites
- Go 1.26+
- Docker (with the compose plugin) for Postgres/Redis
- Task (taskfile.dev)

### Installation

1. Install dependencies:
```bash
go mod download
```

2. Set up environment:
```bash
cp .env.example .env
# Configure your environment variables
```

3. Start the local database and redis (Postgres 16, Redis 8):
```bash
docker compose up -d
```

4. Run migrations:
```bash
task migrations:up
```

5. Start the server:
```bash
task run
```

## Configuration

Configuration is managed through environment variables with the `BOILERPLATE_` prefix:

```bash
# OpenTelemetry tracing (disabled by default)
BOILERPLATE_OBSERVABILITY.TRACING.ENABLED="false"
BOILERPLATE_OBSERVABILITY.TRACING.ENDPOINT="localhost:4317"
BOILERPLATE_OBSERVABILITY.TRACING.SAMPLE_RATE="1.0"

# Rate limiting (disabled by default)
BOILERPLATE_SERVER.RATE_LIMIT.ENABLED="false"
BOILERPLATE_SERVER.RATE_LIMIT.WINDOW="60s"
BOILERPLATE_SERVER.RATE_LIMIT.MAX="100"

# Idempotency (disabled by default)
BOILERPLATE_SERVER.IDEMPOTENCY.ENABLED="false"
BOILERPLATE_SERVER.IDEMPOTENCY.TTL="24h"

# Auto-migrate database on startup (disabled by default)
BOILERPLATE_DATABASE.AUTO_MIGRATE="false"

# Graceful shutdown
BOILERPLATE_SERVER.SHUTDOWN_TIMEOUT="30"
BOILERPLATE_SERVER.DRAIN_TIMEOUT="15"

# Compression (enabled by default)
BOILERPLATE_SERVER.COMPRESSION="true"

# Per-request timeout in seconds (0 = no timeout)
BOILERPLATE_SERVER.REQUEST_TIMEOUT="10"

# HSTS max-age in seconds (0 = disabled, 31536000 = 1 year)
BOILERPLATE_SERVER.SECURITY_HSTS="0"

# Database connection retry
BOILERPLATE_DATABASE.CONNECT_RETRIES="3"
BOILERPLATE_DATABASE.CONNECT_RETRY_DELAY="2"

# Request/response body logging (disabled by default, truncates at 1 KB)
BOILERPLATE_SERVER.REQUEST_BODY_LOG="false"
```

## Development

### Available Tasks

```bash
task help                    # Show all available tasks
task compose:up              # Start local postgres + redis
task compose:down            # Stop local postgres + redis (keeps volumes)
task compose:down-v          # Stop and delete volumes (full reset)
task run                     # Run the application
task test                    # Run tests
task migrations:new name=X   # Create new migration
task migrations:up           # Apply migrations
task tidy                    # Format and tidy dependencies
```

### Project Structure

#### Handlers (`internal/handler/`)
HTTP request handlers that:
- Parse and validate requests
- Call appropriate services
- Format responses
- Handle HTTP-specific concerns

#### Services (`internal/service/`)
Business logic layer that:
- Implements use cases
- Orchestrates operations
- Enforces business rules
- Handles transactions

#### Repositories (`internal/repository/`)
Data access layer that:
- Encapsulates database queries
- Provides data mapping
- Handles database-specific logic
- Supports multiple data sources

#### Models (`internal/model/`)
Domain entities that:
- Define core business objects
- Include validation rules
- Remain database-agnostic

#### Middleware (`internal/middleware/`)
Cross-cutting concerns:
- Authentication/Authorization
- Request logging
- Error handling
- Rate limiting
- CORS
- Security headers

### Testing

#### Unit Tests
```bash
go test ./...
```

Tests use testcontainers (a Postgres container is spun up per test package), so Docker must be running.

## Logging

Structured logging with Zerolog:

```go
log.Info().
    Str("user_id", userID).
    Str("action", "login").
    Msg("User logged in successfully")
```

Log levels:
- `debug`: Detailed debugging information
- `info`: General informational messages
- `warn`: Warning messages
- `error`: Error messages (forwarded to Sentry)
- `fatal`: Fatal errors that cause shutdown (forwarded to Sentry)

### Production Checklist

- [ ] Set production environment variables
- [ ] Enable SSL/TLS
- [ ] Configure production database
- [ ] Set up monitoring alerts (Sentry DSN)
- [ ] Configure log aggregation
- [ ] Enable rate limiting
- [ ] Set up backup strategy
- [ ] Configure auto-scaling
- [ ] Implement graceful shutdown
- [ ] Set up CI/CD pipeline

## Performance Optimization

### Database
- Connection pooling configured
- Prepared statements for frequent queries
- Indexes on commonly queried fields
- Query optimization with EXPLAIN ANALYZE

### Caching
- Redis-backed cache with single, cluster, and sentinel topology support
- Cache-aside pattern with singleflight stampede protection
- Fail-open: cache failures never break the request path
- TTL jitter to prevent synchronized cache stampedes
- HTTP response caching middleware with ETag and Cache-Control
- Atomic hit/miss/error metrics with slow-op logging
- Configurable key prefix namespacing per environment

### Concurrency
- Goroutine pools for parallel processing
- Context-based cancellation
- Proper mutex usage

## Security Best Practices

1. **Input Validation**: All inputs validated and sanitized
2. **SQL Injection**: Parameterized queries only
3. **XSS Protection**: Output encoding and CSP headers
4. **CSRF Protection**: Token-based protection
5. **Rate Limiting**: Per-IP and per-user limits
6. **Secrets Management**: Environment variables, never in code
7. **HTTPS Only**: Enforce TLS in production
8. **Dependency Scanning**: Regular vulnerability checks

## Contributing

1. Follow Go best practices and idioms
2. Write tests for new features
3. Update documentation
4. Run linters before committing
5. Keep commits atomic and well-described

## License

See the parent project's LICENSE file.