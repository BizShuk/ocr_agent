package vision

import "strings"

// ExtractJSONObject returns the outermost {...} span of s, tolerating the
// markdown fences and stray prose small local models wrap answers in even
// when told not to. Returns "" when there is no object.
func ExtractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// Truncate shortens a reply for error messages so a runaway model cannot
// flood the batch log.
func Truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
