package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/model"
	apptesting "backend/internal/testing"
)

func setupPostRepo(t *testing.T) *PostRepository {
	t.Helper()
	db, cleanup := apptesting.SetupTestDB(t)
	t.Cleanup(cleanup)
	return NewPostRepository(db.Pool)
}

func samplePost(title, status string) *model.Post {
	now := time.Now().UTC()
	return &model.Post{
		Base: model.Base{
			BaseWithId:        model.BaseWithId{ID: uuid.New()},
			BaseWithCreatedAt: model.BaseWithCreatedAt{CreatedAt: now},
			BaseWithUpdatedAt: model.BaseWithUpdatedAt{UpdatedAt: now},
		},
		Title:    title,
		Content:  "content for " + title,
		Status:   model.PostStatus(status),
		AuthorID: "author-1",
	}
}

func TestPostRepositoryCreateAndGet(t *testing.T) {
	repo := setupPostRepo(t)
	ctx := context.Background()

	post := samplePost("hello world", "draft")
	require.NoError(t, repo.Create(ctx, post))

	got, err := repo.GetByID(ctx, post.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, post.ID, got.ID)
	assert.Equal(t, "hello world", got.Title)
	assert.Equal(t, model.PostStatusDraft, got.Status)
	assert.Equal(t, "author-1", got.AuthorID)
}

func TestPostRepositoryGetMissing(t *testing.T) {
	repo := setupPostRepo(t)

	got, err := repo.GetByID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPostRepositoryList(t *testing.T) {
	repo := setupPostRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, samplePost("one", "published")))
	require.NoError(t, repo.Create(ctx, samplePost("two", "published")))
	require.NoError(t, repo.Create(ctx, samplePost("three", "draft")))

	posts, total, err := repo.List(ctx, 10, 0, "")
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, posts, 3)

	// Status filter
	posts, total, err = repo.List(ctx, 10, 0, "draft")
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, posts, 1)
	assert.Equal(t, "three", posts[0].Title)

	// Pagination
	posts, total, err = repo.List(ctx, 2, 0, "")
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, posts, 2)
}

func TestPostRepositoryUpdate(t *testing.T) {
	repo := setupPostRepo(t)
	ctx := context.Background()

	post := samplePost("before", "draft")
	require.NoError(t, repo.Create(ctx, post))

	require.NoError(t, repo.Update(ctx, post.ID, "after", "new content", model.PostStatusPublished))

	got, err := repo.GetByID(ctx, post.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "after", got.Title)
	assert.Equal(t, "new content", got.Content)
	assert.Equal(t, model.PostStatusPublished, got.Status)
}

func TestPostRepositoryDelete(t *testing.T) {
	repo := setupPostRepo(t)
	ctx := context.Background()

	post := samplePost("bye", "draft")
	require.NoError(t, repo.Create(ctx, post))

	require.NoError(t, repo.Delete(ctx, post.ID))

	got, err := repo.GetByID(ctx, post.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}
