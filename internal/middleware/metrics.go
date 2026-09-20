package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"backend/internal/server"
)

// MetricsMiddleware instruments HTTP requests with OpenTelemetry metrics.
type MetricsMiddleware struct {
	server        *server.Server
	requestCount  metric.Int64Counter
	requestDur    metric.Float64Histogram
	responseSizes metric.Int64Histogram
}

func NewMetricsMiddleware(s *server.Server) *MetricsMiddleware {
	m := &MetricsMiddleware{server: s}

	if s.Config != nil && s.Config.Observability.Metrics.Enabled {
		m.initOTelMetrics()
	}

	return m
}

func (m *MetricsMiddleware) initOTelMetrics() {
	meter := otel.Meter(m.server.Config.Observability.ServiceName)

	var err error

	m.requestCount, err = meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		m.server.Logger.Warn().Err(err).Msg("failed to create request count meter")
	}

	m.requestDur, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		m.server.Logger.Warn().Err(err).Msg("failed to create request duration meter")
	}

	m.responseSizes, err = meter.Int64Histogram(
		"http.server.response.size",
		metric.WithDescription("HTTP response sizes in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		m.server.Logger.Warn().Err(err).Msg("failed to create response size meter")
	}
}

// Instrument returns an echo.MiddlewareFunc that records OTel metrics.
func (m *MetricsMiddleware) Instrument() echo.MiddlewareFunc {
	if m.server.Config == nil || !m.server.Config.Observability.Metrics.Enabled || m.requestCount == nil {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			duration := time.Since(start).Seconds()
			status := c.Response().Status
			if status == 0 {
				status = 200
			}

			attrs := metric.WithAttributes(
				attribute.String("method", c.Request().Method),
				attribute.String("route", c.Path()),
				attribute.Int("status", status),
			)

			m.requestCount.Add(c.Request().Context(), 1, attrs)
			m.requestDur.Record(c.Request().Context(), duration, attrs)

			if c.Response().Size > 0 {
				m.responseSizes.Record(c.Request().Context(), c.Response().Size, attrs)
			}

			return err
		}
	}
}

// RecordCacheMetrics records cache hit/miss metrics with OTel.
func RecordCacheMetrics(server *server.Server, hit bool, operation string) {
	if server.Config == nil || !server.Config.Observability.Metrics.Enabled {
		return
	}

	meter := otel.Meter(server.Config.Observability.ServiceName)
	counter, err := meter.Int64Counter(
		"cache.operation.count",
		metric.WithDescription("Cache operation count"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return
	}

	status := "miss"
	if hit {
		status = "hit"
	}

	counter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
	))
}
