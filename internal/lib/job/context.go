package job

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hibiken/asynq"

	"backend/internal/lib/propagation"
)

// TaskMetadata holds request-scoped values that are propagated through
// asynq tasks so background jobs can access the original request context.
type TaskMetadata struct {
	RequestID string `json:"request_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
}

// TaskPayload is a wrapper that bundles the actual task data with
// request metadata for context propagation.
type TaskPayload struct {
	Meta    TaskMetadata    `json:"_meta"`
	Payload json.RawMessage `json:"payload"`
}

// isEnvelope checks if the raw bytes look like a TaskPayload envelope
// by checking for the "_meta" key. This avoids false positives when
// the payload is a plain JSON object.
func isEnvelope(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "{") && strings.Contains(s, `"payload"`) && strings.Contains(s, `"_meta"`)
}

// Envelope wraps a task's payload bytes with metadata from the given context.
// Returns the envelope bytes suitable for asynq.NewTask.
func Envelope(ctx context.Context, payload []byte) ([]byte, error) {
	meta := TaskMetadata{}
	if v, ok := propagation.PropagatedValuesFrom(ctx); ok {
		meta.RequestID = v.RequestID
		meta.UserID = v.UserID
		meta.TraceID = v.TraceID
		meta.SpanID = v.SpanID
	}

	return json.Marshal(TaskPayload{
		Meta:    meta,
		Payload: payload,
	})
}

// ExtractMetadata extracts PropagatedValues from a task's envelope payload
// and returns a new context with those values injected. The raw payload
// (without the metadata envelope) is returned separately.
func ExtractMetadata(ctx context.Context, t *asynq.Task) (context.Context, []byte) {
	raw := t.Payload()

	// Quick check: if it doesn't look like an envelope, return as-is
	if !isEnvelope(raw) {
		return ctx, raw
	}

	var envelope TaskPayload
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ctx, raw
	}

	// If no payload field was present, it's not a real envelope
	if envelope.Payload == nil {
		return ctx, raw
	}

	// Only inject if at least one metadata value is present
	if envelope.Meta.RequestID != "" || envelope.Meta.UserID != "" ||
		envelope.Meta.TraceID != "" || envelope.Meta.SpanID != "" {
		ctx = propagation.WithPropagatedValues(ctx, &propagation.PropagatedValues{
			RequestID: envelope.Meta.RequestID,
			UserID:    envelope.Meta.UserID,
			TraceID:   envelope.Meta.TraceID,
			SpanID:    envelope.Meta.SpanID,
		})
	}

	return ctx, []byte(envelope.Payload)
}
