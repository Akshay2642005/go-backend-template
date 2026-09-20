package cache

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCacheMiss(t *testing.T) {
	assert.True(t, IsCacheMiss(ErrCacheMiss))
	assert.False(t, IsCacheMiss(errors.New("other error")))
	assert.False(t, IsCacheMiss(nil))
}
