package mock

import (
	"bytes"
	"fmt"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mediaScheme is the scheme of media URLs in the mock bodies (rule 6).
const mediaScheme = "mock-media://"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// rewriteMedia replaces mock-media:// in body with the address of the media
// server (rule 6). The server starts with the first body that needs it, and
// listens again, on its old port, when the system closed its socket.
func (t *Transport) rewriteMedia(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte(mediaScheme)) {
		return body, nil
	}
	if err := t.media.Ensure(); err != nil {
		return nil, fmt.Errorf("mock: media: %w", err)
	}

	return bytes.ReplaceAll(body, []byte(mediaScheme), []byte(t.media.BaseURL()+"/")), nil
}
