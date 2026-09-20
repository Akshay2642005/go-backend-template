package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	assert.Equal(t, "prefix:a:b", Key("prefix", "a", "b"))
	assert.Equal(t, ":a", Key("", "a"))
	assert.Equal(t, "pfx:user:123:profile", Key("pfx", "user", "123", "profile"))
}

func TestKeyWithEnv(t *testing.T) {
	assert.Equal(t, "boilerplate:local:item:1", KeyWithEnv("local", "item", "1"))
	assert.Equal(t, "boilerplate:item:1", KeyWithEnv("", "item", "1"))
}

func TestPattern(t *testing.T) {
	assert.Equal(t, "ns:item:*", Pattern("ns", "item"))
	assert.Equal(t, ":*", Pattern(""))
}

func TestTTLWithJitter(t *testing.T) {
	ttl := 10 * time.Minute

	// Zero jitter returns original TTL
	assert.Equal(t, ttl, TTLWithJitter(ttl, 0))
	assert.Equal(t, ttl, TTLWithJitter(ttl, -1))
	assert.Equal(t, time.Duration(0), TTLWithJitter(0, 0.2))

	// Jittered TTL should be within ±20% of original
	jittered := TTLWithJitter(ttl, 0.2)
	assert.InDelta(t, float64(ttl), float64(jittered), float64(ttl)*0.21,
		"jittered TTL should be within 20%% of original")
}

func TestKeyLongSegmentHashing(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	key := Key("pfx", long)
	assert.Less(t, len(key), 128, "long segment should be hashed to stay compact")
}
