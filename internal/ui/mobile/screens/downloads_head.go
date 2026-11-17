//go:build !tvos

package screens

import (
	"strings"

	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// nameGroupHead is the key of the group eyebrow, `div.eb` (index.html:459).
const nameGroupHead = "div.eb"

// groupHead is the eyebrow of a group of downloads, "{course} · {n}/{total} bài"
// (index.html:459). The course of production is long (`DanceFIT® Set 35: "..."`),
// and the eyebrow is one line: the name is cut with an ellipsis where the line
// would pass the column, and the count after it stays whole. A name that fits
// leaves the line as the plain eyebrow is (the same box and glyphs).
type groupHead struct {
	ui.Base
	// Describer, below: one heading that reads the whole line; its two labels are
	// pieces of it, not elements.
	kit.Decor

	name, tail *kit.Label
	full       string
}

var (
	_ ui.Describer = (*groupHead)(nil)
	_ ui.Labeler   = (*groupHead)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newGroupHead returns the eyebrow of a group: full is the whole line as the
// template fills it, name the part of it that may be cut (its start).
func newGroupHead(full, name string) *groupHead {
	h := &groupHead{full: full}
	h.Name = nameGroupHead
	rest, ok := strings.CutPrefix(full, name)
	if !ok || name == "" {
		name, rest = full, ""
	}
	h.name = kit.Eyebrow(name)
	h.name.Name, h.name.Ellipsis = "", true
	h.tail = kit.Eyebrow(rest)
	h.tail.Name = ""
	h.Add(h.name, h.tail)

	return h
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel is the whole line.
func (h *groupHead) AccessibleLabel() string { return h.full }

// Describe describes the line as a heading of level 2.
func (h *groupHead) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label, n.Level = platform.RoleHeading, h.full, 2
}

// Layout puts the name and the count on one line, the count after the name at
// its own width and the name in what is left of the room.
func (h *groupHead) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	ts := geom.Size{}
	if h.tail.Text != "" {
		ts = c.Layout(h.tail, free)
	}
	room := ui.Unbounded
	if cs.Max.W < ui.Unbounded {
		room = max(cs.Max.W-ts.W, 0)
	}
	ns := c.Layout(h.name, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(h.name, geom.Point{})
	c.Place(h.tail, geom.Pt(ns.W, 0))

	return cs.Constrain(geom.Sz(ns.W+ts.W, max(ns.H, ts.H)))
}

// LayoutStyle is the eyebrow's style and its whole line, as layout.json holds the
// one element the prototype has.
func (h *groupHead) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	css, _ := h.name.LayoutStyle(c)

	return css, strings.ToUpper(h.full)
}
