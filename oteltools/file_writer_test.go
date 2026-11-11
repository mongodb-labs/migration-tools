package oteltools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mongodb-labs/migration-tools/option"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// lockedFile is a concurrency-safe in-memory file. The pipeline’s writer
// goroutine writes to it while the test reads it.
type lockedFile struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	writes []string
	closes int
}

func (f *lockedFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.writes = append(f.writes, string(p))

	return f.buf.Write(p)
}

func (f *lockedFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closes++

	return nil
}

func (f *lockedFile) contents() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.buf.String()
}

func (f *lockedFile) writeCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.writes)
}

func (f *lockedFile) closeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.closes
}

// parseExportLines decodes a metrics file’s lines. It requires the file to
// end with a complete line, each line to be a JSON object with an RFC 3339
// time, and each message to be a valid Prometheus text exposition.
func parseExportLines(t *testing.T, contents string) []exportLine {
	t.Helper()

	require.True(t, strings.HasSuffix(contents, "\n"), "the file ends with a complete line")

	var exports []exportLine
	for line := range strings.SplitSeq(strings.TrimSuffix(contents, "\n"), "\n") {
		var export exportLine
		require.NoError(t, json.Unmarshal([]byte(line), &export), "line is JSON: %s", line)

		_, err := time.Parse(time.RFC3339Nano, export.Time)
		require.NoError(t, err, "time is RFC 3339: %s", line)

		parser := expfmt.NewTextParser(model.UTF8Validation)
		_, err = parser.TextToMetricFamilies(strings.NewReader(export.Message))
		require.NoError(t, err, "message is a valid exposition: %s", export.Message)

		exports = append(exports, export)
	}

	return exports
}

type sample struct {
	name        string
	series      string
	timestampMS int64
}

// parseSamples returns the samples in Prometheus text. It requires every
// sample to have a timestamp.
func parseSamples(t *testing.T, text string) []sample {
	t.Helper()

	var samples []sample
	for line := range strings.SplitSeq(text, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		require.GreaterOrEqual(t, len(fields), 3, "sample has a timestamp: %s", line)

		ts, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
		require.NoError(t, err, "sample timestamp: %s", line)

		series := strings.Join(fields[:len(fields)-2], " ")
		name, _, _ := strings.Cut(series, "{")

		samples = append(samples, sample{name: name, series: series, timestampMS: ts})
	}

	return samples
}

func sampleNames(samples []sample) []string {
	var names []string
	for _, s := range samples {
		if !slices.Contains(names, s.name) {
			names = append(names, s.name)
		}
	}

	return names
}

type failingWriter struct{}

var errWrite = errors.New("write failed")

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

func TestMetricsFile_FinalExportOnShutdown(t *testing.T) {
	file := &lockedFile{}

	pipeline, err := Setup(Settings{
		Resource:       resource.NewSchemaless(semconv.ServiceName("svc")),
		ResourceLabels: attribute.NewAllowKeysFilter(semconv.ServiceNameKey),
		MetricsOut:     option.Some[io.Writer](file),
		ExportInterval: option.Some(time.Hour),
	})
	require.NoError(t, err)

	meter := pipeline.MeterProvider().Meter("test")

	checkCounter, err := meter.Int64Counter("file_check")
	require.NoError(t, err)
	checkCounter.Add(t.Context(), 3)

	histogram, err := meter.Int64Histogram(
		"file_duration",
		otelmetric.WithUnit("ms"),
		otelmetric.WithExplicitBucketBoundaries(10),
	)
	require.NoError(t, err)
	histogram.Record(t.Context(), 5)

	before := time.Now()
	require.NoError(t, pipeline.Shutdown(t.Context()))
	after := time.Now()

	exports := parseExportLines(t, file.contents())
	require.Len(t, exports, 1, "no periodic write within an hour, then one final write")

	exportTime, err := time.Parse(time.RFC3339Nano, exports[0].Time)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, exportTime.UnixMilli(), before.UnixMilli())
	assert.LessOrEqual(t, exportTime.UnixMilli(), after.UnixMilli())

	samples := parseSamples(t, exports[0].Message)
	require.NotEmpty(t, samples)

	for _, s := range samples {
		assert.Equal(t, exportTime.UnixMilli(), s.timestampMS, "sample: %s", s.series)
	}

	counter, found := findSample(samples, "file_check_total")
	require.True(t, found, "counter in %s", exports[0].Message)
	assert.Contains(t, counter.series, `service_name="svc"`)

	assert.Contains(
		t,
		sampleNames(samples),
		"file_duration_milliseconds_bucket",
		"names match the Prometheus exporter’s, with unit suffixes",
	)

	assert.Equal(t, 1, file.closeCount())
}

func findSample(samples []sample, name string) (sample, bool) {
	idx := slices.IndexFunc(samples, func(s sample) bool { return s.name == name })
	if idx < 0 {
		return sample{}, false
	}

	return samples[idx], true
}

func TestMetricsFile_PeriodicWrites(t *testing.T) {
	file := &lockedFile{}

	pipeline := setupForTest(t, Settings{
		MetricsOut:     option.Some[io.Writer](file),
		ExportInterval: option.Some(10 * time.Millisecond),
	})

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("periodic_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)

	require.Eventually(
		t,
		func() bool { return strings.Count(file.contents(), "\n") >= 3 },
		5*time.Second,
		10*time.Millisecond,
		"several periodic writes",
	)

	for _, export := range parseExportLines(t, file.contents()) {
		assert.Contains(t, export.Message, "periodic_check_total")
	}

	// Each export is written in one Write call, so a rotating writer never
	// splits an export across files.
	for _, call := range file.writeCalls() {
		assert.Len(t, parseExportLines(t, call), 1, "one whole line per write: %s", call)
	}
}

// The file and the HTTP handler read the same registry, so they contain the
// same series.
func TestMetricsFile_MatchesHandler(t *testing.T) {
	file := &lockedFile{}

	pipeline, err := Setup(Settings{
		Resource:       resource.NewSchemaless(semconv.ServiceName("svc")),
		ResourceLabels: attribute.NewAllowKeysFilter(semconv.ServiceNameKey),
		MetricsOut:     option.Some[io.Writer](file),
		ExportInterval: option.Some(time.Hour),
	})
	require.NoError(t, err)

	meter := pipeline.MeterProvider().Meter("test")

	counter, err := meter.Int64Counter("match_count")
	require.NoError(t, err)
	counter.Add(t.Context(), 2, otelmetric.WithAttributes(attribute.String("cluster", "dst")))

	gauge, err := meter.Float64Gauge("match_gauge", otelmetric.WithUnit("s"))
	require.NoError(t, err)
	gauge.Record(t.Context(), 1.5)

	histogram, err := meter.Int64Histogram(
		"match_hist",
		otelmetric.WithUnit("ms"),
		otelmetric.WithExplicitBucketBoundaries(1, 10),
	)
	require.NoError(t, err)
	histogram.Record(t.Context(), 4)

	// Scrape output has no timestamps, so add a placeholder to reuse
	// parseSamples.
	scraped := scrape(t, pipeline.Handler())
	var stamped strings.Builder
	for line := range strings.SplitSeq(scraped, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			line += " 0"
		}
		stamped.WriteString(line + "\n")
	}
	handlerSeries := seriesOf(parseSamples(t, stamped.String()))

	require.NoError(t, pipeline.Shutdown(t.Context()))

	exports := parseExportLines(t, file.contents())
	require.Len(t, exports, 1)

	assert.ElementsMatch(t, handlerSeries, seriesOf(parseSamples(t, exports[0].Message)))
}

func seriesOf(samples []sample) []string {
	series := make([]string, 0, len(samples))
	for _, s := range samples {
		series = append(series, s.series)
	}

	return series
}

func TestMetricsFile_WriteErrorsAreLogged(t *testing.T) {
	var logs lockedFile

	pipeline, err := Setup(Settings{
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)),
		MetricsOut:     option.Some[io.Writer](failingWriter{}),
		ExportInterval: option.Some(10 * time.Millisecond),
	})
	require.NoError(t, err)

	require.Eventually(
		t,
		func() bool { return strings.Contains(logs.contents(), errWrite.Error()) },
		5*time.Second,
		10*time.Millisecond,
	)

	assert.ErrorIs(t, pipeline.Shutdown(t.Context()), errWrite, "the final write fails too")
}

// Each line has exactly the fields of mongosync’s metrics log lines that
// readers of those files rely on.
func TestMetricsFile_LineFields(t *testing.T) {
	file := &lockedFile{}

	pipeline, err := Setup(Settings{
		MetricsOut:     option.Some[io.Writer](file),
		ExportInterval: option.Some(time.Hour),
	})
	require.NoError(t, err)

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("fields_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 1, otelmetric.WithAttributes(attribute.String("op", "<&>")))
	require.NoError(t, pipeline.Shutdown(t.Context()))

	var fields map[string]any
	require.NoError(t, json.Unmarshal([]byte(file.contents()), &fields))

	assert.ElementsMatch(t, []string{"time", "message"}, slices.Collect(maps.Keys(fields)))
	assert.Contains(t, file.contents(), `op=\"<&>\"`, "the exposition isn’t HTML-escaped")
}
