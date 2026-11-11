package oteltools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mongodb-labs/migration-tools/option"
	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func scrape(t *testing.T, handler http.Handler) string {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	return string(body)
}

func defaultGathererNames(t *testing.T) []string {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	return lo.Map(
		families,
		func(f *prometheusclient.MetricFamily, _ int) string { return f.GetName() },
	)
}

func setupForTest(t *testing.T, settings Settings) *Pipeline {
	t.Helper()

	pipeline, err := Setup(settings)
	require.NoError(t, err)

	// t.Context() is already canceled when cleanups run.
	t.Cleanup(func() { require.NoError(t, pipeline.Shutdown(context.Background())) })

	return pipeline
}

func TestSetup_LeavesGlobalStateAlone(t *testing.T) {
	globalProvider := otel.GetMeterProvider()
	gathererNamesBefore := defaultGathererNames(t)

	pipeline := setupForTest(t, Settings{
		Resource:       resource.NewSchemaless(semconv.ServiceName("svc")),
		ResourceLabels: attribute.NewAllowKeysFilter(semconv.ServiceNameKey),
		MetricsOut:     option.Some[io.Writer](&lockedBuffer{}),
	})

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("global_state_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)
	_ = scrape(t, pipeline.Handler())
	require.NoError(t, pipeline.Shutdown(t.Context()))

	assert.Equal(t, globalProvider, otel.GetMeterProvider(), "global meter provider")
	assert.ElementsMatch(
		t,
		gathererNamesBefore,
		defaultGathererNames(t),
		"Prometheus default registry",
	)
}

func TestSetup_Handler(t *testing.T) {
	pipeline := setupForTest(t, Settings{
		Resource: resource.NewSchemaless(
			semconv.ServiceName("svc"),
			semconv.HostName("host1"),
			attribute.String("not.allowed", "x"),
		),
		ResourceLabels: attribute.NewAllowKeysFilter(
			semconv.ServiceNameKey,
			semconv.HostNameKey,
		),
	})

	counter, err := pipeline.MeterProvider().Meter("test").Int64Counter("handler_check")
	require.NoError(t, err)
	counter.Add(t.Context(), 7, otelmetric.WithAttributes(attribute.String("cluster", "src")))

	body := scrape(t, pipeline.Handler())

	series, found := lo.Find(
		strings.Split(body, "\n"),
		func(line string) bool { return strings.HasPrefix(line, "handler_check_total{") },
	)
	require.True(t, found, "counter series in %s", body)

	assert.Contains(t, series, `cluster="src"`)
	assert.Contains(t, series, `service_name="svc"`)
	assert.Contains(t, series, `host_name="host1"`)
	assert.NotContains(t, series, "not_allowed", "only allowed resource keys become labels")

	// The runtime collectors’ series get the same resource labels.
	for _, prefix := range []string{"go_goroutines{", "process_cpu_seconds_total{"} {
		runtimeSeries, found := lo.Find(
			strings.Split(body, "\n"),
			func(line string) bool { return strings.HasPrefix(line, prefix) },
		)
		require.True(t, found, "%s series in %s", prefix, body)

		assert.Contains(t, runtimeSeries, `service_name="svc"`)
		assert.Contains(t, runtimeSeries, `host_name="host1"`)
		assert.NotContains(t, runtimeSeries, "not_allowed")
	}
}

func TestSetup_RuntimeCollectors(t *testing.T) {
	pipeline := setupForTest(t, Settings{})

	body := scrape(t, pipeline.Handler())
	assert.Contains(t, body, "go_goroutines")
	assert.Contains(t, body, "process_cpu_seconds_total")
}

func TestPipeline_ShutdownIsIdempotent(t *testing.T) {
	out := &lockedBuffer{}

	pipeline, err := Setup(Settings{MetricsOut: option.Some[io.Writer](out)})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
	writes := out.writeCalls()

	require.NoError(t, pipeline.Shutdown(t.Context()))
	assert.Equal(t, writes, out.writeCalls(), "a second Shutdown writes nothing")
}

// The caller opened the metrics output, so the caller closes it.
func TestPipeline_ShutdownLeavesMetricsOutOpen(t *testing.T) {
	out := &lockedBuffer{}

	pipeline, err := Setup(Settings{MetricsOut: option.Some[io.Writer](out)})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
	assert.Zero(t, out.closeCount())
}

func TestPipeline_ShutdownWithoutMetricsOut(t *testing.T) {
	pipeline, err := Setup(Settings{})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
}

func TestPipeline_ShutdownWritesFinalExport(t *testing.T) {
	var buf bytes.Buffer

	pipeline, err := Setup(Settings{MetricsOut: option.Some[io.Writer](&buf)})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
	assert.NotEmpty(t, buf.Bytes(), "Shutdown writes a final export")
}

// blockingWriter blocks every Write until release is closed, as a pipe does
// when its reader stops reading.
type blockingWriter struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.calls.Add(1)

	select {
	case w.entered <- struct{}{}:
	default:
	}

	<-w.release

	return len(p), nil
}

// Shutdown returns once its context ends even if MetricsOut blocks, and the
// pipeline starts no further writes.
func TestPipeline_ShutdownHonorsContext(t *testing.T) {
	writer := &blockingWriter{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}

	pipeline, err := Setup(Settings{
		MetricsOut:     option.Some[io.Writer](writer),
		ExportInterval: option.Some(time.Millisecond),
	})
	require.NoError(t, err)

	select {
	case <-writer.entered:
	case <-time.After(10 * time.Second):
		require.Fail(t, "no periodic write started")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, pipeline.Shutdown(ctx), context.DeadlineExceeded)

	close(writer.release)

	select {
	case <-pipeline.stream.MustGet().result.Ready():
	case <-time.After(10 * time.Second):
		require.Fail(t, "writer goroutine didn’t exit")
	}

	assert.EqualValues(t, 1, writer.calls.Load(), "no write starts after Shutdown gives up")
}

func TestSetup_RejectsNonPositiveExportInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			_, err := Setup(Settings{
				MetricsOut:     option.Some[io.Writer](&lockedBuffer{}),
				ExportInterval: option.Some(interval),
			})
			assert.ErrorIs(t, err, errNonPositiveExportInterval)
		})
	}
}
