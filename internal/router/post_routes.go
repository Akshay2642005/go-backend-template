package router

import (
	"github.com/labstack/echo/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/handler"
	"backend/internal/middleware"
	"backend/internal/server"
)

func registerPostRoutes(api *echo.Group, s *server.Server, h *handler.Handlers, pool *pgxpool.Pool) {
	postHandler := handler.NewPostHandler(s, pool)

	posts := api.Group("/posts")
	posts.Use(middleware.NewAuthMiddleware(s).RequireAuth)

	posts.GET("", postHandler.List)
	posts.GET("/:id", postHandler.GetByID)
	posts.POST("", postHandler.Create)
	posts.PUT("/:id", postHandler.Update)
	posts.DELETE("/:id", postHandler.Delete)
}
