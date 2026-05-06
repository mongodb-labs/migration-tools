// Package scripts embeds the evg-cache runtime scripts into the binary.
package scripts

import "embed"

//go:embed data/*
var FS embed.FS
