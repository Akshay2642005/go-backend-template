package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestBodyLogger_Enabled(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.POST("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	body := `{"message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_Disabled(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: false}).Handle())

	e.POST("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	body := `{"message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_SkipsHealthEndpoints(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "alive"})
	})

	e.GET("/readyz", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ready"})
	})

	e.GET("/status", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "healthy"})
	})

	for _, path := range []string{"/healthz", "/readyz", "/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, 200, rec.Code, "expected 200 for %s", path)
	}
}

func TestBodyLogger_PassesThroughNonHealthPaths(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.GET("/api/v1/posts", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "posts"})
	})

	for _, path := range []string{"/api/v1/posts"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, 200, rec.Code, "expected 200 for %s", path)
	}
}

func TestBodyLogger_SkipsCustomPaths(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{
		Enabled:   true,
		SkipPaths: []string{"/custom"},
	}).Handle())

	e.GET("/custom/endpoint", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "custom"})
	})

	req := httptest.NewRequest(http.MethodGet, "/custom/endpoint", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_LargeBody(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.POST("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Create a body larger than 1KB
	largeBody := strings.Repeat("x", 2048)
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestTruncateBody(t *testing.T) {
	// Small body - no truncation
	small := []byte("hello")
	assert.Equal(t, "hello", truncateBody(small))

	// Large body - truncation
	large := []byte(strings.Repeat("a", 2048))
	result := truncateBody(large)
	truncatedSuffix := "...(truncated)"
	assert.Equal(t, maxBodyLogSize+len(truncatedSuffix), len(result))
	assert.True(t, strings.HasSuffix(result, truncatedSuffix))

	// Empty body
	assert.Equal(t, "", truncateBody([]byte{}))
}

func TestBodyCapture(t *testing.T) {
	var buf bytes.Buffer
	capture := &bodyCapture{Writer: &buf, body: &bytes.Buffer{}}

	data := []byte("test data")
	n, err := capture.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Verify it was written to both writers
	assert.Equal(t, "test data", buf.String())
	assert.Equal(t, "test data", capture.body.String())
}

func TestBodyCapture_Unwrap(t *testing.T) {
	var buf bytes.Buffer
	capture := &bodyCapture{Writer: &buf, body: &bytes.Buffer{}}

	unwrapped := capture.Unwrap()
	assert.Equal(t, &buf, unwrapped)
}

func TestBodyLogger_GETNoBody(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.GET("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_PutBody(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.PUT("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	body := `{"id":1,"name":"updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_PatchBody(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.PATCH("/api/test", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	body := `{"name":"patched"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}

func TestBodyLogger_JSON(t *testing.T) {
	e := echo.New()
	e.Use(NewBodyLogger(BodyLoggerConfig{Enabled: true}).Handle())

	e.POST("/api/test", func(c echo.Context) error {
		var req map[string]interface{}
		if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid JSON"})
		}
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	body := `{"key":"value","nested":{"a":1}}`
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
}
