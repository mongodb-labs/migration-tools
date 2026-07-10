package internal

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// The environment variable to use for the connection string in tests.
	connStrEnv   = "MIGRATION_TOOLS_MONGODB_URI"
	topologyEnv  = "MIGRATION_TOOLS_MONGODB_TOPOLOGY"
	dbVersionEnv = "MIGRATION_TOOLS_MONGODB_VERSION"
)

// GetTopology returns the provisioned cluster's topology: "replset", "sharded", or "standalone".
// If the environment variable is set, it is used directly. Otherwise the topology is detected
// from the server.
func GetTopology(t *testing.T) string {
	t.Helper()
	if val := os.Getenv(topologyEnv); val != "" {
		return val
	}

	client := connectForTest(t)

	var result bson.M
	if err := client.Database("admin").RunCommand(t.Context(), bson.D{{"hello", 1}}).Decode(&result); err != nil {
		t.Fatalf("hello command: %v", err)
	}

	if msg, _ := result["msg"].(string); msg == "isdbgrid" {
		return "sharded"
	}
	if result["setName"] != nil {
		return "replset"
	}
	return "standalone"
}

// GetDBVersion returns the server's major.minor version string (e.g. "4.2", "8.0").
// If the environment variable is set, it is used directly. Otherwise the version is detected
// from the server.
func GetDBVersion(t *testing.T) string {
	t.Helper()
	if val := os.Getenv(dbVersionEnv); val != "" {
		return val
	}

	client := connectForTest(t)

	var buildInfo bson.M
	if err := client.Database("admin").RunCommand(t.Context(), bson.D{{"buildInfo", 1}}).Decode(&buildInfo); err != nil {
		t.Fatalf("buildInfo command: %v", err)
	}

	version, _ := buildInfo["version"].(string)
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		t.Fatalf("unexpected version format: %q", version)
	}
	return parts[0] + "." + parts[1]
}

// GetConnStr returns the provisioned cluster's connection string.
func GetConnStr(t *testing.T) string {
	return getEnvOrSkip(t, connStrEnv)
}

func connectForTest(t *testing.T) *mongo.Client {
	t.Helper()
	uri := GetConnStr(t)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect to %q: %v", uri, err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Logf("disconnect: %v", err)
		}
	})
	return client
}

func getEnvOrSkip(t *testing.T, envName string) string {
	val := os.Getenv(envName)
	if val == "" {
		t.Skipf("%#q not set", envName)
	}

	return val
}
