// Package oteltools sets up OpenTelemetry metrics export for migration tools.
//
// It provides:
//   - Setup, which builds a metrics pipeline that serves Prometheus metrics
//     over HTTP and, optionally, writes the same metrics to a file;
//   - Meter, which wraps an otelmetric.Meter so that instrument-creation
//     failures are logged instead of returned.
//
// The OTel Prometheus exporter does all conversion from OTel to Prometheus.
// The metrics file is written from the same registry that the HTTP handler
// serves, so the two always contain the same series with the same names.
//
// # Metrics file format
//
// The metrics file has the fields of mongosync’s metrics log files that tools
// reading those files use, so those tools read this file too. It is JSON
// Lines: one JSON object per export, each on its own line, such as:
//
//	{"time":"2026-11-11T15:06:02.389Z","message":"# HELP …"}
//
// The fields are:
//   - time: when the export was gathered, in RFC 3339 format;
//   - message: one Prometheus text exposition, the same content that the
//     HTTP handler serves, except that every sample has a timestamp. The
//     timestamp is the export’s time, in Unix milliseconds.
//
// To read the metrics, decode each line and take its message. Each message
// is a complete exposition, so it can go straight to a Prometheus text
// parser or to an importer such as VictoriaMetrics’
// /api/v1/import/prometheus endpoint. Because every sample has a timestamp,
// an importer stores each sample at the time it was gathered, not the time
// it was imported.
//
// A final line that isn’t a complete JSON object is an export that was cut
// short, for example by a crash, and should be discarded.
//
// Nothing in this package alters global state. In particular, it never calls
// otel.SetMeterProvider or any other otel.Set* function, and it never
// registers with prometheus.DefaultRegisterer. Callers receive the meter
// provider and the HTTP handler and decide what to do with them.
//
// All logging goes through log/slog.
package oteltools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mongodb-labs/migration-tools/option"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
)

// DefaultExportInterval is how often metrics are written to the metrics file
// unless Settings.ExportInterval says otherwise.
const DefaultExportInterval = 30 * time.Second

// Settings configures Setup.
type Settings struct {
	// Logger receives this package’s log messages. If nil, they are
	// discarded.
	Logger *slog.Logger

	// Resource describes the process (service name, host name, and so on).
	// If nil, the provider has an empty resource. Setup never reads
	// resource attributes from the environment.
	Resource *resource.Resource

	// ResourceLabels selects the resource attributes to put as labels on
	// every series. Use an allow-list such as attribute.NewAllowKeysFilter
	// so that unexpected resource attributes don’t become labels. If nil,
	// resource attributes appear only in the target_info series.
	ResourceLabels attribute.Filter

	// MetricsFile, if set, receives one export every ExportInterval, in the
	// format that the package documentation describes. Each export is one
	// line, written in a single Write call. The pipeline owns the writer from
	// then on: Pipeline.Shutdown closes it if it implements io.Closer.
	MetricsFile option.Option[io.Writer]

	// ExportInterval is how often metrics are written to MetricsFile.
	// If zero, DefaultExportInterval is used.
	ExportInterval time.Duration

	// RuntimeCollectors adds the Prometheus client’s Go runtime (go_*) and
	// process (process_*) collectors. These are what
	// prometheus.DefaultRegisterer would otherwise provide.
	RuntimeCollectors bool
}

// Pipeline is a running metrics pipeline.
type Pipeline struct {
	// MeterProvider creates the meters whose instruments feed the pipeline.
	// It is the API interface rather than the SDK type so that callers shut
	// down through Pipeline.Shutdown, which also writes the last export to the
	// metrics file.
	MeterProvider otelmetric.MeterProvider

	// Handler serves the metrics in Prometheus text format, typically at
	// /metrics. It reads only a registry that Setup created, never the
	// Prometheus default registry.
	Handler http.Handler

	provider *sdkmetric.MeterProvider
	file     option.Option[*fileWriter]
	writer   option.Option[io.Writer]

	shutdownOnce sync.Once
	shutdownErr  error
}

// Shutdown writes the last export to the metrics file, shuts down the meter
// provider, and closes the metrics file if it implements io.Closer. Call it
// before the program exits. Later calls return the first call’s result.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		var errs []error

		// The final write gathers from the provider, so it must come before
		// the provider shuts down.
		if fw, has := p.file.Get(); has {
			if err := fw.stopAndFlush(); err != nil {
				errs = append(errs, fmt.Errorf("writing final metrics: %w", err))
			}
		}

		if err := p.provider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutting down meter provider: %w", err))
		}

		if w, has := p.writer.Get(); has {
			if closer, ok := w.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					errs = append(errs, fmt.Errorf("closing metrics file: %w", err))
				}
			}
		}

		p.shutdownErr = errors.Join(errs...)
	})

	return p.shutdownErr
}

// Setup creates a meter provider whose only reader is the OTel Prometheus
// exporter, registered on a private registry. The returned pipeline’s
// Handler serves that registry, and if settings.MetricsFile is set, a
// goroutine writes the same registry to the file every ExportInterval.
//
// Exemplars are disabled because neither output uses them, and the default
// filter costs a context lookup on every measurement.
func Setup(settings Settings) (*Pipeline, error) {
	logger := loggerOrDiscard(settings.Logger)

	registry := prometheus.NewRegistry()

	if settings.RuntimeCollectors {
		if err := registerRuntimeCollectors(registry); err != nil {
			return nil, err
		}
	}

	promOpts := []otelprometheus.Option{otelprometheus.WithRegisterer(registry)}
	if settings.ResourceLabels != nil {
		promOpts = append(
			promOpts,
			otelprometheus.WithResourceAsConstantLabels(settings.ResourceLabels),
		)
	}

	promExporter, err := otelprometheus.New(promOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus exporter: %w", err)
	}

	res := settings.Resource
	if res == nil {
		res = resource.Empty()
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithExemplarFilter(exemplar.AlwaysOffFilter),
		sdkmetric.WithReader(promExporter),
	)

	pipeline := &Pipeline{
		MeterProvider: provider,
		Handler: promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
				ErrorHandling: promhttp.ContinueOnError,
			},
		),
		provider: provider,
		writer:   settings.MetricsFile,
	}

	if writer, has := settings.MetricsFile.Get(); has {
		interval := settings.ExportInterval
		if interval == 0 {
			interval = DefaultExportInterval
		}

		pipeline.file = option.Some(startFileWriter(registry, writer, logger, interval))
	}

	return pipeline, nil
}

func registerRuntimeCollectors(registry *prometheus.Registry) error {
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	} {
		if err := registry.Register(c); err != nil {
			return fmt.Errorf("registering runtime collector: %w", err)
		}
	}

	return nil
}

func loggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.DiscardHandler)
	}

	return logger
}
