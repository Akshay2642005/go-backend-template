// Package propagation provides shared context value types used across the
// application to avoid import cycles between middleware and library packages.
package propagation

import "context"

type propagationKey struct{}

// PropagatedValues holds request-scoped values that are injected into
// context.Context for use in service, repository, and background job layers.
type PropagatedValues struct {
	RequestID string
	UserID    string
	TraceID   string
	SpanID    string
}

// WithPropagatedValues injects propagated values into the context.
func WithPropagatedValues(ctx context.Context, v *PropagatedValues) context.Context {
	return context.WithValue(ctx, propagationKey{}, v)
}

// PropagatedValuesFrom extracts propagated values from the context.
func PropagatedValuesFrom(ctx context.Context) (*PropagatedValues, bool) {
	v, ok := ctx.Value(propagationKey{}).(*PropagatedValues)
	return v, ok
}

// RequestIDFrom extracts the request ID from the context.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := PropagatedValuesFrom(ctx); ok {
		return v.RequestID
	}
	return ""
}

// UserIDFrom extracts the user ID from the context.
func UserIDFrom(ctx context.Context) string {
	if v, ok := PropagatedValuesFrom(ctx); ok {
		return v.UserID
	}
	return ""
}

// TraceIDFrom extracts the trace ID from the context.
func TraceIDFrom(ctx context.Context) string {
	if v, ok := PropagatedValuesFrom(ctx); ok {
		return v.TraceID
	}
	return ""
}

// SpanIDFrom extracts the span ID from the context.
func SpanIDFrom(ctx context.Context) string {
	if v, ok := PropagatedValuesFrom(ctx); ok {
		return v.SpanID
	}
	return ""
}
