package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		dest     interface{}
		expected interface{}
	}{
		{"string", "hello", new(string), "hello"},
		{"int", 42, new(int), 42},
		{"int64", int64(999), new(int64), int64(999)},
		{"float64", 3.14, new(float64), 3.14},
		{"bool", true, new(bool), true},
		{"bytes", []byte("raw"), new([]byte), []byte("raw")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalValue(tt.value)
			require.NoError(t, err)

			err = UnmarshalValue(data, tt.dest)
			require.NoError(t, err)

			switch d := tt.dest.(type) {
			case *string:
				assert.Equal(t, tt.expected, *d)
			case *int:
				assert.Equal(t, tt.expected, *d)
			case *int64:
				assert.Equal(t, tt.expected, *d)
			case *float64:
				assert.Equal(t, tt.expected, *d)
			case *bool:
				assert.Equal(t, tt.expected, *d)
			case *[]byte:
				assert.Equal(t, tt.expected, *d)
			}
		})
	}
}

func TestUnmarshalInvalid(t *testing.T) {
	var n int
	err := UnmarshalValue([]byte("not_a_number"), &n)
	assert.Error(t, err)

	var b bool
	err = UnmarshalValue([]byte("not_bool"), &b)
	assert.Error(t, err)
}
