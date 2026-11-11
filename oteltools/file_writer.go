package oteltools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
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

// fileWriter periodically writes a gatherer’s metrics to a file. Because it
// reads the same registry that the HTTP handler serves, the file always
// matches the handler’s output.
type fileWriter struct {
	gatherer prometheus.Gatherer
	writer   io.Writer
	logger   *slog.Logger

	stop chan struct{}
	done chan struct{}
}

func startFileWriter(
	gatherer prometheus.Gatherer,
	writer io.Writer,
	logger *slog.Logger,
	interval time.Duration,
) *fileWriter {
	fw := &fileWriter{
		gatherer: gatherer,
		writer:   writer,
		logger:   logger,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	go fw.run(interval)

	return fw
}

func (fw *fileWriter) run(interval time.Duration) {
	defer close(fw.done)

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := fw.writeOnce(); err != nil {
				fw.logger.Warn("Failed to write metrics to file.", "error", err)
			}
		case <-fw.stop:
			return
		}

		timer.Reset(interval)
	}
}

// stopAndFlush stops the periodic writes and then writes one last export, so
// that the file includes everything recorded up to now.
func (fw *fileWriter) stopAndFlush() error {
	close(fw.stop)
	<-fw.done

	return fw.writeOnce()
}

// writeOnce gathers the current metrics and writes them as one export, on
// one line. It writes nothing if there are no metrics.
//
// The line is written in a single Write call, so a rotating writer never
// splits it across files.
func (fw *fileWriter) writeOnce() error {
	families, err := fw.gatherer.Gather()
	if err != nil {
		// Gather returns whatever it could collect along with the error,
		// so write that rather than nothing.
		fw.logger.Warn("Failed to gather some metrics for the metrics file.", "error", err)
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
			fw.logger.Warn(
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

	wrote, err := fw.writer.Write(line.Bytes())

	if err != nil {
		return fmt.Errorf("writing metrics export: %w", err)
	}

	if wrote != line.Len() {
		return fmt.Errorf("incomplete write: only %d of %d bytes", wrote, line.Len())
	}

	return nil
}
