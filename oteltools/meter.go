package oteltools

import (
	"log/slog"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// Meter wraps an otelmetric.Meter. Its methods log instrument-creation failures
// instead of returning them, so a bad instrument never stops the program.
//
// It is a value type with no global state. Create one from whatever
// MeterProvider the caller was given.
type Meter struct {
	meter  otelmetric.Meter
	logger *slog.Logger
}

// NewMeter returns a Meter that creates instruments from meter and logs
// failures to logger. If logger is nil, failures are discarded.
func NewMeter(meter otelmetric.Meter, logger *slog.Logger) Meter {
	return Meter{
		meter:  meter,
		logger: loggerOrDiscard(logger),
	}
}

// Unwrap returns the underlying otelmetric.Meter.
func (m Meter) Unwrap() otelmetric.Meter {
	return m.meter
}

// instrumentOrFallback logs err, if any, and returns inst, or fallback
// if inst is nil. The SDK returns a usable instrument along with some
// errors (for example, an invalid name), so inst is preferred when present.
func instrumentOrFallback[T any](m Meter, name string, inst T, err error, fallback T) T {
	if err != nil {
		m.logger.Warn(
			"Failed to create metric instrument.",
			"metric", name,
			"error", err,
		)
	}

	if any(inst) == nil {
		return fallback
	}

	return inst
}

// RegisterCallback registers f with the underlying meter. On failure it
// logs the error and returns a no-op registration.
func (m Meter) RegisterCallback(
	f otelmetric.Callback,
	instruments ...otelmetric.Observable,
) otelmetric.Registration {
	reg, err := m.meter.RegisterCallback(f, instruments...)
	if err != nil {
		m.logger.Warn("Failed to register metric callback.", "error", err)

		return noop.Registration{}
	}

	return reg
}

// Int64Counter creates an Int64Counter.
func (m Meter) Int64Counter(
	name string,
	options ...otelmetric.Int64CounterOption,
) otelmetric.Int64Counter {
	inst, err := m.meter.Int64Counter(name, options...)
	return instrumentOrFallback[otelmetric.Int64Counter](m, name, inst, err, noop.Int64Counter{})
}

// Int64UpDownCounter creates an Int64UpDownCounter.
func (m Meter) Int64UpDownCounter(
	name string,
	options ...otelmetric.Int64UpDownCounterOption,
) otelmetric.Int64UpDownCounter {
	inst, err := m.meter.Int64UpDownCounter(name, options...)
	return instrumentOrFallback[otelmetric.Int64UpDownCounter](
		m, name, inst, err, noop.Int64UpDownCounter{},
	)
}

// Int64Histogram creates an Int64Histogram.
func (m Meter) Int64Histogram(
	name string,
	options ...otelmetric.Int64HistogramOption,
) otelmetric.Int64Histogram {
	inst, err := m.meter.Int64Histogram(name, options...)
	return instrumentOrFallback[otelmetric.Int64Histogram](
		m,
		name,
		inst,
		err,
		noop.Int64Histogram{},
	)
}

// Int64Gauge creates an Int64Gauge.
func (m Meter) Int64Gauge(
	name string,
	options ...otelmetric.Int64GaugeOption,
) otelmetric.Int64Gauge {
	inst, err := m.meter.Int64Gauge(name, options...)
	return instrumentOrFallback[otelmetric.Int64Gauge](m, name, inst, err, noop.Int64Gauge{})
}

// Int64ObservableCounter creates an Int64ObservableCounter.
func (m Meter) Int64ObservableCounter(
	name string,
	options ...otelmetric.Int64ObservableCounterOption,
) otelmetric.Int64ObservableCounter {
	inst, err := m.meter.Int64ObservableCounter(name, options...)
	return instrumentOrFallback[otelmetric.Int64ObservableCounter](
		m, name, inst, err, noop.Int64ObservableCounter{},
	)
}

// Int64ObservableUpDownCounter creates an Int64ObservableUpDownCounter.
func (m Meter) Int64ObservableUpDownCounter(
	name string,
	options ...otelmetric.Int64ObservableUpDownCounterOption,
) otelmetric.Int64ObservableUpDownCounter {
	inst, err := m.meter.Int64ObservableUpDownCounter(name, options...)
	return instrumentOrFallback[otelmetric.Int64ObservableUpDownCounter](
		m, name, inst, err, noop.Int64ObservableUpDownCounter{},
	)
}

// Int64ObservableGauge creates an Int64ObservableGauge.
func (m Meter) Int64ObservableGauge(
	name string,
	options ...otelmetric.Int64ObservableGaugeOption,
) otelmetric.Int64ObservableGauge {
	inst, err := m.meter.Int64ObservableGauge(name, options...)
	return instrumentOrFallback[otelmetric.Int64ObservableGauge](
		m, name, inst, err, noop.Int64ObservableGauge{},
	)
}

// Float64Counter creates a Float64Counter.
func (m Meter) Float64Counter(
	name string,
	options ...otelmetric.Float64CounterOption,
) otelmetric.Float64Counter {
	inst, err := m.meter.Float64Counter(name, options...)
	return instrumentOrFallback[otelmetric.Float64Counter](
		m,
		name,
		inst,
		err,
		noop.Float64Counter{},
	)
}

// Float64UpDownCounter creates a Float64UpDownCounter.
func (m Meter) Float64UpDownCounter(
	name string,
	options ...otelmetric.Float64UpDownCounterOption,
) otelmetric.Float64UpDownCounter {
	inst, err := m.meter.Float64UpDownCounter(name, options...)
	return instrumentOrFallback[otelmetric.Float64UpDownCounter](
		m, name, inst, err, noop.Float64UpDownCounter{},
	)
}

// Float64Histogram creates a Float64Histogram.
func (m Meter) Float64Histogram(
	name string,
	options ...otelmetric.Float64HistogramOption,
) otelmetric.Float64Histogram {
	inst, err := m.meter.Float64Histogram(name, options...)
	return instrumentOrFallback[otelmetric.Float64Histogram](
		m, name, inst, err, noop.Float64Histogram{},
	)
}

// Float64Gauge creates a Float64Gauge.
func (m Meter) Float64Gauge(
	name string,
	options ...otelmetric.Float64GaugeOption,
) otelmetric.Float64Gauge {
	inst, err := m.meter.Float64Gauge(name, options...)
	return instrumentOrFallback[otelmetric.Float64Gauge](m, name, inst, err, noop.Float64Gauge{})
}

// Float64ObservableCounter creates a Float64ObservableCounter.
func (m Meter) Float64ObservableCounter(
	name string,
	options ...otelmetric.Float64ObservableCounterOption,
) otelmetric.Float64ObservableCounter {
	inst, err := m.meter.Float64ObservableCounter(name, options...)
	return instrumentOrFallback[otelmetric.Float64ObservableCounter](
		m, name, inst, err, noop.Float64ObservableCounter{},
	)
}

// Float64ObservableUpDownCounter creates a Float64ObservableUpDownCounter.
func (m Meter) Float64ObservableUpDownCounter(
	name string,
	options ...otelmetric.Float64ObservableUpDownCounterOption,
) otelmetric.Float64ObservableUpDownCounter {
	inst, err := m.meter.Float64ObservableUpDownCounter(name, options...)
	return instrumentOrFallback[otelmetric.Float64ObservableUpDownCounter](
		m, name, inst, err, noop.Float64ObservableUpDownCounter{},
	)
}

// Float64ObservableGauge creates a Float64ObservableGauge.
func (m Meter) Float64ObservableGauge(
	name string,
	options ...otelmetric.Float64ObservableGaugeOption,
) otelmetric.Float64ObservableGauge {
	inst, err := m.meter.Float64ObservableGauge(name, options...)
	return instrumentOrFallback[otelmetric.Float64ObservableGauge](
		m, name, inst, err, noop.Float64ObservableGauge{},
	)
}
