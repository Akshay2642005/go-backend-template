package security

import (
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "script tag",
			input:    "<script>alert('xss')</script>Hello",
			expected: "Hello",
		},
		{
			name:     "html escape",
			input:    "<div>Hello</div>",
			expected: "&lt;div&gt;Hello&lt;/div&gt;",
		},
		{
			name:     "event handler",
			input:    "<div onclick=\"alert('xss')\">Click</div>",
			expected: "&lt;div &gt;Click&lt;/div&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeHTML() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "null bytes",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "control characters",
			input:    "Hello\x01World",
			expected: "HelloWorld",
		},
		{
			name:     "newline and tab",
			input:    "Hello\nWorld\t",
			expected: "Hello\nWorld\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal email",
			input:    "USER@EXAMPLE.COM",
			expected: "user@example.com",
		},
		{
			name:     "email with spaces",
			input:    " user@example.com ",
			expected: "user@example.com",
		},
		{
			name:     "email with newline",
			input:    "user@example.com\n",
			expected: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeEmail() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateInputLength(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		min      int
		max      int
		expected bool
	}{
		{
			name:     "valid length",
			input:    "Hello",
			min:      3,
			max:      10,
			expected: true,
		},
		{
			name:     "too short",
			input:    "Hi",
			min:      3,
			max:      10,
			expected: false,
		},
		{
			name:     "too long",
			input:    "Hello World",
			min:      3,
			max:      5,
			expected: false,
		},
		{
			name:     "exact min",
			input:    "Hi",
			min:      2,
			max:      10,
			expected: true,
		},
		{
			name:     "exact max",
			input:    "Hello",
			min:      1,
			max:      5,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateInputLength(tt.input, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("ValidateInputLength() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStripSQLKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "select keyword",
			input:    "SELECT * FROM users",
			expected: " * FROM users",
		},
		{
			name:     "drop keyword",
			input:    "DROP TABLE users",
			expected: " TABLE users",
		},
		{
			name:     "mixed case",
			input:    "SeLeCt * From users",
			expected: " * From users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripSQLKeywords(tt.input)
			if result != tt.expected {
				t.Errorf("StripSQLKeywords() = %v, want %v", result, tt.expected)
			}
		})
	}
}
