package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"backend/internal/errs"
	"backend/internal/lib/propagation"
	"backend/internal/model"
	"backend/internal/repository"
)

// PostService handles business logic for posts.
type PostService struct {
	repo *repository.PostRepository
}

// NewPostService creates a new PostService.
func NewPostService(repo *repository.PostRepository) *PostService {
	return &PostService{
		repo: repo,
	}
}

// Create creates a new post. The authorID is resolved from propagated
// context values (injected by ContextEnhancer), falling back to "anonymous".
func (s *PostService) Create(ctx context.Context, req model.CreatePostRequest) (*model.Post, error) {
	authorID := propagation.UserIDFrom(ctx)
	if authorID == "" {
		authorID = "anonymous"
	}

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
	if limit > model.MaxListLimit {
		limit = model.MaxListLimit
	}
	offset := (page - 1) * limit

	return s.repo.List(ctx, limit, offset, req.Status)
}

// authorizePostAuthor ensures the caller owns the post. An empty user ID
// never matches, so unauthenticated callers are denied by default.
func authorizePostAuthor(ctx context.Context, post *model.Post) error {
	userID := propagation.UserIDFrom(ctx)
	if userID == "" || userID != post.AuthorID {
		return errs.NewForbiddenError("you do not have permission to modify this post", false)
	}
	return nil
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
	if err := authorizePostAuthor(ctx, existing); err != nil {
		return nil, err
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

// Delete removes a post by ID. Returns 404 if the post does not exist.
// Repository errors propagate so sqlerr maps infrastructure failures
// to 5xx problems instead of masking them as 404s.
func (s *PostService) Delete(ctx context.Context, id uuid.UUID) error {
	post, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := authorizePostAuthor(ctx, post); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
