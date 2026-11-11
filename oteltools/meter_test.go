package oteltools

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func newTestMeter(t *testing.T) (Meter, *sdkmetric.ManualReader, *bytes.Buffer) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	return NewMeter(provider.Meter("test"), logger), reader, &logs
}

func collectNames(t *testing.T, reader *sdkmetric.ManualReader) []string {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &rm))

	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names = append(names, m.Name)
		}
	}

	return names
}

func TestMeter_ValidInstrument(t *testing.T) {
	meter, reader, logs := newTestMeter(t)

	meter.Int64Counter("good_counter").Add(t.Context(), 1)
	meter.Float64Histogram("good_histogram").Record(t.Context(), 1.5)

	assert.ElementsMatch(t, []string{"good_counter", "good_histogram"}, collectNames(t, reader))
	assert.Empty(t, logs.String())
}

func TestMeter_InvalidNameIsLogged(t *testing.T) {
	meter, _, logs := newTestMeter(t)

	counter := meter.Int64Counter("bad name!")
	require.NotNil(t, counter)

	// The returned instrument must be safe to use.
	counter.Add(t.Context(), 1)

	assert.Contains(t, logs.String(), "WARN")
	assert.Contains(t, logs.String(), "bad name!")
}

func TestMeter_RegisterCallback(t *testing.T) {
	meter, reader, logs := newTestMeter(t)

	gauge := meter.Int64ObservableGauge("observed_gauge")
	reg := meter.RegisterCallback(
		func(_ context.Context, o otelmetric.Observer) error {
			o.ObserveInt64(gauge, 42)
			return nil
		},
		gauge,
	)
	require.NotNil(t, reg)
	t.Cleanup(func() { require.NoError(t, reg.Unregister()) })

	assert.Contains(t, collectNames(t, reader), "observed_gauge")
	assert.Empty(t, logs.String())
}

func TestMeter_NoopMeter(t *testing.T) {
	meter := NewMeter(noop.NewMeterProvider().Meter("test"), nil)

	meter.Int64Counter("c").Add(t.Context(), 1)
	meter.Float64Gauge("g").Record(t.Context(), 1)
	assert.Equal(t, noop.NewMeterProvider().Meter("test"), meter.Unwrap())
}
