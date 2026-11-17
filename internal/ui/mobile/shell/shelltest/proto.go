//go:build !tvos

package shelltest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/ui/kit/layoutdump"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Delta is how far one element of the app's layout is from the prototype's.
type Delta struct {
	// Key is the element's layout.json key ("div.inp#2").
	Key string
	// Missing is an element of the prototype the app does not have.
	Missing bool
	// DX, DY, DW, DH are the app's x, y, width and height less the prototype's,
	// in DIPs.
	DX, DY, DW, DH float64
}

// Max returns the largest of the four distances.
func (d Delta) Max() float64 {
	return math.Max(math.Max(math.Abs(d.DX), math.Abs(d.DY)), math.Max(math.Abs(d.DW), math.Abs(d.DH)))
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ProtoLayout reads the prototype's capture of state id
// (local/shots/prototype-kit/mobile/<id>.layout.json, the one with every kit
// element recorded); the test is skipped when local/ is not there.
func ProtoLayout(tb testing.TB, id string) layoutdump.Layout {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatalf("shelltest: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("shelltest: no go.mod above the test")
		}
		dir = parent
	}
	data, err := os.ReadFile(filepath.Join(dir, "local", "shots", "prototype-kit", "mobile", id+".layout.json"))
	if err != nil {
		tb.Skipf("prototype capture not available: %v", err)
	}
	var lay layoutdump.Layout
	if err := json.Unmarshal(data, &lay); err != nil {
		tb.Fatalf("shelltest: %s: %v", id, fmt.Errorf("decode: %w", err))
	}

	return lay
}

// Compare returns, for every element of the prototype's layout, how far the
// app's is; skip names the keys left out (what the state does not draw, or a
// documented difference).
func Compare(want, got layoutdump.Layout, skip map[string]bool) []Delta {
	var out []Delta
	for _, w := range want.Elements {
		if skip[w.Key] {
			continue
		}
		g, ok := got.Find(w.Key)
		if !ok {
			out = append(out, Delta{Key: w.Key, Missing: true})

			continue
		}
		out = append(out, Delta{Key: w.Key, DX: g.X - w.X, DY: g.Y - w.Y, DW: g.W - w.W, DH: g.H - w.H})
	}

	return out
}
