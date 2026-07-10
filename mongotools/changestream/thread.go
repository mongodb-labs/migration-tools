package changestream

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type threadConfig struct {
	watcher   Watcher
	threadNum int
	pipeline  mongo.Pipeline
	csOpts    options.Lister[options.ChangeStreamOptions]
	client    *mongo.Client
	curChan   chan<- eventsBatch
	errSetter func(error)
}

// runChangeStreamThread runs the loop for a single change stream thread.
// It reads events from the change stream and sends them to the channel.
func runChangeStreamThread(ctx context.Context, cfg threadConfig) {
	defer close(cfg.curChan)

	csOpts := cfg.csOpts
	if csOpts == nil {
		csOpts = options.ChangeStream()
	}

	sess, err := cfg.client.StartSession(options.Session().SetCausalConsistency(true))
	if err != nil {
		cfg.errSetter(fmt.Errorf("start thread %d’s session: %w", cfg.threadNum, err))
		return
	}
	sctx := mongo.NewSessionContext(ctx, sess)
	defer sess.EndSession(sctx)

	cs, err := cfg.watcher.Watch(sctx, cfg.pipeline, csOpts)
	if err != nil {
		cfg.errSetter(fmt.Errorf("open thread %d’s change stream: %w", cfg.threadNum, err))
		return
	}
	defer cs.Close(sctx)

	var events []bson.Raw
	for {
		if cs.TryNext(sctx) {
			events = append(events, slices.Clone(cs.Current))
		} else if err := cs.Err(); err != nil {
			cfg.errSetter(fmt.Errorf("read thread %d’s change stream: %w", cfg.threadNum, err))
			return
		}

		if cs.RemainingBatchLength() == 0 {
			myEvents := events
			if len(myEvents) > 0 {
				myEvents = slices.Clone(myEvents)
			}

			batch := eventsBatch{
				Events:               myEvents,
				OperationTime:        lo.FromPtr(sess.OperationTime()),
				ClusterTime:          sess.ClusterTime(),
				PostBatchResumeToken: cs.ResumeToken(),
			}

			if !cfg.sendBatch(sctx, batch) {
				return
			}

			// Reset/reuse the events slice.
			events = events[:0]
		}
	}
}

// sendBatch sends a batch to the channel, or records an error if the context
// is done first. Returns false when the thread should exit.
func (cfg threadConfig) sendBatch(ctx context.Context, batch eventsBatch) bool {
	select {
	case <-ctx.Done():
		cfg.errSetter(ctx.Err())
		return false
	case cfg.curChan <- batch:
		return true
	}
}
