package oteltools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mongodb-labs/migration-tools/contextplus"
	"github.com/mongodb-labs/migration-tools/future"
	"github.com/prometheus/client_golang/prometheus"
)

// exportLine is one line of a metrics file: one export, as a JSON object. Its
// fields are the ones that tools reading mongosync’s metrics log files use,
// so those tools read these files too.
type exportLine struct {
	// Time is when the metrics were gathered, in RFC 3339 format. It is the
	// same instant as every sample’s timestamp in Message.
	Time string `json:"time"`

	// Message is one Prometheus text exposition.
	Message string `json:"message"`
}

// exporter is the handle on a goroutine that periodically writes a
// gatherer’s metrics to a file. Because the goroutine reads the same registry
// that the HTTP handler serves, the file always matches the handler’s output.
type exporter struct {
	// setStopCtx tells the goroutine to stop. Its context is the one that
	// stopAndFlush received.
	setStopCtx future.Setter[context.Context]

	// result is the final write’s error, set when the goroutine exits.
	result *future.Future[error]
}

func startFileWriter(
	gatherer prometheus.Gatherer,
	writer io.Writer,
	logger *slog.Logger,
	interval time.Duration,
) *exporter {
	stopCtx, setStopCtx := future.New[context.Context]()
	result, setResult := future.New[error]()

	exporterBackend := &exporterBackend{
		gatherer: gatherer,
		writer:   writer,
		logger:   logger,
	}

	go exporterBackend.run(interval, stopCtx, setResult)

	return &exporter{
		setStopCtx: setStopCtx,
		result:     result,
	}
}

// stopAndFlush stops the periodic writes and then writes one last export, so
// that the file includes everything recorded up to now. Call it only once.
//
// A Write can’t be canceled, so if ctx ends before the writes finish,
// stopAndFlush returns without waiting for them. A Write may then still be in
// progress, and the goroutine skips the last export unless it has already
// begun.
func (fw *exporter) stopAndFlush(ctx context.Context) error {
	fw.setStopCtx(ctx)

	select {
	case <-fw.result.Ready():
		return fw.result.Get()
	case <-ctx.Done():
		return fmt.Errorf("awaiting last write: %w", contextplus.New(ctx).Err())
	}
}
