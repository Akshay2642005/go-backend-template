package handler

import (
	"backend/internal/server"
	"backend/internal/service"
)

type Handlers struct {
	Health *HealthHandler
	Post   *PostHandler
}

func NewHandlers(s *server.Server, services *service.Services) *Handlers {
	return &Handlers{
		Health: NewHealthHandler(s),
		Post:   NewPostHandler(s, services.Post),
	}
}
