package sysinfo

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// logfWriter wraps t.Logf to implement io.Writer.
type logfWriter struct {
	t *testing.T
}

func (w logfWriter) Write(p []byte) (n int, err error) {
	w.t.Log(p)
	return len(p), nil
}

func TestLogSystemInfo(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Write to both the test log and the buffer
	multiWriter := io.MultiWriter(logfWriter{t}, &buf)

	// Create a logger that writes to the multi-writer
	handler := slog.NewTextHandler(multiWriter, nil)
	logger := slog.New(handler)

	// Call the function
	LogSystemInfo(logger)

	// Assert the buffer contains expected content
	output := buf.String()

	expectedFields := []string{
		"System info",
		"cpus",
		"gomaxprocs",
		"gomemlimit",
		"totalRAM",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("expected log output to contain %q, got: %s", field, output)
		}
	}
}
