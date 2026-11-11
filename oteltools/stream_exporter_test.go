package oteltools

import (
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
	"github.com/mongodb-labs/migration-tools/synctools"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// recordingWriter is a concurrency-safe writer that records each Write call
// and counts Close calls. Tests that need only the written bytes use
// synctools.Buffer instead.
type recordingWriter struct {
	mu     sync.Mutex
	writes []string
	closes int
}

var _ io.WriteCloser = &recordingWriter{}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.writes = append(w.writes, string(p))

	return len(p), nil
}

func (w *recordingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closes++

	return nil
}

func (w *recordingWriter) contents() string {
	return strings.Join(w.writeCalls(), "")
}

func (w *recordingWriter) writeCalls() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.writes)
}

func (w *recordingWriter) closeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.closes
}

// parseExportLines decodes exported lines. It requires the output to end
// with a complete line, each line to be a JSON object with an RFC 3339
// time, and each message to be a valid Prometheus text exposition.
func parseExportLines(t *testing.T, contents string) []exportLine {
	t.Helper()

	require.True(t, strings.HasSuffix(contents, "\n"), "the output ends with a complete line")

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

func TestMetricsOut_FinalExportOnShutdown(t *testing.T) {
	out := &synctools.Buffer{}

	pipeline, err := Setup(Settings{
		Resource:       resource.NewSchemaless(semconv.ServiceName("svc")),
		ResourceLabels: attribute.NewAllowKeysFilter(semconv.ServiceNameKey),
		MetricsOut:     option.Some[io.Writer](out),
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

	exports := parseExportLines(t, string(out.Bytes()))
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
}

func findSample(samples []sample, name string) (sample, bool) {
	idx := slices.IndexFunc(samples, func(s sample) bool { return s.name == name })
	if idx < 0 {
		return sample{}, false
	}

	return samples[idx], true
}

func TestMetricsOut_PeriodicWrites(t *testing.T) {
	out := &recordingWriter{}

	pipeline := setupForTest(t, Settings{
		MetricsOut:     option.Some[io.Writer](out),
		ExportInterval: option.Some(10 * time.Millisecond),
	})

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("periodic_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)

	// The worker starts in Setup, before the counter exists, so the first
	// exports may lack it.
	const counterName = "periodic_check_total"

	require.Eventually(
		t,
		func() bool {
			withCounter := 0
			for line := range strings.Lines(out.contents()) {
				if strings.Contains(line, counterName) {
					withCounter++
				}
			}

			return withCounter >= 2
		},
		5*time.Second,
		10*time.Millisecond,
		"several periodic writes with the counter",
	)

	exports := parseExportLines(t, out.contents())

	first := slices.IndexFunc(exports, func(export exportLine) bool {
		return strings.Contains(export.Message, counterName)
	})
	require.GreaterOrEqual(t, first, 0, "some export has the counter")

	// A counter, once exported, is in every later export.
	for _, export := range exports[first:] {
		assert.Contains(t, export.Message, counterName)
	}

	// Each export is written in one Write call, so a rotating writer never
	// splits an export across files.
	for _, call := range out.writeCalls() {
		assert.Len(t, parseExportLines(t, call), 1, "one whole line per write: %s", call)
	}
}

// MetricsOut and the HTTP handler read the same registry, so they contain the
// same series.
func TestMetricsOut_MatchesHandler(t *testing.T) {
	out := &synctools.Buffer{}

	pipeline, err := Setup(Settings{
		Resource:       resource.NewSchemaless(semconv.ServiceName("svc")),
		ResourceLabels: attribute.NewAllowKeysFilter(semconv.ServiceNameKey),
		MetricsOut:     option.Some[io.Writer](out),
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

	exports := parseExportLines(t, string(out.Bytes()))
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

func TestMetricsOut_WriteErrorsAreLogged(t *testing.T) {
	var logs synctools.Buffer

	pipeline, err := Setup(Settings{
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)),
		MetricsOut:     option.Some[io.Writer](failingWriter{}),
		ExportInterval: option.Some(10 * time.Millisecond),
	})
	require.NoError(t, err)

	require.Eventually(
		t,
		func() bool { return strings.Contains(string(logs.Bytes()), errWrite.Error()) },
		5*time.Second,
		10*time.Millisecond,
	)

	assert.ErrorIs(t, pipeline.Shutdown(t.Context()), errWrite, "the final write fails too")
}

// Each line has exactly the fields of mongosync’s metrics log lines that
// readers of those files rely on.
func TestMetricsOut_LineFields(t *testing.T) {
	out := &synctools.Buffer{}

	pipeline, err := Setup(Settings{
		MetricsOut:     option.Some[io.Writer](out),
		ExportInterval: option.Some(time.Hour),
	})
	require.NoError(t, err)

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("fields_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 1, otelmetric.WithAttributes(attribute.String("op", "<&>")))
	require.NoError(t, pipeline.Shutdown(t.Context()))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &fields))

	assert.ElementsMatch(t, []string{"time", "message"}, slices.Collect(maps.Keys(fields)))
	assert.Contains(t, string(out.Bytes()), `op=\"<&>\"`, "the exposition isn’t HTML-escaped")
}
