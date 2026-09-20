package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropagatedValuesRoundTrip(t *testing.T) {
	vals := &PropagatedValues{
		RequestID: "req-123",
		UserID:    "user-456",
		TraceID:   "trace-789",
		SpanID:    "span-abc",
	}

	ctx := WithPropagatedValues(context.Background(), vals)

	got, ok := PropagatedValuesFrom(ctx)
	assert.True(t, ok)
	assert.Equal(t, "req-123", got.RequestID)
	assert.Equal(t, "user-456", got.UserID)
	assert.Equal(t, "trace-789", got.TraceID)
	assert.Equal(t, "span-abc", got.SpanID)
}

func TestPropagatedValuesMissing(t *testing.T) {
	_, ok := PropagatedValuesFrom(context.Background())
	assert.False(t, ok)
}

func TestAccessorFunctions(t *testing.T) {
	vals := &PropagatedValues{
		RequestID: "req-1",
		UserID:    "user-2",
		TraceID:   "trace-3",
		SpanID:    "span-4",
	}

	ctx := WithPropagatedValues(context.Background(), vals)

	assert.Equal(t, "req-1", RequestIDFrom(ctx))
	assert.Equal(t, "user-2", UserIDFrom(ctx))
	assert.Equal(t, "trace-3", TraceIDFrom(ctx))
	assert.Equal(t, "span-4", SpanIDFrom(ctx))
}

func TestAccessorFunctionsEmptyContext(t *testing.T) {
	ctx := context.Background()

	assert.Equal(t, "", RequestIDFrom(ctx))
	assert.Equal(t, "", UserIDFrom(ctx))
	assert.Equal(t, "", TraceIDFrom(ctx))
	assert.Equal(t, "", SpanIDFrom(ctx))
}
