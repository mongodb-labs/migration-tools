// Package changestream provides a parallel change stream implementation.
package changestream

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/mongodb-labs/migration-tools/contextplus"
	"github.com/mongodb-labs/migration-tools/future"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	errCloseCalled = fmt.Errorf("close called on ParallelChangeStream")
)

// ParallelChangeStream runs multiple change streams in parallel and merges
// their results into a single stream. Change event order is preserved, and
// causally-consistent sessions are updated accordingly.
type ParallelChangeStream struct {
	channels     []chan eventsBatch
	curChanBatch []eventsBatch
	errFuture    *future.Future[error]
	nextErr      error
	current      bson.Raw
	resumeToken  bson.Raw
	canceler     context.CancelCauseFunc
}

// Options are the options for creating a ParallelChangeStream.
type Options struct {
	// Streams is the number of parallel change streams to use.
	Streams int

	// Pipeline is the aggregation pipeline to apply to the change stream.
	Pipeline mongo.Pipeline

	// DispatchRef identifies the field in the change event used to dispatch a
	// given event to a stream. If not provided, the default is `$_id`.
	DispatchRef any

	// Options are the change stream options to use.
	Options options.Lister[options.ChangeStreamOptions]
}

// Watcher abstracts a struct that can create a change stream.
type Watcher interface {
	Watch(ctx context.Context, pipeline any,
		opts ...options.Lister[options.ChangeStreamOptions],
	) (*mongo.ChangeStream, error)
}

var _ Watcher = (*mongo.Collection)(nil)

type collLike interface {
	Database() *mongo.Database
}

type dbLike interface {
	Client() *mongo.Client
}

type eventsBatch struct {
	Events        []bson.Raw
	OperationTime bson.Timestamp
	ClusterTime   bson.Raw
	ResumeToken   bson.Raw
}

type threadConfig struct {
	watcher   Watcher
	threadNum int
	pipeline  mongo.Pipeline
	csOpts    options.Lister[options.ChangeStreamOptions]
	client    *mongo.Client
	curChan   chan<- eventsBatch
	setErr    func(error)
}

// NewParallel creates a new ParallelChangeStream.
func NewParallel(
	ctxIn context.Context,
	watcher Watcher,
	opts Options,
) (*ParallelChangeStream, error) {
	if opts.Streams <= 0 {
		return nil, fmt.Errorf("streams (%d) must be positive", opts.Streams)
	}

	dispatchInput := cmp.Or(opts.DispatchRef, "$_id._data")

	errFuture, errSetter := future.New[error]()
	errIsSet := &atomic.Bool{}
	setErr := func(err error) {
		if errIsSet.CompareAndSwap(false, true) {
			errSetter(err)
		}
	}

	channels := make([]chan eventsBatch, opts.Streams)
	ctx, canceler := contextplus.WithCancelCause(ctxIn)

	var client *mongo.Client
	switch w := watcher.(type) {
	case collLike:
		client = w.Database().Client()
	case dbLike:
		client = w.Client()
	case *mongo.Client:
		client = w
	default:
		panic(fmt.Sprintf("watcher type (%T) is unexpected", watcher))
	}

	for threadNum := range opts.Streams {
		curChan := make(chan eventsBatch, 10)
		channels[threadNum] = curChan
		go runChangeStreamThread(ctx, threadConfig{
			watcher:   watcher,
			threadNum: threadNum,
			pipeline: createPipeline(
				threadNum,
				opts.Streams,
				dispatchInput,
				opts.Pipeline,
			),
			csOpts:  opts.Options,
			client:  client,
			curChan: curChan,
			setErr:  setErr,
		})
	}

	return &ParallelChangeStream{
		channels:     channels,
		curChanBatch: make([]eventsBatch, opts.Streams),
		errFuture:    errFuture,
		canceler:     canceler,
	}, nil
}

func createPipeline(
	threadNum int,
	streams int,
	dispatchInput any,
	userPipeline mongo.Pipeline,
) mongo.Pipeline {
	return lo.Concat(
		mongo.Pipeline{
			{
				{"$match", bson.D{
					{"$expr", bson.D{
						{"$eq", bson.A{
							threadNum,
							bson.D{{"$abs", bson.D{
								{"$mod", bson.A{
									bson.D{{"$toHashedIndexKey", dispatchInput}},
									streams,
								}},
							}}},
						}},
					}},
				}},
			},
		},
		userPipeline,
	)
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
			}) {
				return
			}
			clear(events)
			events = events[:0]
		}
	}
}

// Next iterates the change stream. It blocks until the next change event is
// available, an error occurs, or the change stream is closed.
func (pcs *ParallelChangeStream) Next(ctx context.Context) bool {
	return pcs.next(ctx, true)
}

// TryNext is like Next, but it will only block long enough to send a single
// `getMore` request to the server. If that response contains no events, this
// returns false.
func (pcs *ParallelChangeStream) TryNext(ctx context.Context) bool {
	return pcs.next(ctx, false)
}

// Current returns the current change event.
func (pcs *ParallelChangeStream) Current() bson.Raw {
	return pcs.current
}

// Close closes the change stream. It is safe to call Close multiple times.
func (pcs *ParallelChangeStream) Close() {
	pcs.canceler(errCloseCalled)
}

// Err returns whatever error, if any, happened while iterating the change
// stream. This may include errors from the underlying streams or from the
// "top-level" stream (or both).
func (pcs *ParallelChangeStream) Err() error {
	nextErr := pcs.nextErr

	var threadErr error
	select {
	case <-pcs.errFuture.Ready():
		threadErr = pcs.errFuture.Get()
		if errors.Is(threadErr, errCloseCalled) {
			threadErr = nil
		}
	default:
	}

	if nextErr != nil {
		if threadErr != nil {
			return fmt.Errorf("thread error: %w; iteration error: %w", threadErr, nextErr)
		}
		return nextErr
	}

	return threadErr
}

func (pcs *ParallelChangeStream) ResumeToken() bson.Raw {
	return pcs.resumeToken
}

func (pcs *ParallelChangeStream) advanceSession(sess *mongo.Session, batch eventsBatch) bool {
	if sess == nil {
		return true
	}
	if err := sess.AdvanceOperationTime(&batch.OperationTime); err != nil {
		pcs.nextErr = err
		pcs.canceler(err)
		return false
	}
	if err := sess.AdvanceClusterTime(batch.ClusterTime); err != nil {
		pcs.nextErr = err
		pcs.canceler(err)
		return false
	}
	return true
}

func (pcs *ParallelChangeStream) fillBatch(
	ctx context.Context,
	i int,
	sess *mongo.Session,
) bool {
	if len(pcs.curChanBatch[i].Events) == 0 {
		select {
		case <-ctx.Done():
			pcs.nextErr = ctx.Err()
			pcs.canceler(pcs.nextErr)
			return false
		case batch, ok := <-pcs.channels[i]:
			if !ok {
				pcs.nextErr = fmt.Errorf("channel %d closed unexpectedly", i)
				pcs.canceler(pcs.nextErr)
				return false
			}

			if !pcs.advanceSession(sess, batch) {
				return false
			}

			pcs.curChanBatch[i] = batch
		}
	}

	return len(pcs.curChanBatch[i].Events) > 0
}

func (pcs *ParallelChangeStream) pickAndConsume(chansWithEvents []int, chanToken [][]byte) error {
	nextChan := lo.MinBy(chansWithEvents, func(i, j int) bool {
		return bytes.Compare(chanToken[i], chanToken[j]) < 0
	})

	pcs.current = pcs.curChanBatch[nextChan].Events[0]
	pcs.curChanBatch[nextChan].Events = pcs.curChanBatch[nextChan].Events[1:]

	var err error
	pcs.resumeToken, err = bsontools.RawLookup[bson.Raw](pcs.current, "_id")
	if err != nil {
		return fmt.Errorf("lookup resume token for thread %d: %w", nextChan, err)
	}

	return nil
}

func (pcs *ParallelChangeStream) next(ctx context.Context, blocking bool) bool {
	chanToken := make([][]byte, len(pcs.channels))
	chansWithEvents := make([]int, 0, len(pcs.channels))
	sess := mongo.SessionFromContext(ctx)

	for len(chansWithEvents) == 0 {
		for i := range pcs.channels {
			if !pcs.fillBatch(ctx, i, sess) {
				continue
			}

			token, err := pcs.curChanBatch[i].Events[0].LookupErr(
				"_id",
				"_data",
			)
			if err != nil {
				pcs.nextErr = fmt.Errorf("lookup resume token for thread %d: %w", i, err)
				pcs.canceler(pcs.nextErr)
				return false
			}
			if token.Type != bson.TypeString {
				pcs.nextErr = fmt.Errorf("resume token for thread %d is %s not %s", i, token.Type, bson.TypeString)
				pcs.canceler(pcs.nextErr)
				return false
			}

			chanToken[i] = token.Value
			chansWithEvents = append(chansWithEvents, i)
		}

		if !blocking && len(chansWithEvents) == 0 {
			tokens := lo.Map(
				pcs.curChanBatch,
				func(batch eventsBatch, _ int) []byte {
					return batch.ResumeToken
				},
			)

			tokenData := make([]string, 0, len(tokens))

			for _, token := range tokens {
				tokenD, err := bsontools.RawLookup[string](
					token,
					"_id",
					"_data",
				)
				if err != nil {
					pcs.nextErr = fmt.Errorf("lookup resume token: %w", err)
					pcs.canceler(pcs.nextErr)
					return false
				}

				tokenData = append(tokenData, tokenD)
			}

			nextTokenIdx := lo.MinBy(
				lo.Range(len(tokens)),
				func(a, b int) bool {
					return cmp.Compare(tokenData[a], tokenData[b]) < 0
				},
			)

			pcs.resumeToken = tokens[nextTokenIdx]

			return false
		}
	}

	err := pcs.pickAndConsume(chansWithEvents, chanToken)
	if err != nil {
		pcs.nextErr = err
		pcs.canceler(pcs.nextErr)
		return false
	}

	return true
}
