package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/errs"
	"backend/internal/lib/propagation"
	"backend/internal/model"
	"backend/internal/repository"
	apptesting "backend/internal/testing"
)

func setupPostService(t *testing.T) *PostService {
	t.Helper()
	db, cleanup := apptesting.SetupTestDB(t)
	t.Cleanup(cleanup)
	return NewPostService(repository.NewPostRepository(db.Pool))
}

func ctxWithUser(userID string) context.Context {
	return propagation.WithPropagatedValues(context.Background(), &propagation.PropagatedValues{
		RequestID: "test-req",
		UserID:    userID,
	})
}

func createAs(t *testing.T, svc *PostService, userID, title string) *model.Post {
	t.Helper()
	post, err := svc.Create(ctxWithUser(userID), model.CreatePostRequest{
		Title:   title,
		Content: "content for " + title,
	})
	require.NoError(t, err)
	return post
}

func TestPostServiceCreateSetsAuthor(t *testing.T) {
	svc := setupPostService(t)

	post := createAs(t, svc, "user-1", "my post")
	assert.Equal(t, "user-1", post.AuthorID)
	assert.Equal(t, model.PostStatusDraft, post.Status)
}

func TestPostServiceCreateAnonymousFallback(t *testing.T) {
	svc := setupPostService(t)

	post, err := svc.Create(context.Background(), model.CreatePostRequest{
		Title:   "no user",
		Content: "content",
	})
	require.NoError(t, err)
	assert.Equal(t, "anonymous", post.AuthorID)
}

func TestPostServiceUpdateOwnerOK(t *testing.T) {
	svc := setupPostService(t)
	post := createAs(t, svc, "user-1", "original")

	updated, err := svc.Update(ctxWithUser("user-1"), post.ID, model.UpdatePostRequest{
		Title: "edited",
	})
	require.NoError(t, err)
	assert.Equal(t, "edited", updated.Title)
}

func TestPostServiceUpdateStrangerForbidden(t *testing.T) {
	svc := setupPostService(t)
	post := createAs(t, svc, "user-1", "mine")

	_, err := svc.Update(ctxWithUser("user-2"), post.ID, model.UpdatePostRequest{
		Title: "hijacked",
	})
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusForbidden, httpErr.Status)
}

func TestPostServiceUpdateMissingNotFound(t *testing.T) {
	svc := setupPostService(t)

	_, err := svc.Update(ctxWithUser("user-1"), uuid.New(), model.UpdatePostRequest{
		Title: "ghost",
	})
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusNotFound, httpErr.Status)
}

func TestPostServiceDeleteOwnerOK(t *testing.T) {
	svc := setupPostService(t)
	post := createAs(t, svc, "user-1", "doomed")

	require.NoError(t, svc.Delete(ctxWithUser("user-1"), post.ID))

	_, err := svc.GetByID(context.Background(), post.ID)
	require.Error(t, err)
}

func TestPostServiceDeleteStrangerForbidden(t *testing.T) {
	svc := setupPostService(t)
	post := createAs(t, svc, "user-1", "not yours")

	err := svc.Delete(ctxWithUser("user-2"), post.ID)
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusForbidden, httpErr.Status)
}

func TestPostServiceDeleteMissingNotFound(t *testing.T) {
	svc := setupPostService(t)

	err := svc.Delete(ctxWithUser("user-1"), uuid.New())
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusNotFound, httpErr.Status)
}

func TestPostServiceListClampsLimit(t *testing.T) {
	svc := setupPostService(t)
	createAs(t, svc, "user-1", "a")
	createAs(t, svc, "user-1", "b")

	// A huge limit must not blow up — the service clamps to MaxListLimit.
	posts, total, err := svc.List(context.Background(), model.ListPostsRequest{
		Page:  1,
		Limit: 1000000,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, posts, 2)
}
