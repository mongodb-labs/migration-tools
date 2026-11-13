package oteltools

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mongodb-labs/migration-tools/option"
	"github.com/mongodb-labs/migration-tools/synctools"
	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
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

// parseExports splits the output into exports. It requires the output to end
// with a complete export and each export to be a valid Prometheus text
// exposition. The returned exports omit their EndOfExport lines.
func parseExports(t *testing.T, contents string) []string {
	t.Helper()

	require.True(
		t,
		strings.HasSuffix(contents, "\n"+EndOfExport+"\n"),
		"the output ends with a complete export",
	)

	var exports []string
	var current strings.Builder
	for line := range strings.Lines(contents) {
		if line != EndOfExport+"\n" {
			current.WriteString(line)
			continue
		}

		parser := expfmt.NewTextParser(model.UTF8Validation)
		_, err := parser.TextToMetricFamilies(strings.NewReader(current.String()))
		require.NoError(t, err, "export is a valid exposition: %s", current.String())

		exports = append(exports, current.String())
		current.Reset()
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

	exports := parseExports(t, string(out.Bytes()))
	require.Len(t, exports, 1, "no periodic write within an hour, then one final write")

	samples := parseSamples(t, exports[0])
	require.NotEmpty(t, samples)

	// Every sample has the time the export was gathered.
	exportTimeMS := samples[0].timestampMS
	assert.GreaterOrEqual(t, exportTimeMS, before.UnixMilli())
	assert.LessOrEqual(t, exportTimeMS, after.UnixMilli())

	for _, s := range samples {
		assert.Equal(t, exportTimeMS, s.timestampMS, "sample: %s", s.series)
	}

	counter, found := findSample(samples, "file_check_total")
	require.True(t, found, "counter in %s", exports[0])
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
				if strings.HasPrefix(line, counterName) {
					withCounter++
				}
			}

			return withCounter >= 2
		},
		5*time.Second,
		10*time.Millisecond,
		"several periodic writes with the counter",
	)

	exports := parseExports(t, out.contents())

	first := slices.IndexFunc(exports, func(export string) bool {
		return strings.Contains(export, counterName)
	})
	require.GreaterOrEqual(t, first, 0, "some export has the counter")

	// A counter, once exported, is in every later export.
	for _, export := range exports[first:] {
		assert.Contains(t, export, counterName)
	}

	// Each export is written in one Write call, so a rotating writer never
	// splits an export across files.
	for _, call := range out.writeCalls() {
		assert.Len(t, parseExports(t, call), 1, "one whole export per write: %s", call)
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

	exports := parseExports(t, string(out.Bytes()))
	require.Len(t, exports, 1)

	assert.ElementsMatch(t, handlerSeries, seriesOf(parseSamples(t, exports[0])))
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

	// Every failed export is logged, not just the first.
	require.Eventually(
		t,
		func() bool { return strings.Count(string(logs.Bytes()), errWrite.Error()) >= 2 },
		5*time.Second,
		10*time.Millisecond,
	)

	assert.ErrorIs(
		t,
		pipeline.Shutdown(t.Context()),
		errWrite,
		"Shutdown reports the last export’s error",
	)
}

// partialOnceWriter writes half of its first Write and fails, then accepts
// every later Write in full.
type partialOnceWriter struct {
	mu      sync.Mutex
	written []byte
	calls   int

	// failed is closed once the first Write has returned.
	failed chan struct{}
}

var _ io.Writer = &partialOnceWriter{}

func (w *partialOnceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.calls++
	if w.calls > 1 {
		w.written = append(w.written, p...)

		return len(p), nil
	}

	defer close(w.failed)

	half := len(p) / 2
	w.written = append(w.written, p[:half]...)

	return half, errWrite
}

func (w *partialOnceWriter) state() (string, int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return string(w.written), w.calls
}

// After a Write fails, later exports are still written. Shutdown reports only
// the last export’s error, so it succeeds once a later Write does.
func TestMetricsOut_WritesContinueAfterFailedWrite(t *testing.T) {
	out := &partialOnceWriter{failed: make(chan struct{})}

	pipeline, err := Setup(Settings{
		MetricsOut:     option.Some[io.Writer](out),
		ExportInterval: option.Some(time.Millisecond),
	})
	require.NoError(t, err)

	<-out.failed

	require.Eventually(
		t,
		func() bool {
			_, calls := out.state()
			return calls >= 2
		},
		5*time.Second,
		time.Millisecond,
	)

	require.NoError(t, pipeline.Shutdown(t.Context()), "the last export succeeded")

	written, _ := out.state()
	assert.True(
		t,
		strings.HasSuffix(written, "\n"+EndOfExport+"\n"),
		"the output ends with a complete export",
	)
}

// If stopAndFlush gives up while the last export is still gathering, that
// export writes nothing, even once gathering finishes.
func TestStreamExporter_NoWriteAfterGivingUpDuringGather(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "gather_check_total"}))

	gathering := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	gatherer := prometheus.GathererFunc(func() ([]*prometheusclient.MetricFamily, error) {
		once.Do(func() { close(gathering) })
		<-release

		return registry.Gather()
	})

	out := &recordingWriter{}
	stream := startStreamExporter(gatherer, out, slog.New(slog.DiscardHandler), time.Hour)

	ctx, cancel := context.WithCancel(t.Context())
	stopErr := make(chan error, 1)
	go func() { stopErr <- stream.stopAndFlush(ctx) }()

	<-gathering
	cancel()
	require.ErrorIs(t, <-stopErr, context.Canceled)

	close(release)
	<-stream.result.Ready()

	assert.Empty(t, out.writeCalls(), "no write after stopAndFlush gives up")
}
