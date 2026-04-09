package slogtools

import (
	"fmt"
	"log/slog"
	"strings"
)

// StringSlice is a string slice that implements slog.LogValuer to format
// itself as a JSON-like array with quoted string elements.
type StringSlice []string

func (ss StringSlice) LogValue() slog.Value {
	return slog.StringValue(fmt.Sprintf(`["%s"]`, strings.Join(ss, `", "`)))
}
