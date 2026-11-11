package oteltools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		MetricsOut:     option.Some[io.Writer](&lockedFile{}),
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
}

func TestSetup_RuntimeCollectors(t *testing.T) {
	pipeline := setupForTest(t, Settings{})

	body := scrape(t, pipeline.Handler())
	assert.Contains(t, body, "go_goroutines")
	assert.Contains(t, body, "process_cpu_seconds_total")
}

func TestPipeline_ShutdownIsIdempotent(t *testing.T) {
	file := &lockedFile{}

	pipeline, err := Setup(Settings{MetricsOut: option.Some[io.Writer](file)})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
	require.NoError(t, pipeline.Shutdown(t.Context()))

	assert.Equal(t, 1, file.closeCount(), "Shutdown closes the metrics file once")
}

func TestPipeline_ShutdownWithoutMetricsFile(t *testing.T) {
	pipeline, err := Setup(Settings{})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
}

func TestPipeline_ShutdownWithNonCloserFile(t *testing.T) {
	var buf bytes.Buffer

	pipeline, err := Setup(Settings{MetricsOut: option.Some[io.Writer](&buf)})
	require.NoError(t, err)

	require.NoError(t, pipeline.Shutdown(t.Context()))
	assert.NotEmpty(t, buf.Bytes(), "Shutdown writes a final export")
}

func TestSetup_RejectsNonPositiveExportInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			_, err := Setup(Settings{
				MetricsOut:     option.Some[io.Writer](&lockedFile{}),
				ExportInterval: option.Some(interval),
			})
			assert.ErrorIs(t, err, errNonPositiveExportInterval)
		})
	}
}
