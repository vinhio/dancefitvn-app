package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// etagOf returns the ETag of a body: the first 16 hex digits of its sha256,
// quoted.
func etagOf(body []byte) string {
	sum := sha256.Sum256(body)

	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

// etagMatches reports whether an If-None-Match value names tag: "*", or one
// of a comma list, weak or strong.
func etagMatches(header, tag string) bool {
	for _, c := range strings.Split(header, ",") {
		c = strings.TrimPrefix(strings.TrimSpace(c), "W/")
		if c == "*" || c == tag {
			return true
		}
	}

	return false
}
