package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// PostStatus represents the lifecycle state of a post.
type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
	PostStatusArchived  PostStatus = "archived"
)

// Post represents a blog post or article.
type Post struct {
	Base
	Title    string     `json:"title" db:"title"`
	Content  string     `json:"content" db:"content"`
	Status   PostStatus `json:"status" db:"status"`
	AuthorID string     `json:"author_id" db:"author_id"` // Clerk user ID
}

// CreatePostRequest is the request body for creating a post.
type CreatePostRequest struct {
	Title   string `json:"title" validate:"required,min=3,max=200"`
	Content string `json:"content" validate:"required"`
	Status  string `json:"status" validate:"omitempty,oneof=draft published archived"`
}

func (r CreatePostRequest) Validate() error {
	// Cross-field validation
	if r.Status == "published" && len(r.Content) < 50 {
		return errors.New("published posts must have at least 50 characters of content")
	}
	return nil
}

// UpdatePostRequest is the request body for updating a post.
type UpdatePostRequest struct {
	Title   string `json:"title" validate:"omitempty,min=3,max=200"`
	Content string `json:"content" validate:"omitempty"`
	Status  string `json:"status" validate:"omitempty,oneof=draft published archived"`
}

func (r UpdatePostRequest) Validate() error {
	// Cross-field validation
	if r.Status == "published" && r.Content != "" && len(r.Content) < 50 {
		return errors.New("published posts must have at least 50 characters of content")
	}
	return nil
}

// MaxListLimit caps page size for list endpoints. The request validation
// tag enforces the same bound for HTTP callers; the service clamps
// defensively so direct callers (jobs, future code) can't dump the table.
const MaxListLimit = 100

// ListPostsRequest is the request query parameters for listing posts.
type ListPostsRequest struct {
	Page   int    `query:"page" validate:"omitempty,min=1"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Status string `query:"status" validate:"omitempty,oneof=draft published archived"`
}

func (r ListPostsRequest) Validate() error {
	// Cross-field validation
	if r.Page > 0 && r.Limit == 0 {
		return errors.New("limit is required when page is specified")
	}
	return nil
}

// PostResponse is the response body for a single post.
type PostResponse struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Status    PostStatus `json:"status"`
	AuthorID  string     `json:"author_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func PostFromModel(p *Post) PostResponse {
	return PostResponse{
		ID:        p.ID,
		Title:     p.Title,
		Content:   p.Content,
		Status:    p.Status,
		AuthorID:  p.AuthorID,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
