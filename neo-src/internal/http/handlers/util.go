package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// pathID extracts the trailing integer ID from a Gin route after the
// supplied prefix.  Mirrors Zig `parsePathId(path, prefix)`.
func pathID(c *gin.Context, prefix string) (int64, error) {
	full := c.Param("id")
	if full == "" {
		return 0, errInvalidID
	}
	return strconv.ParseInt(full, 10, 64)
}

// pathIDWithSuffix extracts the integer ID between a prefix and a
// suffix (e.g. "/api/habits/:id/detail").  Mirrors Zig
// `parsePathIdWithSuffix(path, prefix, suffix)`.
func pathIDWithSuffix(c *gin.Context, prefix, suffix string) (int64, error) {
	tail := strings.TrimPrefix(c.Request.URL.Path, prefix)
	tail = strings.TrimSuffix(tail, suffix)
	if tail == "" {
		return 0, errInvalidID
	}
	return strconv.ParseInt(tail, 10, 64)
}

// nowUnix returns the current unix timestamp (seconds).  Mirrors
// Zig's `std.time.timestamp()`.
func nowUnix() int64 { return time.Now().Unix() }

// errInvalidID is returned by pathID* when the URL segment doesn't
// parse as int64.
var errInvalidID = &handlerError{message: "invalid id"}