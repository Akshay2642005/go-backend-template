package router

import (
	"github.com/labstack/echo/v4"

	"backend/internal/handler"
	"backend/internal/middleware"
	"backend/internal/server"
	"backend/internal/service"
)

func NewRouter(s *server.Server, h *handler.Handlers, services *service.Services) *echo.Echo {
	middlewares := middleware.NewMiddlewares(s)

	router := echo.New()

	router.HTTPErrorHandler = middlewares.Global.GlobalErrorHandler

	// global middlewares
	router.Use(
		middlewares.RateLimit.Handle(),
		middlewares.Global.CORS(),
		middlewares.Global.Secure(),
		middlewares.RequestTimeout.Handle(),
		middleware.RequestID(),
		middlewares.Tracing.EnhanceTracing(),
		middlewares.ContextEnhancer.EnhanceContext(),
		middlewares.Metrics.Instrument(),
		middlewares.Global.RequestLogger(),
		middlewares.Global.Recover(),
	)

	// compression (after response writing, before send)
	if s.Config.Server.Compression {
		router.Use(middlewares.Global.Compression())
	}

	// request/response body logging (optional, controlled by config)
	bodyLogger := middleware.NewBodyLogger(middleware.BodyLoggerConfig{
		Enabled: s.Config.Server.RequestBodyLog,
	})
	router.Use(bodyLogger.Handle())

	// register system routes
	registerSystemRoutes(router, h)

	// register versioned routes with HTTP response caching and idempotency
	api := router.Group("/api/v1")
	api.Use(middlewares.CacheControl.Handle())
	api.Use(middlewares.Idempotency.Handle())

	// register post routes (example CRUD)
	registerPostRoutes(api, s, h)

	return router
}
