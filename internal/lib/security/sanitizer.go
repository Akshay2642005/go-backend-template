package security

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// SanitizeHTML removes dangerous markup and escapes the rest. Stripping
// happens before escaping: once entities are escaped there are no literal
// tags left for the patterns to match.
func SanitizeHTML(input string) string {
	// Strip script blocks entirely (including contents).
	scriptRegex := regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	sanitized := scriptRegex.ReplaceAllString(input, "")

	// Strip event-handler attributes with their values, preserving the
	// preceding space so `<div onclick="...">` becomes `<div >`.
	eventRegex := regexp.MustCompile(`(?i)on\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	sanitized = eventRegex.ReplaceAllString(sanitized, "")

	// Escape everything that remains.
	return html.EscapeString(sanitized)
}

// SanitizeString removes potentially dangerous characters from user input.
// Surrounding spaces are trimmed, but newlines and tabs are preserved
// (they are legitimate content); other control characters are stripped.
func SanitizeString(input string) string {
	// Trim spaces only — TrimSpace would also strip the newlines and
	// tabs this function promises to preserve.
	sanitized := strings.Trim(input, " ")

	// Remove null bytes
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")

	// Remove control characters except newline and tab
	var result strings.Builder
	for _, r := range sanitized {
		if unicode.IsGraphic(r) || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// SanitizeEmail validates and sanitizes email addresses
func SanitizeEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	// Remove any potentially dangerous characters
	email = strings.ReplaceAll(email, "\n", "")
	email = strings.ReplaceAll(email, "\r", "")
	return email
}

// SanitizeUUID validates and sanitizes UUID strings
func SanitizeUUID(uuid string) string {
	uuid = strings.TrimSpace(uuid)
	// Remove any non-hyphen, non-hex characters
	cleaned := regexp.MustCompile(`[^a-fA-F0-9-]`).ReplaceAllString(uuid, "")
	return cleaned
}

// ValidateInputLength checks if input is within acceptable length limits
func ValidateInputLength(input string, min, max int) bool {
	length := len(input)
	return length >= min && length <= max
}

// StripSQLKeywords removes potential SQL injection keywords (basic protection)
// Note: This is not a substitute for proper parameterized queries
func StripSQLKeywords(input string) string {
	// Remove common SQL keywords (case-insensitive)
	keywordRegex := regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|DROP|UNION|ALTER|CREATE|TRUNCATE|EXEC|SCRIPT)\b`)
	return keywordRegex.ReplaceAllString(input, "")
}
