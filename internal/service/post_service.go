package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/errs"
	"backend/internal/model"
	"backend/internal/repository"
)

// PostService handles business logic for posts.
type PostService struct {
	repo *repository.PostRepository
}

// NewPostService creates a new PostService.
func NewPostService(pool *pgxpool.Pool) *PostService {
	return &PostService{
		repo: repository.NewPostRepository(pool),
	}
}

// Create creates a new post. The authorID comes from the Clerk user context.
func (s *PostService) Create(ctx context.Context, req model.CreatePostRequest, authorID string) (*model.Post, error) {
	now := time.Now().UTC()
	status := model.PostStatusDraft
	if req.Status != "" {
		status = model.PostStatus(req.Status)
	}

	post := &model.Post{
		Base: model.Base{
			BaseWithId: model.BaseWithId{ID: uuid.New()},
			BaseWithCreatedAt: model.BaseWithCreatedAt{CreatedAt: now},
			BaseWithUpdatedAt: model.BaseWithUpdatedAt{UpdatedAt: now},
		},
		Title:    req.Title,
		Content:  req.Content,
		Status:   status,
		AuthorID: authorID,
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}
	return post, nil
}

// GetByID retrieves a post by ID. Returns 404 if not found.
func (s *PostService) GetByID(ctx context.Context, id uuid.UUID) (*model.Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errs.ProblemNotFound("Post")
	}
	return post, nil
}

// List returns a paginated list of posts.
func (s *PostService) List(ctx context.Context, req model.ListPostsRequest) ([]model.Post, int, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	return s.repo.List(ctx, limit, offset, req.Status)
}

// Update modifies an existing post.
func (s *PostService) Update(ctx context.Context, id uuid.UUID, req model.UpdatePostRequest) (*model.Post, error) {
	// Fetch existing post
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.ProblemNotFound("Post")
	}

	// Apply updates
	title := existing.Title
	if req.Title != "" {
		title = req.Title
	}
	content := existing.Content
	if req.Content != "" {
		content = req.Content
	}
	status := existing.Status
	if req.Status != "" {
		status = model.PostStatus(req.Status)
	}

	if err := s.repo.Update(ctx, id, title, content, status); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
}

// Delete removes a post by ID.
func (s *PostService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return errs.ProblemNotFound("Post")
	}
	return nil
}
