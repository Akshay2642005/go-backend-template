package service

import (
	"backend/internal/lib/job"
	"backend/internal/repository"
	"backend/internal/server"
)

type Services struct {
	Auth *AuthService
	Post *PostService
	Job  *job.JobService
}

func NewServices(s *server.Server, repos *repository.Repositories) (*Services, error) {
	authService := NewAuthService(s)

	return &Services{
		Job:  s.Job,
		Auth: authService,
		Post: NewPostService(repos.Post),
	}, nil
}
