// Package app hosts process-wide constants for little-timer.
//
// Version, BuildTime, and GitCommit are stamped at link time so the
// CLI's `version` subcommand and any future `/api/version` handler
// report a single source of truth.  BuildTime is captured at init
// (not via `go build -ldflags`) so a `go run` or `make build` produces
// a meaningful timestamp without extra flags.
package app

import (
	"os/exec"
	"strings"
	"time"
)

// Version is the human-readable release tag.  Bumped on every release;
// matches the `version` field in cliff.toml.
const Version = "0.1.0"

// BuildTime is captured when the package is initialised.  Resolves to
// the time the binary was loaded — close enough to the build time for
// the purpose of "when was this binary assembled".
var BuildTime = time.Now().UTC().Format(time.RFC3339)

// GitCommit is the abbreviated HEAD commit hash.  Read once at init
// via `git rev-parse --short HEAD`; falls back to "unknown" when the
// binary is built outside a git working copy (release tarballs, Docker
// COPY-from-context, etc.).
//
// The command is run read-only — it never modifies the working tree.
var GitCommit = resolveGitCommit()

// resolveGitCommit shells out to git and returns the short HEAD hash.
// Best-effort: any failure returns "unknown" so the binary still
// boots in non-git environments.
func resolveGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	h := strings.TrimSpace(string(out))
	if h == "" {
		return "unknown"
	}
	return h
}
