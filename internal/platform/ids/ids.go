// Package ids generates sortable, unique identifiers for domain records.
package ids

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// New returns an identifier of the form "<prefix>_<ulid>" using lowercase
// ULIDs so identifiers sort by creation time.
func New(prefix string) string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	if prefix == "" {
		return strings.ToLower(id.String())
	}
	return prefix + "_" + strings.ToLower(id.String())
}

// NewWithTime is like New but allows an explicit timestamp, which keeps
// deterministic tests readable.
func NewWithTime(prefix string, t time.Time) string {
	id := ulid.MustNew(ulid.Timestamp(t), rand.Reader)
	if prefix == "" {
		return strings.ToLower(id.String())
	}
	return prefix + "_" + strings.ToLower(id.String())
}
