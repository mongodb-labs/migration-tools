// Package session exports useful tools for MongoDB sessions.
package session

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/goaux/timer"
	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

const (
	staleClusterTimeErrCode   = 209
	notWritablePrimaryErrCode = 10107
	opTimeKeyInServerResponse = "operationTime"
	dollarClusterTime         = "$clusterTime"
)

var bootstrapRequest = lo.Must(bson.Marshal(
	bson.D{
		{"appendOplogNote", 1},
		{"data", bson.D{
			{"bootstrap", true},
		}},
		{"writeConcern", bson.D{{"w", "majority"}}},
	},
))

// BootstrapCausalConsistency performs an appendOplogNote command to advance
// the cluster’s operation & cluster times. It then advances the session’s
// to match those new times.
//
// This is a simple means to causal consistency without persisting session
// state.
//
// This function internally retries the appendOplogNote command if it
// encounters a NotWritablePrimary error, which can occur if the command is
// routed to a secondary that hasn’t yet learned of the new primary. It also
// ignores StaleClusterTime errors, which can occur if any shard’s cluster time
// is >= the maxTime specified in the command. This is because such an error
// doesn’t indicate a failure of the command, but rather that the cluster time
// has already advanced past the time of the note we attempted to append.
func BootstrapCausalConsistency(
	ctx context.Context,
	sess *mongo.Session,
) error {
	var resp bson.Raw
	var err error

	for range 5 {
		resp, err = sess.Client().Database("admin").RunCommand(
			ctx,
			bootstrapRequest,
		).Raw()

		// Success? Yay.
		if err == nil {
			break
		}

		errCodes := mongo.ErrorCodes(err)

		// If any shard’s cluster time >= maxTime, the mongos will return a
		// StaleClusterTime error. This particular error doesn’t indicate a
		// failure, so we ignore it.
		if slices.Contains(errCodes, staleClusterTimeErrCode) {
			break
		}

		if slices.Contains(errCodes, notWritablePrimaryErrCode) {
			if err := timer.SleepCause(ctx, time.Second); err != nil {
				return err
			}

			continue
		}

		return fmt.Errorf("appendOplogNote: %w", err)
	}

	opTime, err := bsontools.RawLookup[bson.Timestamp](resp, opTimeKeyInServerResponse)
	if err != nil {
		return fmt.Errorf("read %q in server response: %w", opTimeKeyInServerResponse, err)
	}

	if err := sess.AdvanceOperationTime(&opTime); err != nil {
		return fmt.Errorf("advance session operation time: %w", err)
	}

	ctVal, err := resp.LookupErr(dollarClusterTime)
	if err != nil {
		return fmt.Errorf("read %q in server response: %w", dollarClusterTime, err)
	}

	// The driver’s cluster-time interfaces return & expect this wrapped form.
	clusterTime := bson.Raw(
		bsoncore.BuildDocumentFromElements(
			nil,
			bsoncore.AppendValueElement(
				nil,
				"$clusterTime",
				bsoncore.Value{
					Type: bsoncore.Type(ctVal.Type),
					Data: ctVal.Value,
				},
			),
		),
	)

	if err := sess.AdvanceClusterTime(clusterTime); err != nil {
		return fmt.Errorf("advance session cluster time: %w", err)
	}

	return nil
}
