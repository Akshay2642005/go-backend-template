package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/errs"
	"backend/internal/lib/propagation"
	"backend/internal/model"
	"backend/internal/repository"
	"backend/internal/service"
	apptesting "backend/internal/testing"
)

func setupPostHandler(t *testing.T) *PostHandler {
	t.Helper()
	db, cleanup := apptesting.SetupTestDB(t)
	t.Cleanup(cleanup)
	svc := service.NewPostService(repository.NewPostRepository(db.Pool))
	// Zero Handler is fine: the posts paths never touch h.server.
	return &PostHandler{postService: svc}
}

func requestWithUser(method, target, body string, userID string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if userID != "" {
		req = req.WithContext(propagation.WithPropagatedValues(
			req.Context(),
			&propagation.PropagatedValues{RequestID: "test-req", UserID: userID},
		))
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestPostHandlerList(t *testing.T) {
	h := setupPostHandler(t)
	ctx := propagation.WithPropagatedValues(context.Background(),
		&propagation.PropagatedValues{UserID: "user-1"})
	_, err := h.postService.Create(ctx, model.CreatePostRequest{Title: "one", Content: "c1"})
	require.NoError(t, err)
	_, err = h.postService.Create(ctx, model.CreatePostRequest{Title: "two", Content: "c2"})
	require.NoError(t, err)

	c, rec := requestWithUser(http.MethodGet, "/?page=1&limit=10", "", "")
	require.NoError(t, h.List(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.PaginatedResponse[model.PostResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Data, 2)
}

func TestPostHandlerCreate(t *testing.T) {
	h := setupPostHandler(t)

	c, rec := requestWithUser(http.MethodPost, "/",
		`{"title":"hello","content":"world"}`, "user-9")
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp model.PostResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "hello", resp.Title)
	assert.Equal(t, "user-9", resp.AuthorID)
}

func TestPostHandlerCreateValidationFails(t *testing.T) {
	h := setupPostHandler(t)

	// Title too short (min=3).
	c, _ := requestWithUser(http.MethodPost, "/",
		`{"title":"x","content":"world"}`, "user-9")
	err := h.Create(c)
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}

func TestPostHandlerGetByID(t *testing.T) {
	h := setupPostHandler(t)
	ctx := propagation.WithPropagatedValues(context.Background(),
		&propagation.PropagatedValues{UserID: "user-1"})
	post, err := h.postService.Create(ctx, model.CreatePostRequest{Title: "find me", Content: "c"})
	require.NoError(t, err)

	c, rec := requestWithUser(http.MethodGet, "/", "", "")
	c.SetParamNames("id")
	c.SetParamValues(post.ID.String())
	require.NoError(t, h.GetByID(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp model.PostResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, post.ID, resp.ID)
}

func TestPostHandlerGetByIDBadUUID(t *testing.T) {
	h := setupPostHandler(t)

	c, _ := requestWithUser(http.MethodGet, "/", "", "")
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")
	err := h.GetByID(c)
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}

func TestPostHandlerGetByIDMissing(t *testing.T) {
	h := setupPostHandler(t)

	c, _ := requestWithUser(http.MethodGet, "/", "", "")
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	err := h.GetByID(c)
	require.Error(t, err)
	httpErr, ok := err.(*errs.HTTPError)
	require.True(t, ok, "expected *errs.HTTPError, got %T", err)
	assert.Equal(t, http.StatusNotFound, httpErr.Status)
}

func TestPostHandlerDelete(t *testing.T) {
	h := setupPostHandler(t)
	ctx := propagation.WithPropagatedValues(context.Background(),
		&propagation.PropagatedValues{UserID: "user-1"})
	post, err := h.postService.Create(ctx, model.CreatePostRequest{Title: "doomed", Content: "c"})
	require.NoError(t, err)

	c, rec := requestWithUser(http.MethodDelete, "/", "", "user-1")
	c.SetParamNames("id")
	c.SetParamValues(post.ID.String())
	require.NoError(t, h.Delete(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
