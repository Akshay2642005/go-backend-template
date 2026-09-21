# Architecture Documentation

## System Overview

This Go backend template follows a clean, layered architecture with clear separation of concerns. The system is designed to be production-ready with built-in authentication, database management, caching, background jobs, and observability.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                             │
│                   (Web, Mobile, External APIs)                   │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP/REST
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                      HTTP Layer (Echo)                           │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   Middleware Stack                          │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │  │
│  │  │ Rate Limit   │→ │ Auth/Clerk    │→ │ Request ID   │    │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘    │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │  │
│  │  │ Tracing      │→ │ Context Enh. │→ │ Logging      │    │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘    │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Handler Layer                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ PostHandler  │  │ UserHandler  │  │ HealthHandler│          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Service Layer (Business Logic)                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ PostService  │  │ UserService  │  │ AuthService   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Repository Layer (Data Access)                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │PostRepository│  │UserRepository│  │ (Add more)   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Infrastructure Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Postgres   │  │    Redis     │  │  Job Queue   │          │
│  │   (pgx)      │  │  (go-redis)  │  │  (asynq)     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Clerk      │  │   Resend     │  │ OpenTelemetry│          │
│  │   (Auth)     │  │   (Email)    │  │  (Otel)      │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Request Flow

A typical request flows through the system as follows:

1. **HTTP Request** → Echo router
2. **Middleware Chain** → Rate limiting, authentication, context enhancement
3. **Handler** → Request validation, calls service
4. **Service** → Business logic, calls repository
5. **Repository** → Database operations
6. **Response** → Flows back through layers with proper error handling

## Key Design Principles

### 1. Layered Architecture
- Each layer has a single responsibility
- Dependencies flow downward (higher layers depend on lower layers)
- No circular dependencies between layers

### 2. Context Propagation
- User ID, request ID, and trace IDs travel via `context.Context`
- Services and repositories remain HTTP-agnostic
- Background jobs inherit request context automatically

### 3. Error Handling
- All errors follow RFC 7807 Problem Details format
- Consistent error structure across all endpoints
- Request IDs in all error responses for debugging

### 4. Dependency Injection
- Manual DI via constructors (no framework)
- Clear dependency graph in `main.go`
- Easy to test with mock implementations

### 5. Database Access
- Repository pattern with Querier interface
- Support for both pool and transaction contexts
- Parameterized queries (no SQL injection risk)

## Directory Structure

```
internal/
├── handler/          # HTTP request handlers
├── service/          # Business logic
├── repository/       # Data access layer
├── model/            # Domain models and DTOs
├── middleware/       # HTTP middleware
├── router/           # Route registration
├── config/           # Configuration management
├── database/         # Database setup and migrations
├── cache/            # Redis caching
├── lib/              # Shared libraries
├── errs/             # Error types and constructors
├── validation/       # Request validation
├── logger/           # Structured logging
├── otel/             # OpenTelemetry setup
├── server/           # Server instance management
└── testing/          # Test utilities
```

## Security Features

1. **Authentication**: Clerk JWT validation via middleware
2. **Input Sanitization**: HTML sanitization, string cleaning
3. **SQL Injection Prevention**: Parameterized queries
4. **Rate Limiting**: Redis-backed sliding window
5. **CORS**: Configurable origin restrictions
6. **Security Headers**: XSS, clickjacking protection

## Performance Optimizations

1. **Database Indexing**: Composite indexes for common query patterns
2. **Caching**: Redis cache-aside pattern with stampede protection
3. **Connection Pooling**: Configurable Postgres connection pools
4. **Compression**: Gzip compression for responses
5. **Query Optimization**: Efficient SQL with proper indexing

## Observability

1. **Structured Logging**: JSON logs via zerolog
2. **Distributed Tracing**: OpenTelemetry integration
3. **Metrics**: Prometheus-compatible metrics
4. **Request Tracking**: Unique request IDs per request
5. **Health Checks**: Liveness and readiness probes

## Scalability Considerations

1. **Stateless Services**: Easy horizontal scaling
2. **Database Pooling**: Efficient connection management
3. **Caching Layer**: Reduces database load
4. **Background Jobs**: Asynchronous task processing
5. **Graceful Shutdown**: Proper cleanup on termination

## Testing Strategy

1. **Integration Tests**: Real Postgres/Redis via testcontainers
2. **Unit Tests**: Layer-specific testing with mocks
3. **Transaction Rollback**: Test isolation
4. **Table-Driven Tests**: Comprehensive test coverage
5. **HTTP Tests**: Full request/response testing

## External Dependencies

- **Echo**: HTTP framework
- **pgx**: Postgres driver
- **go-redis**: Redis client
- **asynq**: Background job queue
- **Clerk**: Authentication
- **Resend**: Email service
- **koanf**: Configuration management
- **zerolog**: Structured logging
- **OpenTelemetry**: Observability
