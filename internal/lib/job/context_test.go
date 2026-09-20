package job

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/lib/propagation"
)

func TestEnvelope_WithMetadata(t *testing.T) {
	ctx := context.Background()
	ctx = propagation.WithPropagatedValues(ctx, &propagation.PropagatedValues{
		RequestID: "req-123",
		UserID:    "user-456",
		TraceID:   "trace-789",
		SpanID:    "span-abc",
	})

	payload := []byte(`{"to":"alice@example.com","first_name":"Alice"}`)
	envelope, err := Envelope(ctx, payload)
	require.NoError(t, err)

	var parsed TaskPayload
	require.NoError(t, json.Unmarshal(envelope, &parsed))

	assert.Equal(t, "req-123", parsed.Meta.RequestID)
	assert.Equal(t, "user-456", parsed.Meta.UserID)
	assert.Equal(t, "trace-789", parsed.Meta.TraceID)
	assert.Equal(t, "span-abc", parsed.Meta.SpanID)
	assert.Equal(t, payload, []byte(parsed.Payload))
}

func TestEnvelope_NoMetadata(t *testing.T) {
	ctx := context.Background()
	payload := []byte(`{"to":"bob@example.com"}`)
	envelope, err := Envelope(ctx, payload)
	require.NoError(t, err)

	var parsed TaskPayload
	require.NoError(t, json.Unmarshal(envelope, &parsed))

	assert.Empty(t, parsed.Meta.RequestID)
	assert.Empty(t, parsed.Meta.UserID)
	assert.Equal(t, payload, []byte(parsed.Payload))
}

func TestExtractMetadata_WithEnvelope(t *testing.T) {
	original := &propagation.PropagatedValues{
		RequestID: "req-abc",
		UserID:    "user-def",
		TraceID:   "trace-ghi",
		SpanID:    "span-jkl",
	}

	payload := []byte(`{"to":"test@example.com"}`)
	envelope, err := json.Marshal(TaskPayload{
		Meta: TaskMetadata{
			RequestID: original.RequestID,
			UserID:    original.UserID,
			TraceID:   original.TraceID,
			SpanID:    original.SpanID,
		},
		Payload: payload,
	})
	require.NoError(t, err)

	task := asynq.NewTask("email:welcome", envelope)
	ctx, rawPayload := ExtractMetadata(context.Background(), task)

	v, ok := propagation.PropagatedValuesFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "req-abc", v.RequestID)
	assert.Equal(t, "user-def", v.UserID)
	assert.Equal(t, "trace-ghi", v.TraceID)
	assert.Equal(t, "span-jkl", v.SpanID)
	assert.Equal(t, payload, rawPayload)
}

func TestExtractMetadata_NoEnvelope(t *testing.T) {
	rawPayload := []byte(`{"to":"test@example.com"}`)
	task := asynq.NewTask("email:welcome", rawPayload)

	ctx, payload := ExtractMetadata(context.Background(), task)

	_, ok := propagation.PropagatedValuesFrom(ctx)
	assert.False(t, ok)
	assert.Equal(t, rawPayload, payload)
}

func TestExtractMetadata_EmptyMetadata(t *testing.T) {
	envelope, err := json.Marshal(TaskPayload{
		Meta:    TaskMetadata{},
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)

	task := asynq.NewTask("test", envelope)
	ctx, rawPayload := ExtractMetadata(context.Background(), task)

	_, ok := propagation.PropagatedValuesFrom(ctx)
	assert.False(t, ok, "should not inject values when all metadata is empty")
	assert.Equal(t, []byte(`{}`), rawPayload)
}

func TestRoundTrip(t *testing.T) {
	// Simulate: HTTP handler creates task with metadata
	ctx := context.Background()
	ctx = propagation.WithPropagatedValues(ctx, &propagation.PropagatedValues{
		RequestID: "req-roundtrip",
		UserID:    "user-roundtrip",
	})

	// Create task payload
	origPayload, err := json.Marshal(WelcomeEmailPayload{
		To:        "roundtrip@example.com",
		FirstName: "Round",
	})
	require.NoError(t, err)

	// Envelope with metadata
	envelope, err := Envelope(ctx, origPayload)
	require.NoError(t, err)

	// Simulate: job handler extracts metadata
	task := asynq.NewTask(TaskWelcome, envelope)
	jobCtx, rawPayload := ExtractMetadata(context.Background(), task)

	// Verify metadata propagated
	v, ok := propagation.PropagatedValuesFrom(jobCtx)
	require.True(t, ok)
	assert.Equal(t, "req-roundtrip", v.RequestID)
	assert.Equal(t, "user-roundtrip", v.UserID)

	// Verify payload is intact
	var p WelcomeEmailPayload
	require.NoError(t, json.Unmarshal(rawPayload, &p))
	assert.Equal(t, "roundtrip@example.com", p.To)
	assert.Equal(t, "Round", p.FirstName)
}

func TestIsEnvelope(t *testing.T) {
	assert.True(t, isEnvelope([]byte(`{"_meta":{},"payload":{}}`)))
	assert.True(t, isEnvelope([]byte(`{"payload":"abc","_meta":{"request_id":"1"}}`)))
	assert.False(t, isEnvelope([]byte(`{"to":"test@example.com"}`)))
	assert.False(t, isEnvelope([]byte(`"just a string"`)))
	assert.False(t, isEnvelope([]byte(`[1,2,3]`)))
	assert.False(t, isEnvelope(nil))
}
