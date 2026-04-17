// Package slogtools exports useful tools for working with
// Go's slog package.
package slogtools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
)

// SliceFormattingHandler wraps slog.NewTextHandler and formats slice values
// as [elem1, elem2, ...] where string elements are quoted and other types
// are formatted with fmt.Sprint.
//
// It formats slices consistently:
//   - Numeric slices: [1, 4, 77]
//   - String slices: ["foo", "bar"]
type SliceFormattingHandler struct {
	inner slog.Handler
}

// NewSliceFormattingHandler creates a new SliceFormattingHandler with a
// TextHandler writing to w with the given options.
func NewSliceFormattingHandler(w io.Writer, opts *slog.HandlerOptions) *SliceFormattingHandler {
	return &SliceFormattingHandler{
		inner: slog.NewTextHandler(w, opts),
	}
}

// Enabled implements slog.Handler.
func (h *SliceFormattingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle implements slog.Handler by formatting any slice-type values and
// delegating to the wrapped handler.
func (h *SliceFormattingHandler) Handle(ctx context.Context, record slog.Record) error {
	// Collect modified attributes
	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Value.Kind() == slog.KindAny {
			rv := reflect.ValueOf(attr.Value.Any())
			if rv.Kind() == reflect.Slice {
				attr.Value = slog.AnyValue(formatSliceValue(rv))
			}
		}
		attrs = append(attrs, attr)
		return true
	})

	// Create a new record with modified attributes
	newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	newRecord.AddAttrs(attrs...)

	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs implements slog.Handler.
func (h *SliceFormattingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SliceFormattingHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h *SliceFormattingHandler) WithGroup(name string) slog.Handler {
	return &SliceFormattingHandler{inner: h.inner.WithGroup(name)}
}

type formattedSliceValue string

func (fs formattedSliceValue) String() string {
	return string(fs)
}

func formatSliceValue(rv reflect.Value) formattedSliceValue {
	parts := make([]string, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.String {
			parts[i] = strconv.Quote(elem.String())
		} else {
			parts[i] = fmt.Sprint(elem.Interface())
		}
	}
	return formattedSliceValue("[" + strings.Join(parts, ", ") + "]")
}
