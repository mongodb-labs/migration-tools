package oteltools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mongodb-labs/migration-tools/contextplus"
	"github.com/mongodb-labs/migration-tools/future"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// exporterBackend writes a gatherer’s metrics to a file, as the goroutine that
// fileWriter controls.
type exporterBackend struct {
	gatherer prometheus.Gatherer
	writer   io.Writer
	logger   *slog.Logger
}

// run writes an export every interval until stopCtx is set. It then writes
// one last export, unless the stop context has already ended, and sets the
// result.
func (eb *exporterBackend) run(
	interval time.Duration,
	stopCtx *future.Future[context.Context],
	setResult future.Setter[error],
) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := eb.gatherAndWriteOnce(); err != nil {
				eb.logger.Warn("Failed to write metrics to file.", "error", err)
			}
		case <-stopCtx.Ready():
			ctx := stopCtx.Get()

			// If ctx has ended, stopAndFlush has stopped waiting, so nobody
			// expects the last export anymore.
			if ctx.Err() != nil {
				setResult(contextplus.New(ctx).Err())
			} else {
				setResult(eb.gatherAndWriteOnce())
			}

			return
		}

		timer.Reset(interval)
	}
}

// gatherAndWriteOnce gathers the current metrics and writes them as one export, on
// one line. It writes nothing if there are no metrics.
//
// The line is written in a single Write call, so a rotating writer never
// splits it across files.
func (eb *exporterBackend) gatherAndWriteOnce() error {
	families, err := eb.gatherer.Gather()
	if err != nil {
		// Gather returns whatever it could collect along with the error,
		// so write that rather than nothing.
		eb.logger.Warn("Failed to gather some metrics for the metrics file.", "error", err)
	}

	if len(families) == 0 {
		return nil
	}

	// Scraped metrics normally carry no timestamps because the scraper
	// stamps them. A file is read long after the fact, so stamp every
	// sample with the time it was gathered.
	nowMS := time.Now().UnixMilli()
	for _, family := range families {
		for _, m := range family.GetMetric() {
			m.TimestampMs = new(nowMS)
		}
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(
		&buf,
		expfmt.NewFormat(expfmt.TypeTextPlain).WithEscapingScheme(model.UnderscoreEscaping),
	)

	for _, family := range families {
		if err := encoder.Encode(family); err != nil {
			eb.logger.Warn(
				"Failed to encode metric for the metrics file.",
				"metric", family.GetName(),
				"error", err,
			)
		}
	}

	if buf.Len() == 0 {
		return nil
	}

	// As of this writing json.Encoder always writes a single line per Encode
	// call. That’s undocumented behavior, though. To avoid reliance on that we
	// use our own buffer.
	var line bytes.Buffer

	lineEncoder := json.NewEncoder(&line)

	// Keep the exposition readable: there is no HTML here to protect.
	lineEncoder.SetEscapeHTML(false)

	// Encode also ends the line with a newline.
	err = lineEncoder.Encode(exportLine{
		Time:    time.UnixMilli(nowMS).UTC().Format(time.RFC3339Nano),
		Message: buf.String(),
	})
	if err != nil {
		return fmt.Errorf("encoding metrics export as JSON: %w", err)
	}

	wrote, err := eb.writer.Write(line.Bytes())

	if err != nil {
		return fmt.Errorf("writing metrics export: %w", err)
	}

	if wrote != line.Len() {
		return fmt.Errorf("incomplete write: only %d of %d bytes", wrote, line.Len())
	}

	return nil
}
