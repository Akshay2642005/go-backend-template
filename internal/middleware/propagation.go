package middleware

import (
	"backend/internal/lib/propagation"
)

// Re-export contextutil types for backward compatibility.
// New code should import contextutil directly.

type PropagatedValues = propagation.PropagatedValues

var (
	WithPropagatedValues = propagation.WithPropagatedValues
	PropagatedValuesFrom = propagation.PropagatedValuesFrom
	RequestIDFrom        = propagation.RequestIDFrom
	UserIDFrom           = propagation.UserIDFrom
	TraceIDFrom          = propagation.TraceIDFrom
	SpanIDFrom           = propagation.SpanIDFrom
)

// Ensure the type alias works correctly at compile time.
var _ PropagatedValues = propagation.PropagatedValues{}
