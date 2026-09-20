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
		middleware.RequestID(),
		middlewares.Tracing.EnhanceTracing(),
		middlewares.ContextEnhancer.EnhanceContext(),
		middlewares.Metrics.Instrument(),
		middlewares.Global.RequestLogger(),
		middlewares.Global.Recover(),
	)

	// register system routes
	registerSystemRoutes(router, h)

	// register versioned routes with HTTP response caching and idempotency
	api := router.Group("/api/v1")
	api.Use(middlewares.CacheControl.Handle())
	api.Use(middlewares.Idempotency.Handle())

	return router
}
