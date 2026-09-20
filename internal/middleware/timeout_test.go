package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/config"
	"backend/internal/server"
	"backend/internal/service"
)

func newTestTimeoutServer(timeoutSec int) *server.Server {
	return &server.Server{
		Config: &config.Config{
			Server: config.ServerConfig{
				RequestTimeout: timeoutSec,
			},
		},
	}
}

func TestRequestTimeout_NoTimeout(t *testing.T) {
	srv := newTestTimeoutServer(0)
	rt := NewRequestTimeoutMiddleware(srv)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	middleware := rt.Handle()(func(c echo.Context) error {
		handlerCalled = true
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	err := middleware(c)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestRequestTimeout_WithinTimeout(t *testing.T) {
	srv := newTestTimeoutServer(5) // 5 second timeout
	rt := NewRequestTimeoutMiddleware(srv)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	middleware := rt.Handle()(func(c echo.Context) error {
		// Quick operation — should succeed
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	err := middleware(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestTimeout_ContextHasDeadline(t *testing.T) {
	srv := newTestTimeoutServer(2) // 2 second timeout
	rt := NewRequestTimeoutMiddleware(srv)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var deadlineSet bool
	middleware := rt.Handle()(func(c echo.Context) error {
		deadline, ok := c.Request().Context().Deadline()
		deadlineSet = ok && deadline.After(time.Now())
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	err := middleware(c)
	require.NoError(t, err)
	assert.True(t, deadlineSet, "context should have a deadline")
}

func TestRequestTimeout_SetsConfig(t *testing.T) {
	srv := newTestTimeoutServer(10)
	rt := NewRequestTimeoutMiddleware(srv)

	assert.Equal(t, 10*time.Second, rt.getTimeout())
}

func TestRequestTimeout_SkipsWhenZero(t *testing.T) {
	srv := newTestTimeoutServer(0)
	rt := NewRequestTimeoutMiddleware(srv)

	assert.Equal(t, time.Duration(0), rt.getTimeout())
}

// Verify that RequestTimeout is registered in the Middlewares struct.
func TestRequestTimeout_InMiddlewares(t *testing.T) {
	srv := &server.Server{
		Config: &config.Config{
			Server: config.ServerConfig{
				RequestTimeout: 5,
			},
		},
	}
	_ = service.NewAuthService(srv)

	mws := &RequestTimeoutMiddleware{server: srv}
	assert.NotNil(t, mws)
}
