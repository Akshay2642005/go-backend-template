package cache

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// MarshalValue serializes a value for storage. []byte values pass through
// unchanged; everything else is JSON-encoded.
func MarshalValue(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case []byte:
		return val, nil
	case string:
		return []byte(val), nil
	case int:
		return []byte(strconv.Itoa(val)), nil
	case int64:
		return []byte(strconv.FormatInt(val, 10)), nil
	case float64:
		return []byte(strconv.FormatFloat(val, 'f', -1, 64)), nil
	case bool:
		return []byte(strconv.FormatBool(val)), nil
	default:
		return json.Marshal(val)
	}
}

// UnmarshalValue deserializes a value from Redis bytes into dest.
// If dest is a *[]byte it receives the raw bytes.
// If dest is a *string it receives the string representation.
// Otherwise JSON unmarshalling is used.
func UnmarshalValue(data []byte, dest interface{}) error {
	switch d := dest.(type) {
	case *[]byte:
		*d = make([]byte, len(data))
		copy(*d, data)
		return nil
	case *string:
		*d = string(data)
		return nil
	case *int:
		n, err := strconv.Atoi(string(data))
		if err != nil {
			return fmt.Errorf("cache: unmarshal int: %w", err)
		}
		*d = n
		return nil
	case *int64:
		n, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return fmt.Errorf("cache: unmarshal int64: %w", err)
		}
		*d = n
		return nil
	case *float64:
		n, err := strconv.ParseFloat(string(data), 64)
		if err != nil {
			return fmt.Errorf("cache: unmarshal float64: %w", err)
		}
		*d = n
		return nil
	case *bool:
		b, err := strconv.ParseBool(string(data))
		if err != nil {
			return fmt.Errorf("cache: unmarshal bool: %w", err)
		}
		*d = b
		return nil
	default:
		return json.Unmarshal(data, dest)
	}
}
