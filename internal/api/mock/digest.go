package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io/fs"
	"strings"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// digest is the size and sha256 of a media file.
type digest struct {
	size int64
	sum  string
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// patchAudio sets size_bytes and sha256 of a lessons.audio body to those of
// the file its url names, as the media server serves it (the _sample when
// the lesson has no file). The mock JSON holds numbers of another file, and
// the download would fail its check. A body it cannot understand is
// returned as it is. Key order is not kept.
func (t *Transport) patchAudio(body []byte) ([]byte, error) {
	var m map[string]jsontext.Value
	if err := json.Unmarshal(body, &m); err != nil {
		return body, nil //nolint:nilerr // not our shape: leave it
	}
	var u string
	if err := json.Unmarshal(m["url"], &u); err != nil {
		return body, nil //nolint:nilerr // not our shape: leave it
	}
	name, ok := strings.CutPrefix(u, mediaScheme)
	if !ok {
		return body, nil
	}
	name, _, _ = strings.Cut(name, "?")
	d, err := t.digestOf(name)
	if err != nil {
		return nil, err
	}
	size, _ := json.Marshal(d.size)
	sum, _ := json.Marshal(d.sum)
	m["size_bytes"], m["sha256"] = size, sum
	out, err := json.Marshal(m, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("mock: audio body: %w", err)
	}

	return out, nil
}

// digestOf returns the digest of a media file by the name in the URL, the
// _sample fallback applied, computed once per file.
func (t *Transport) digestOf(name string) (digest, error) {
	file, ok := t.media.Resolve(name)
	if !ok {
		return digest{}, fmt.Errorf("mock: media %s: %w", name, core.ErrNotFound)
	}
	t.digestMu.Lock()
	defer t.digestMu.Unlock()
	if d, ok := t.digests[file]; ok {
		return d, nil
	}
	data, err := fs.ReadFile(t.media.files, file)
	if err != nil {
		return digest{}, fmt.Errorf("mock: media %s: %w", file, err)
	}
	sum := sha256.Sum256(data)
	d := digest{size: int64(len(data)), sum: hex.EncodeToString(sum[:])}
	t.digests[file] = d

	return d, nil
}
