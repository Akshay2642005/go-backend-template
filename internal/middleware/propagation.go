package middleware

import (
	"backend/internal/contextutil"
)

// Re-export contextutil types for backward compatibility.
// New code should import contextutil directly.

type PropagatedValues = contextutil.PropagatedValues

var (
	WithPropagatedValues   = contextutil.WithPropagatedValues
	PropagatedValuesFrom   = contextutil.PropagatedValuesFrom
	RequestIDFrom          = contextutil.RequestIDFrom
	UserIDFrom             = contextutil.UserIDFrom
	TraceIDFrom            = contextutil.TraceIDFrom
	SpanIDFrom             = contextutil.SpanIDFrom
)

// Ensure the type alias works correctly at compile time.
var _ PropagatedValues = contextutil.PropagatedValues{}
