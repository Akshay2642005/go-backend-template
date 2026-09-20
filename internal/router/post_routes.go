package router

import (
	"github.com/labstack/echo/v4"

	"backend/internal/handler"
	"backend/internal/middleware"
	"backend/internal/server"
)

func registerPostRoutes(api *echo.Group, s *server.Server, h *handler.Handlers) {
	posts := api.Group("/posts")
	posts.Use(middleware.NewAuthMiddleware(s).RequireAuth)

	posts.GET("", h.Post.List)
	posts.GET("/:id", h.Post.GetByID)
	posts.POST("", h.Post.Create)
	posts.PUT("/:id", h.Post.Update)
	posts.DELETE("/:id", h.Post.Delete)
}
