package internal

import (
	"os"
	"testing"
)

const (
	// The environment variable to use for the connection string in tests.
	connStrEnv = "MIGRATION_TOOLS_MONGODB_URI"
)

func GetConnStr(t *testing.T) string {
	cs := os.Getenv(connStrEnv)
	if cs == "" {
		t.Skipf("%#q not set", connStrEnv)
	}

	return cs
}
