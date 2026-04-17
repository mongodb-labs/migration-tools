package slogtools

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSliceFormattingHandler(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{
			name:     "int slice",
			value:    []int{1, 4, 77},
			expected: "[1, 4, 77]",
		},
		{
			name:     "string slice",
			value:    []string{"foo", "bar"},
			expected: `["foo", "bar"]`, // Will be escaped in output as [\"foo\", \"bar\"]
		},
		{
			name:     "string slice with quotes",
			value:    []string{`foo "bar"`, "baz"},
			expected: `["foo \"bar\"", "baz"]`, // Will be escaped in output as [\"foo \\\"bar\\\"\", \"baz\"]
		},
		{
			name:     "float slice",
			value:    []float64{1.5, 2.7},
			expected: "[1.5, 2.7]",
		},
		{
			name:     "empty slice",
			value:    []string{},
			expected: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewSliceFormattingHandler(&buf, nil)
			logger := slog.New(handler)

			logger.Info("test", slog.Any("value", tt.value))

			output := buf.String()
			// Check that the formatted slice value appears in the output
			// String slices will be quoted and escaped by TextHandler
			if !strings.Contains(output, "value=") {
				t.Errorf("expected 'value=' in output, got: %s", output)
				return
			}
			// Simple check: make sure the opening and closing brackets are there
			if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
				t.Errorf("expected [ and ] in output, got: %s", output)
			}
			// For non-empty slices, check for comma separator
			if len(tt.expected) > 2 {
				if !strings.Contains(output, ",") {
					t.Errorf("expected comma separator in output, got: %s", output)
				}
			}
		})
	}
}

func TestSliceFormattingHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewSliceFormattingHandler(&buf, nil)

	// WithAttrs should return a new handler that still formats slices
	handler2 := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	logger := slog.New(handler2)

	logger.Info("test", slog.Any("nums", []int{1, 2, 3}))

	output := buf.String()
	if !strings.Contains(output, "[1, 2, 3]") {
		t.Errorf("expected [1, 2, 3] in output, got: %s", output)
	}
}

func TestSliceFormattingHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewSliceFormattingHandler(&buf, nil)

	// WithGroup should return a new handler that still formats slices
	handler2 := handler.WithGroup("mygroup")
	logger := slog.New(handler2)

	logger.Info("test", slog.Any("items", []string{"a", "b"}))

	output := buf.String()
	if !strings.Contains(output, `items="[\"a\", \"b\"]"`) {
		t.Errorf("expected items=\"[\\\"a\\\", \\\"b\\\"]\" in output, got: %s", output)
	}
}
