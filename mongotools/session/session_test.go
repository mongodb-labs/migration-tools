package session

import (
	"context"
	"testing"
	"time"

	"github.com/mongodb-labs/migration-tools/internal"
	"github.com/mongodb-labs/migration-tools/legacytools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

func TestIntegration_BootstrapCausalConsistency(t *testing.T) {
	legacytools.SetDriverCompatibility("4.0")

	if internal.GetTopology(t) == "sharded" && internal.GetDBVersion(t) == "4.0" {
		t.Skip("4.0 sharded clusters lacked appendOplogNote.")
	}

	ctx := t.Context()

	uri := internal.GetConnStr(t)

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err, "connect client")

	db := client.Database(t.Name())
	defer func() { assert.NoError(t, db.Drop(ctx)) }()

	coll := db.Collection(
		"stuff",
		options.Collection().
			SetReadConcern(readconcern.Majority()).
			SetWriteConcern(writeconcern.Majority()),
	)

	// Do these individually on purpose so as to trip causal-consistency if it’s not working.
	for i := range 100 {
		_, err = coll.InsertOne(ctx, bson.D{{"_id", i}})
		require.NoError(t, err, "insert document")
	}

	// Don’t check this for success since in old server versions it failed.
	_ = client.Database("admin").RunCommand(
		ctx,
		bson.D{
			{"replSetStepDown", 1},
			{"force", true},
		},
		options.RunCmd().SetReadPreference(readpref.Primary()),
	)

	sess, err := client.StartSession()
	require.NoError(t, err, "start session")

	defer sess.EndSession(ctx)

	require.Eventually(
		t,
		func() bool {
			err := BootstrapCausalConsistency(ctx, sess)

			if err == nil {
				return true
			}

			t.Logf("Bootstrap attempt failed (%v); may retry", err)
			return false
		},
		time.Minute,
		time.Second,
		"bootstrap should eventually succeed",
	)

	assert.NotZero(
		t,
		sess.OperationTime(),
		"session should have an operation time after bootstrapping",
	)
	assert.NotZero(t, sess.ClusterTime(), "session should have a cluster time after bootstrapping")

	// If the session is causally consistent, it should see all 100 documents. If not, it may see fewer.
	err = mongo.WithSession(ctx, sess, func(ctx context.Context) error {
		count, err := coll.CountDocuments(ctx, bson.D{})
		if err != nil {
			return err
		}

		assert.Equal(t, int64(100), count, "session should see all 100 documents")

		return nil
	})
	require.NoError(t, err, "use session")
}
