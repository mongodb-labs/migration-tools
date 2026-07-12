// Package changestream provides a parallel change stream implementation.
package changestream

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/mongodb-labs/migration-tools/contextplus"
	"github.com/mongodb-labs/migration-tools/future"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	dispatchInput = "$_id._data"
)

var (
	errCloseCalled = fmt.Errorf("close called on ParallelChangeStream")
)

// Parallel runs multiple change streams in parallel and merges
// their results into a single stream. Change event order is preserved, and
// causally-consistent sessions are updated accordingly. In many (most?)
// applications this is almost a drop-in replacement for mongo.ChangeStream.
type Parallel struct {
	channels     []chan eventsBatch
	curChanBatch []eventsBatch
	errFuture    *future.Future[error]
	nextErr      error
	current      bson.Raw
	resumeToken  bson.Raw
	canceler     context.CancelCauseFunc
}

// Options are the options for creating a Parallel.
type Options struct {
	// Streams is the number of parallel change streams to use.
	Streams int

	// Pipeline is the aggregation pipeline to apply to the change stream.
	Pipeline mongo.Pipeline

	// BatchesBufferSize is the number of change stream batches to buffer per
	// stream. The default is 10.
	BatchesBufferSize int

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
	Events               []bson.Raw
	OperationTime        bson.Timestamp
	ClusterTime          bson.Raw
	PostBatchResumeToken bson.Raw
}

// NewParallel creates a new Parallel.
func NewParallel(
	ctxIn context.Context,
	watcher Watcher,
	opts Options,
) (*Parallel, error) {
	if opts.Streams <= 0 {
		return nil, fmt.Errorf("streams (%d) must be positive", opts.Streams)
	}

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

	chanBufSize := cmp.Or(opts.BatchesBufferSize, 10)

	for threadNum := range opts.Streams {
		curChan := make(chan eventsBatch, chanBufSize)
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
			csOpts:    opts.Options,
			client:    client,
			curChan:   curChan,
			errSetter: setErr,
		})
	}

	return &Parallel{
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

// Next iterates the change stream. It blocks until the next change event is
// available, an error occurs, or the change stream is closed.
func (pcs *Parallel) Next(ctx context.Context) bool {
	return pcs.next(ctx, blockingForever)
}

// TryNext is like Next, but it will only block long enough to send a single
// `getMore` request to the server. If that response contains no events, this
// returns false.
func (pcs *Parallel) TryNext(ctx context.Context) bool {
	return pcs.next(ctx, blockingOnce)
}

// Current returns the current change event.
func (pcs *Parallel) Current() bson.Raw {
	return pcs.current
}

// Close closes the change stream. It is safe to call Close multiple times.
func (pcs *Parallel) Close() {
	pcs.canceler(errCloseCalled)
}

// Err returns whatever error, if any, happened while iterating the change
// stream. This may include errors from the underlying streams, from the
// “top-level” stream, or both.
func (pcs *Parallel) Err() error {
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

func (pcs *Parallel) ResumeToken() bson.Raw {
	return pcs.resumeToken
}

type blockingType int

const (
	blockingReturn  = 0
	blockingOnce    = 1
	blockingForever = 2
)

func (pcs *Parallel) next(
	ctx context.Context,
	blocking blockingType,
) bool {
	//sess := mongo.SessionFromContext(ctx)

	// Optimize the case where each channel has events buffered:
	nextChanEvent := make(map[int]bson.Raw, len(pcs.channels))

	emptyBatchResumeTokenData := map[int][]byte{}

	for i, batch := range pcs.curChanBatch {
		if len(batch.Events) > 0 {
			nextChanEvent[i] = batch.Events[0]
		} else if len(batch.PostBatchResumeToken) > 0 {
			data, err := getResumeTokenHexBytes(batch.PostBatchResumeToken)
			if err != nil {
				pcs.nextErr = fmt.Errorf("get resume token hex bytes for thread %d: %w", i, err)
				pcs.canceler(pcs.nextErr)
				return false
			}
			emptyBatchResumeTokenData[i] = data
		}
	}

	//fmt.Printf("------ have %d nonempty channels\n", len(nextChanEvent))

	if len(nextChanEvent) == 0 {
		if blocking == blockingReturn {
			pcs.setResumeTokenWhenEmpty()
			return false
		}

		//fmt.Printf("------ all batches empty; fetching\n")

		// There are no cached events, so we need to refresh all channels.
		chansToFetch := lo.Range(len(pcs.channels))

		if err := pcs.refreshChanBatches(ctx, chansToFetch); err != nil {
			pcs.nextErr = fmt.Errorf("refresh channel batches: %w", err)
			pcs.canceler(pcs.nextErr)
			return false
		}

		if blocking == blockingOnce {
			blocking = blockingReturn
		}

		//fmt.Printf("------ refreshed all channels; calling next again\n")

		return pcs.next(ctx, blocking)
	}

	// We have at least one cached event. First identify the next among those.

	var chanToken = map[int][]byte{}
	for i, event := range nextChanEvent {
		var err error
		rt, err := bsontools.RawLookup[bson.Raw](event, "_id")
		if err != nil {
			pcs.nextErr = fmt.Errorf("extract event’s resume token in thread %d: %w", i, err)
			pcs.canceler(pcs.nextErr)
			return false
		}

		chanToken[i], err = getResumeTokenHexBytes(rt)
		if err != nil {
			pcs.nextErr = fmt.Errorf("extract resume token’s data in thread %d: %w", i, err)
			pcs.canceler(pcs.nextErr)
			return false
		}
	}

	nextChan := lo.MinBy(lo.Keys(nextChanEvent), func(i, j int) bool {
		return bytes.Compare(chanToken[i], chanToken[j]) < 0
	})

	returnNext := func() bool {
		//fmt.Printf("------ returning next event from channel %d\n", nextChan)

		pcs.current = pcs.curChanBatch[nextChan].Events[0]
		pcs.curChanBatch[nextChan].Events = pcs.curChanBatch[nextChan].Events[1:]

		var err error
		pcs.resumeToken, err = bsontools.RawLookup[bson.Raw](pcs.current, "_id")
		if err != nil {
			pcs.nextErr = fmt.Errorf("lookup resume token for thread %d: %w", nextChan, err)
			pcs.canceler(pcs.nextErr)
			return false
		}

		return true
	}

	if len(nextChanEvent) == len(pcs.channels) {
		// All channels have events buffered, so we can pick the next event without
		// blocking.

		return returnNext()
	}

	// At least one channel has no events buffered, so we need to check if any of
	// those channels could have events earlier than the current best candidate.
	// If so, we need to block until we can get the next event from those channels.
	nextEventRT := []byte(chanToken[nextChan])

	var chansToFetch []int

	for i, rtData := range emptyBatchResumeTokenData {
		if bytes.Compare(rtData, nextEventRT) < 0 {
			//fmt.Printf("------ channel %d has empty batch with rtData=%v < nextEventRT=%v; must fetch\n", i, string(rtData), string(nextEventRT))
			chansToFetch = append(chansToFetch, i)
		}
	}

	if len(chansToFetch) == 0 {
		// All empty channels are current with the earliest-cached event.
		// Thus, the earliest event is the one cached.
		return returnNext()
	}

	switch blocking {
	case blockingReturn:
		pcs.setResumeTokenWhenEmpty()
		return false
	case blockingOnce:
		blocking = blockingReturn
	case blockingForever:
	default:
		panic(fmt.Sprintf("unexpected blocking type: %v", blocking))
	}

	if err := pcs.refreshChanBatches(ctx, chansToFetch); err != nil {
		pcs.nextErr = fmt.Errorf("refresh channel batches: %w", err)
		pcs.canceler(pcs.nextErr)
		return false
	}

	// Now that we’ve refreshed the relevant batches, we can pick the next event again.

	return pcs.next(ctx, blocking)
}

func (pcs *Parallel) refreshChanBatches(
	ctx context.Context,
	chansToFetch []int,
) error {
	// For each indicated channel, read a batch & update curChanBatch.
	//fmt.Printf("------ refreshing channels: %v\n", chansToFetch)

	chans := lo.Map(chansToFetch, func(i int, _ int) <-chan eventsBatch {
		return pcs.channels[i]
	})

	batches, err := readEachChannelOnce(ctx, chans...)
	if err != nil {
		return fmt.Errorf("read from channels: %w", err)
	}

	for i, idx := range chansToFetch {
		pcs.curChanBatch[idx] = batches[i]
	}

	return nil
}

func (pcs *Parallel) setResumeTokenWhenEmpty() {
	// Iterate the current batches and find the minimum resume token. Then
	// set that as the PCS’s resume token.
	//
	// The tokens themselves are BSON documents. We need to sort them by
	// their `_data` field, which is a hex-encoded string.

	tokens := lo.Map(
		pcs.curChanBatch,
		func(batch eventsBatch, _ int) []byte {
			return batch.PostBatchResumeToken
		},
	)

	tokenData := make([][]byte, 0, len(tokens))

	for _, token := range tokens {
		tokenD, err := getResumeTokenHexBytes(token)
		if err != nil {
			pcs.nextErr = fmt.Errorf("lookup resume token data: %w", err)
			pcs.canceler(pcs.nextErr)
			return
		}

		tokenData = append(tokenData, tokenD)
	}

	nextTokenIdx := lo.MinBy(
		lo.Range(len(tokens)),
		func(a, b int) bool {
			return bytes.Compare(tokenData[a], tokenData[b]) < 0
		},
	)

	pcs.resumeToken = tokens[nextTokenIdx]
}
