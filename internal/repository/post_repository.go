package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/database"
	"backend/internal/model"
)

// PostRepository handles database operations for posts.
type PostRepository struct {
	pool *pgxpool.Pool
}

// NewPostRepository creates a new PostRepository.
func NewPostRepository(pool *pgxpool.Pool) *PostRepository {
	return &PostRepository{pool: pool}
}

// Create inserts a new post and returns the created record.
func (r *PostRepository) Create(ctx context.Context, post *model.Post) error {
	query := `
		INSERT INTO posts (id, title, content, status, author_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, query,
		post.ID, post.Title, post.Content, post.Status, post.AuthorID,
		post.CreatedAt, post.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert post: %w", err)
	}
	return nil
}

// GetByID retrieves a post by its UUID.
func (r *PostRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Post, error) {
	query := `
		SELECT id, title, content, status, author_id, created_at, updated_at
		FROM posts WHERE id = $1`

	post := &model.Post{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&post.ID, &post.Title, &post.Content, &post.Status, &post.AuthorID,
		&post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get post by id: %w", err)
	}
	return post, nil
}

// List returns a paginated list of posts, optionally filtered by status.
func (r *PostRepository) List(ctx context.Context, limit, offset int, status string) ([]model.Post, int, error) {
	var args []interface{}
	argIdx := 1

	// Count query
	countQuery := "SELECT COUNT(*) FROM posts"
	where := ""

	if status != "" {
		where = fmt.Sprintf(" WHERE status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	err := r.pool.QueryRow(ctx, countQuery+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	// Data query
	dataQuery := fmt.Sprintf(`
		SELECT id, title, content, status, author_id, created_at, updated_at
		FROM posts%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Content, &p.Status, &p.AuthorID,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}

	return posts, total, nil
}

// Update modifies an existing post. Only non-zero fields are updated.
func (r *PostRepository) Update(ctx context.Context, id uuid.UUID, title, content string, status model.PostStatus) error {
	query := `
		UPDATE posts
		SET title = $2, content = $3, status = $4, updated_at = now()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, title, content, status)
	if err != nil {
		return fmt.Errorf("update post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

// Delete removes a post by ID.
func (r *PostRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM posts WHERE id = $1"

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

// Querier returns the underlying pool as a Querier interface for use in transactions.
func (r *PostRepository) Querier() database.Querier {
	return r.pool
}
