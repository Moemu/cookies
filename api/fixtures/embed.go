// Package fixtures exposes immutable development fixtures to server-side
// domain modules without depending on the process working directory.
package fixtures

import "embed"

// Files contains the versioned JSON fixtures that are checked into this
// directory.
//
//go:embed *.json
var Files embed.FS
