package slogtools

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type logfWriter struct {
	t *testing.T
}

func (w logfWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}

func TestStringSliceLogValue(t *testing.T) {
	tests := []struct {
		name     string
		slice    StringSlice
		expected string
	}{
		{
			name:     "empty slice",
			slice:    StringSlice{},
			expected: `[""]`,
		},
		{
			name:     "single element",
			slice:    StringSlice{"1 GiB"},
			expected: `["1 GiB"]`,
		},
		{
			name:     "multiple elements",
			slice:    StringSlice{"1 GiB", "2 MiB"},
			expected: `["1 GiB", "2 MiB"]`,
		},
		{
			name:     "with spaces and special chars",
			slice:    StringSlice{"Apple M1 Max", "ARM 64-bit"},
			expected: `["Apple M1 Max", "ARM 64-bit"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Create a multi-writer that writes to both t.Logf and the buffer
			multiWriter := io.MultiWriter(logfWriter{t}, &buf)

			// Create a logger that writes to the multi-writer
			handler := slog.NewTextHandler(multiWriter, nil)
			logger := slog.New(handler)

			// Log the StringSlice
			logger.Info("test", slog.Any("values", tt.slice))

			// Check that the buffer contains the expected format (escaped for TextHandler output)
			output := buf.String()
			// TextHandler escapes quotes in string values
			escaped := strings.ReplaceAll(tt.expected, `"`, `\"`)
			if !strings.Contains(output, escaped) {
				t.Errorf("expected log output to contain %q, got: %s", escaped, output)
			}
		})
	}
}

func TestStringSliceEmpty(t *testing.T) {
	ss := StringSlice{}
	val := ss.LogValue()

	// Should return a StringValue
	if val.Kind() != slog.KindString {
		t.Errorf("expected StringValue, got %v", val.Kind())
	}

	// Should be the quoted empty array format
	if val.String() != `[""]` {
		t.Errorf("expected %q, got %q", `[""]`, val.String())
	}
}

func TestStringSliceNil(t *testing.T) {
	var ss StringSlice
	val := ss.LogValue()

	// Should return a StringValue
	if val.Kind() != slog.KindString {
		t.Errorf("expected StringValue, got %v", val.Kind())
	}

	// Should be the quoted empty array format
	if val.String() != `[""]` {
		t.Errorf("expected %q, got %q", `[""]`, val.String())
	}
}
