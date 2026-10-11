package internal

import (
	"os"
	"testing"
)

const (
	// The environment variable to use for the connection string in tests.
	connStrEnv   = "MIGRATION_TOOLS_MONGODB_URI"
	topologyEnv  = "MIGRATION_TOOLS_MONGODB_TOPOLOGY"
	dbVersionEnv = "MIGRATION_TOOLS_MONGODB_VERSION"
)

// GetTopology returns the provisioned cluster’s topology.
func GetTopology(t *testing.T) string {
	return getEnvOrSkip(t, topologyEnv)
}

func GetDBVersion(t *testing.T) string {
	return getEnvOrSkip(t, dbVersionEnv)
}

// GetConnStr returns the provisioned cluster’s connection string.
func GetConnStr(t *testing.T) string {
	return getEnvOrSkip(t, connStrEnv)
}

func getEnvOrSkip(t *testing.T, envName string) string {
	val := os.Getenv(envName)
	if val == "" {
		t.Skipf("%#q not set", envName)
	}

	return val
}
