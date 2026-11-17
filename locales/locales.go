// Package locales holds the app's display strings, embedded. Only the
// packages that draw text read them; the Vietnamese text lives in vi.json
// and nowhere in Go code.
package locales

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// FS holds vi.json, a flat object of key to text.
//
//go:embed vi.json
var FS embed.FS

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Texts returns the texts of vi.json, by key. It fails with
// core.ErrInvalid when the embedded file is not a flat object of strings.
func Texts() (map[string]string, error) {
	data, err := FS.ReadFile("vi.json")
	if err != nil {
		return nil, fmt.Errorf("locales: read vi.json: %w", err)
	}

	return decode(data)
}

// MustTexts is Texts for callers that cannot go on without the texts: it
// panics on Texts' error, which no test run could miss since the file is
// embedded.
func MustTexts() map[string]string { return must(Texts()) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// must returns texts, and panics with err when it is not nil.
func must(texts map[string]string, err error) map[string]string {
	if err != nil {
		panic(err)
	}

	return texts
}

// decode reads data as a flat object of key to text.
func decode(data []byte) (map[string]string, error) {
	var texts map[string]string
	if err := json.Unmarshal(data, &texts); err != nil {
		return nil, fmt.Errorf("locales: decode vi.json: %w: %w", core.ErrInvalid, err)
	}
	if texts == nil {
		return nil, fmt.Errorf("locales: decode vi.json: not an object: %w", core.ErrInvalid)
	}

	return texts, nil
}
