package cache

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// metrics tracks cache performance with atomic counters.
type metrics struct {
	hits         atomic.Int64
	misses       atomic.Int64
	errors       atomic.Int64
	sets         atomic.Int64
	deletes      atomic.Int64
	singleflight atomic.Int64
	slowOps      atomic.Int64
}

func (m *metrics) RecordHit() {
	m.hits.Add(1)
}

func (m *metrics) RecordMiss() {
	m.misses.Add(1)
}

func (m *metrics) RecordError() {
	m.errors.Add(1)
}

func (m *metrics) RecordSet() {
	m.sets.Add(1)
}

func (m *metrics) RecordDelete() {
	m.deletes.Add(1)
}

func (m *metrics) RecordSingleflight() {
	m.singleflight.Add(1)
}

func (m *metrics) RecordSlowOp(duration time.Duration, threshold time.Duration) {
	if duration >= threshold {
		m.slowOps.Add(1)
	}
}

// Snapshot returns the current counter values.
type MetricsSnapshot struct {
	Hits         int64
	Misses       int64
	Errors       int64
	Sets         int64
	Deletes      int64
	Singleflight int64
	SlowOps      int64
}

func (m *metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		Hits:         m.hits.Load(),
		Misses:       m.misses.Load(),
		Errors:       m.errors.Load(),
		Sets:         m.sets.Load(),
		Deletes:      m.deletes.Load(),
		Singleflight: m.singleflight.Load(),
		SlowOps:      m.slowOps.Load(),
	}
}

// LogMetrics writes the current metrics snapshot to the logger.
func LogMetrics(logger *zerolog.Logger, snap MetricsSnapshot) {
	logger.Info().
		Int64("cache_hits", snap.Hits).
		Int64("cache_misses", snap.Misses).
		Int64("cache_errors", snap.Errors).
		Int64("cache_sets", snap.Sets).
		Int64("cache_deletes", snap.Deletes).
		Int64("cache_singleflight", snap.Singleflight).
		Int64("cache_slow_ops", snap.SlowOps).
		Msg("cache metrics")
}
