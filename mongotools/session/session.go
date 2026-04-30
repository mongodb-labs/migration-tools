// Package session exports useful tools for MongoDB sessions.
package session

import (
	"context"
	"fmt"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

const (
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
// the cluster’s operation & cluster times. It then advances the given session
// to match those new times.
//
// This is a simple path to causal consistency across application restarts.
// It works with all 4.2+ clusters as well as 4.0 replica sets.
//
// NB: This does not retry if `appendOplogNote` fails. Since that command
// may fail transiently if the cluster is under load or experiencing failover,
// applications should apply their own retry logic around this function.
func BootstrapCausalConsistency(
	ctx context.Context,
	sess *mongo.Session,
) error {
	resp, err := sess.Client().Database("admin").RunCommand(
		ctx,
		bootstrapRequest,
	).Raw()
	if err != nil {
		return fmt.Errorf("appendOplogNote: %w", err)
	}

	opTime, err := bsontools.RawLookup[bson.Timestamp](resp, opTimeKeyInServerResponse)
	if err != nil {
		return fmt.Errorf("read %#q in server response: %w", opTimeKeyInServerResponse, err)
	}

	if err := sess.AdvanceOperationTime(&opTime); err != nil {
		return fmt.Errorf("advance session operation time: %w", err)
	}

	ctVal, err := resp.LookupErr(dollarClusterTime)
	if err != nil {
		return fmt.Errorf("read %#q in server response: %w", dollarClusterTime, err)
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
