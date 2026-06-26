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
	setErr    func(error)
}

// sendBatch sends a batch to the channel, or records an error if the context
// is done first. Returns false when the thread should exit.
func (cfg threadConfig) sendBatch(ctx context.Context, batch eventsBatch) bool {
	select {
	case <-ctx.Done():
		cfg.setErr(ctx.Err())
		return false
	case cfg.curChan <- batch:
		return true
	}
}

func runChangeStreamThread(ctx context.Context, cfg threadConfig) {
	defer close(cfg.curChan)

	csOpts := cfg.csOpts
	if csOpts == nil {
		csOpts = options.ChangeStream()
	}

	sess, err := cfg.client.StartSession(options.Session().SetCausalConsistency(true))
	if err != nil {
		cfg.setErr(fmt.Errorf("start session for thread %d: %w", cfg.threadNum, err))
		return
	}
	sctx := mongo.NewSessionContext(ctx, sess)
	defer sess.EndSession(sctx)

	cs, err := cfg.watcher.Watch(sctx, cfg.pipeline, csOpts)
	if err != nil {
		cfg.setErr(fmt.Errorf("watch change stream for thread %d: %w", cfg.threadNum, err))
		return
	}
	defer cs.Close(sctx)

	var events []bson.Raw
	for {
		if !cs.TryNext(sctx) {
			if err := cs.Err(); err != nil {
				cfg.setErr(fmt.Errorf("change stream error for thread %d: %w", cfg.threadNum, err))
				return
			}
			if !cfg.sendBatch(sctx, eventsBatch{
				OperationTime: lo.FromPtr(sess.OperationTime()),
				ClusterTime:   sess.ClusterTime(),
				ResumeToken:   cs.ResumeToken(),
			}) {
				return
			}
			continue
		}

		events = append(events, slices.Clone(cs.Current))

		if cs.RemainingBatchLength() == 0 {
			if !cfg.sendBatch(sctx, eventsBatch{
				Events:        slices.Clone(events),
				OperationTime: lo.FromPtr(sess.OperationTime()),
				ClusterTime:   sess.ClusterTime(),
				ResumeToken:   cs.ResumeToken(),
			}) {
				return
			}
			clear(events)
			events = events[:0]
		}
	}
}
