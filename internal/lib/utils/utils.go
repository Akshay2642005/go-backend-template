package utils

import "encoding/json"

func JsonEncode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func JsonDecode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// StrPtr returns a pointer to the given string
//
//go:fix inline
func StrPtr(s string) *string {
	return new(s)
}

// IntPtr returns a pointer to the given int
func IntPtr(i int) *int {
	return new(i)
}

// IsEmptyString returns true if the string is empty
func IsEmptyString(s string) bool {
	return s == ""
}
