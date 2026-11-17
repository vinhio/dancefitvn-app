//go:build !tvos

package kitdemo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPlanXMatchesShotcmp runs the case of cmd/shotcmp's TestStretchRuleNested
// (same boxes, same expected outputs, copied from there) through planX, so the
// two copies of the rule cannot drift apart unnoticed.
func TestPlanXMatchesShotcmp(t *testing.T) {
	t.Parallel()
	type box struct {
		name       string
		x, y, w, h float32
		wantX      float32
		wantW      float32
	}
	// x, w are the prototype's at 393; wantX, wantW the app's at 412 (cmd/shotcmp/shotcmp_test.go).
	cases := []box{
		{"card", 20, 100, 353, 60, 20, 372},
		{"ico", 34, 110, 36, 36, 34, 36},
		{"text", 82, 112, 245, 19, 82, 264},
		{"chev", 339, 112, 20, 20, 358, 20},
		{"tabs", 0, 700, 393, 84, 0, 412},
		{"t1", 0, 701, 131, 83, 0, 137.33},
		{"t2", 131, 701, 131, 83, 137.33, 137.33},
		{"t3", 262, 701, 131, 83, 274.67, 137.33},
		{"a", 100, 300, 46, 60, 109.5, 46},
		{"b", 247, 300, 46, 60, 256.5, 46},
		{"c1", 20, 500, 156, 100, 20, 156},
		{"c2", 188, 500, 156, 100, 188, 156},
		{"c3", 356, 500, 156, 100, 356, 156},
	}
	boxes := make([]span, len(cases))
	for i, c := range cases {
		boxes[i] = span{c.x, c.w, c.y, c.h}
	}
	xs, ws := planX(boxes, make([]bool, len(cases)), make([]bool, len(cases)), 19, 393)
	for i, c := range cases {
		assert.InDelta(t, c.wantX, xs[i], 0.01, c.name+" x")
		assert.InDelta(t, c.wantW, ws[i], 0.01, c.name+" w")
	}
}
