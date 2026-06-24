package changestream

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mongodb-labs/migration-tools/internal"
	"github.com/mongodb-labs/migration-tools/legacytools"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestIntegration_EventOrdering(t *testing.T) {
	legacytools.SetDriverCompatibility("4.0")

	ctx := t.Context()

	uri := internal.GetConnStr(t)

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	// Unique DB name to avoid cross-test interference.
	dbName := fmt.Sprintf("test_cs_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	t.Cleanup(func() { _ = db.Drop(context.Background()) })

	// Detect server version to know whether "create" change events are emitted
	// (MongoDB 6.0+).
	var buildInfo bson.M
	require.NoError(t, client.Database("admin").RunCommand(ctx, bson.D{{"buildInfo", 1}}).Decode(&buildInfo))
	serverVersion := buildInfo["version"].(string)
	majorVersion, _ := strconv.Atoi(strings.SplitN(serverVersion, ".", 2)[0])
	supportsExpandedEvents := majorVersion >= 6

	// Capture the cluster time now so that SetStartAtOperationTime can be used
	// below. This ensures the change stream sees events even if the Watch cursor
	// is established after the operations fire.
	sess, err := client.StartSession(options.Session().SetCausalConsistency(true))
	require.NoError(t, err)
	defer sess.EndSession(ctx)
	sctx := mongo.NewSessionContext(ctx, sess)

	require.NoError(t, client.Ping(sctx, nil))
	t.Log("Connected to MongoDB cluster.")

	startTime := sess.OperationTime()
	require.NotNil(t, startTime)

	t.Logf("Reading change events from cluster time %v", startTime)

	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	csOpts := options.ChangeStream().
		SetStartAtOperationTime(startTime)

	if supportsExpandedEvents {
		csOpts.SetShowExpandedEvents(true)
	}

	pcs, err := NewParallel(tctx, db, Options{
		Streams: 1,
		Options: csOpts,
	})
	require.NoError(t, err)
	defer pcs.Close()

	const (
		collName        = "docs"
		renamedCollName = "docs_renamed"
	)

	coll := db.Collection(collName)

	// create
	require.NoError(t, db.CreateCollection(ctx, collName))

	// insert x2
	insertRes, err := coll.InsertMany(ctx, []any{
		bson.D{{"x", 1}},
		bson.D{{"x", 2}},
	})
	require.NoError(t, err)
	id1, id2 := insertRes.InsertedIDs[0], insertRes.InsertedIDs[1]

	// update
	_, err = coll.UpdateOne(ctx,
		bson.D{{"_id", id1}},
		bson.D{{"$set", bson.D{{"x", 10}}}},
	)
	require.NoError(t, err)

	// replace
	_, err = coll.ReplaceOne(ctx,
		bson.D{{"_id", id2}},
		bson.D{{"x", 20}},
	)
	require.NoError(t, err)

	// delete
	_, err = coll.DeleteOne(ctx, bson.D{{"_id", id1}})
	require.NoError(t, err)

	// rename (admin command; works for unsharded collections)
	require.NoError(t, client.Database("admin").RunCommand(ctx, bson.D{
		{"renameCollection", dbName + "." + collName},
		{"to", dbName + "." + renamedCollName},
	}).Err())

	// drop
	require.NoError(t, db.Collection(renamedCollName).Drop(ctx))

	expandedEventTypes := mapset.NewSet("create", "rename")

	// Build the expected event-type sequence.
	expected := []string{"create", "insert", "insert", "update", "replace", "delete", "rename", "drop"}
	if !supportsExpandedEvents {
		expected = lo.Filter(expected, func(opType string, _ int) bool {
			return !expandedEventTypes.Contains(opType)
		})
	}

	got := make([]string, 0, len(expected))
	for len(got) < len(expected) {
		require.True(t, pcs.Next(tctx), "change stream stopped early: %v", pcs.Err())
		got = append(got, pcs.Current().Lookup("operationType").StringValue())
	}

	require.NoError(t, pcs.Err())
	require.Equal(t, expected, got)
}
