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

// streamExporter is the handle on a goroutine that periodically writes a
// gatherer’s metrics to Settings.MetricsOut. Because the goroutine reads the
// same registry that the HTTP handler serves, the output always matches the
// handler’s.
type streamExporter struct {
	// setStopCtx tells the goroutine to stop. Its context is the one that
	// stopAndFlush received.
	setStopCtx future.Setter[context.Context]

	// result is the final write’s error, set when the goroutine exits.
	result *future.Future[error]
}

func startStreamExporter(
	gatherer prometheus.Gatherer,
	writer io.Writer,
	logger *slog.Logger,
	interval time.Duration,
) *streamExporter {
	stopCtx, setStopCtx := future.New[context.Context]()
	result, setResult := future.New[error]()

	worker := &exportWorker{
		gatherer: gatherer,
		writer:   writer,
		logger:   logger,
	}

	go worker.run(interval, stopCtx, setResult)

	return &streamExporter{
		setStopCtx: setStopCtx,
		result:     result,
	}
}

// stopAndFlush stops the periodic writes and then writes one last export, so
// that the output includes everything recorded up to now. Call it only once.
//
// A Write can’t be canceled, so if ctx ends before the writes finish,
// stopAndFlush returns without waiting for them. A Write may then still be in
// progress, and the goroutine skips the last export unless it has already
// begun.
func (se *streamExporter) stopAndFlush(ctx context.Context) error {
	se.setStopCtx(ctx)

	select {
	case <-se.result.Ready():
		return se.result.Get()
	case <-ctx.Done():
		return fmt.Errorf("awaiting last write: %w", contextplus.New(ctx).Err())
	}
}
