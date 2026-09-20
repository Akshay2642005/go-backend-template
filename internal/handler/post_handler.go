package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"backend/internal/errs"
	"backend/internal/model"
	"backend/internal/server"
	"backend/internal/service"
)

// PostHandler handles HTTP requests for posts.
type PostHandler struct {
	Handler
	postService *service.PostService
}

// NewPostHandler creates a new PostHandler.
func NewPostHandler(s *server.Server, postService *service.PostService) *PostHandler {
	return &PostHandler{
		Handler:     NewHandler(s),
		postService: postService,
	}
}

// List returns a paginated list of posts.
func (h *PostHandler) List(c echo.Context) error {
	return Handle(h.Handler, func(_ echo.Context, req *model.ListPostsRequest) (interface{}, error) {
		posts, total, err := h.postService.List(c.Request().Context(), *req)
		if err != nil {
			return nil, err
		}

		page := req.Page
		if page <= 0 {
			page = 1
		}
		limit := req.Limit
		if limit <= 0 {
			limit = 20
		}

		// Convert to response
		data := make([]model.PostResponse, len(posts))
		for i, p := range posts {
			data[i] = model.PostFromModel(&p)
		}

		return model.PaginatedResponse[model.PostResponse]{
			Data:       data,
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: (total + limit - 1) / limit,
		}, nil
	}, 200, &model.ListPostsRequest{})(c)
}

// GetByID returns a single post by ID.
func (h *PostHandler) GetByID(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.NewBadRequestError("invalid post ID", false, nil, nil, nil)
	}

	post, err := h.postService.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(200, model.PostFromModel(post))
}

// Create creates a new post.
func (h *PostHandler) Create(c echo.Context) error {
	return Handle(h.Handler, func(_ echo.Context, req *model.CreatePostRequest) (interface{}, error) {
		post, err := h.postService.Create(c.Request().Context(), *req)
		if err != nil {
			return nil, err
		}

		return model.PostFromModel(post), nil
	}, 201, &model.CreatePostRequest{})(c)
}

// Update modifies an existing post.
func (h *PostHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.NewBadRequestError("invalid post ID", false, nil, nil, nil)
	}

	return Handle(h.Handler, func(_ echo.Context, req *model.UpdatePostRequest) (interface{}, error) {
		post, err := h.postService.Update(c.Request().Context(), id, *req)
		if err != nil {
			return nil, err
		}

		return model.PostFromModel(post), nil
	}, 200, &model.UpdatePostRequest{})(c)
}

// Delete removes a post.
func (h *PostHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.NewBadRequestError("invalid post ID", false, nil, nil, nil)
	}

	if err := h.postService.Delete(c.Request().Context(), id); err != nil {
		return err
	}

	return c.NoContent(204)
}
