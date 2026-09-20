package repository

import "backend/internal/server"

type Repositories struct {
	Post *PostRepository
}

func NewRepositories(s *server.Server) *Repositories {
	return &Repositories{
		Post: NewPostRepository(s.DB.Pool),
	}
}
