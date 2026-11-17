//go:build tvos

package tvtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// protoDir is the prototype's TV layout files, from a test's package
// directory under internal/ui/tv/<pkg>.
var protoDir = filepath.Join("..", "..", "..", "..", "local", "shots", "prototype-kit", "tv")

// protoLayout is the part of a layout.json the boxes need.
type protoLayout struct {
	Elements []struct {
		Key string  `json:"key"`
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
		W   float64 `json:"w"`
		H   float64 `json:"h"`
	} `json:"elements"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ProtoBox returns the box of the element key in the prototype's layout of the
// state id ("TV04"), as the capture recorded it; it skips the test where the
// capture is not on disk (local/ is not in the repository).
func ProtoBox(t testing.TB, id, key string) geom.Rect {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(protoDir, id+".layout.json"))
	if err != nil {
		t.Skipf("no prototype layout %s: %v", id, err)
	}
	var l protoLayout
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatalf("layout %s: %v", id, err)
	}
	for _, e := range l.Elements {
		if e.Key == key {
			return geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
		}
	}
	t.Fatalf("layout %s has no element %q", id, key)

	return geom.Rect{}
}

// ProtoKeys returns the keys of the elements of the prototype's layout of the
// state id, in the order of the capture; it skips the test where the capture is
// not on disk.
func ProtoKeys(t testing.TB, id string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(protoDir, id+".layout.json"))
	if err != nil {
		t.Skipf("no prototype layout %s: %v", id, err)
	}
	var l protoLayout
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatalf("layout %s: %v", id, err)
	}
	keys := make([]string, 0, len(l.Elements))
	for _, e := range l.Elements {
		keys = append(keys, e.Key)
	}

	return keys
}
