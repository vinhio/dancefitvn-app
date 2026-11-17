// Adapted from jplayer-mobile/internal/app/devaccess.go at 49de3c2: the
// script's step a11y is the developer's variable DFV_A11Y=1 here, which
// dumps the state's accessibility once its layout is written (devshot.go),
// then again after each key press of DFV_A11Y_MOVE (the TV's focus moves);
// the log goes through the services' logger, prefixed "dfv:".

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dancefitvn-app/internal/sys"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// This file is a developer's, as devshot.go: the accessibility of the
// window, dumped where no screen reader runs (the simulators have no
// VoiceOver; TalkBack reads nothing of the content, G10) by
// scripts/a11y-dump.sh (local/plans/P8.md P8.3d).

// a11yMoveWait is how long, by the view's clock, a key press of
// DFV_A11Y_MOVE is let settle (the focus's glide, the bridge's layout
// change) before the next dump.
const a11yMoveWait = 700 * time.Millisecond

// devA11y dumps the accessibility of a state into the shot folder: once,
// then after each key press of moves. UI thread.
type devA11y struct {
	id    string
	v     *ui.View
	dir   string
	logf  func(format string, args ...any)
	moves []keys.Key
	next  int
	step  func()
}

// a11yReport is the dump of the window's accessibility at one moment: the
// content's tree as gDesk builds it (ui.View.Semantics, every system) and,
// on iOS and tvOS, the elements the uikit bridge hands VoiceOver
// (sys.AccessibilityDump), or why there are none. Name is the state's id,
// with the key presses that followed it ("TV04", "TV04-m1").
type a11yReport struct {
	Name       string            `json:"name"`
	OS         string            `json:"os"`
	Size       geom.Size         `json:"size"`
	Tree       []a11yNode        `json:"tree"`
	System     []sys.A11yElement `json:"system,omitempty"`
	SystemNote string            `json:"systemNote,omitempty"`
}

// a11yNode is a node of the content's tree, as a person reads it.
type a11yNode struct {
	Depth    int       `json:"depth"`
	Role     string    `json:"role"`
	Label    string    `json:"label,omitempty"`
	Value    string    `json:"value,omitempty"`
	Toggle   string    `json:"toggle,omitempty"` // "on", "off", "mixed"
	Level    int32     `json:"level,omitempty"`
	Range    []float64 `json:"range,omitempty"` // min, max, now of a slider or a progress
	Flags    []string  `json:"flags,omitempty"`
	Actions  []string  `json:"actions,omitempty"`
	Bounds   geom.Rect `json:"bounds"`
	Focused  bool      `json:"focused,omitempty"`
	Children int       `json:"children,omitempty"`
}

// a11yRoles name the roles of a dump.
var a11yRoles = map[platform.SemanticRole]string{
	platform.RoleUnknown: "group", platform.RoleGroup: "group", platform.RoleWindow: "window",
	platform.RoleText: "text", platform.RoleHeading: "heading", platform.RoleButton: "button",
	platform.RoleLink: "link", platform.RoleCheckbox: "checkbox", platform.RoleSwitch: "switch",
	platform.RoleRadio: "radio", platform.RoleSlider: "slider", platform.RoleTextField: "textfield",
	platform.RoleTextArea: "textarea", platform.RoleProgress: "progress", platform.RoleImage: "image",
	platform.RoleScroll: "scroll", platform.RoleList: "list", platform.RoleListItem: "listitem",
	platform.RoleTabList: "tablist", platform.RoleTab: "tab", platform.RoleMenu: "menu",
	platform.RoleMenuItem: "menuitem", platform.RoleComboBox: "combobox", platform.RoleDialog: "dialog",
	platform.RoleTooltip: "tooltip", platform.RoleTree: "tree", platform.RoleTreeItem: "treeitem",
	platform.RoleTable: "table", platform.RoleToolbar: "toolbar", platform.RoleStatus: "status",
	platform.RoleRow: "row", platform.RoleCell: "cell", platform.RoleColumnHeader: "columnheader",
}

// a11yActions name the actions of a dump.
var a11yActions = [...]struct {
	a    platform.SemanticActions
	name string
}{
	{platform.ActPress, "press"},
	{platform.ActIncrement, "increment"},
	{platform.ActDecrement, "decrement"},
	{platform.ActFocus, "focus"},
	{platform.ActScrollForward, "scrollForward"},
	{platform.ActScrollBackward, "scrollBackward"},
	{platform.ActExpand, "expand"},
	{platform.ActCollapse, "collapse"},
}

// a11yFlags name the flags of a dump.
var a11yFlags = [...]struct {
	f    platform.SemanticFlags
	name string
}{
	{platform.FlagDisabled, "disabled"},
	{platform.FlagFocusable, "focusable"},
	{platform.FlagSelected, "selected"},
	{platform.FlagReadOnly, "readOnly"},
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDevA11y returns the dumps of state id on v into dir; moves names the
// keys pressed after the first dump, comma separated ("right,down"): an
// arrow, "select" (Enter) or "menu" (Escape), as the Siri Remote's.
func newDevA11y(id string, v *ui.View, dir, moves string, logf func(format string, args ...any)) (*devA11y, error) {
	d := &devA11y{id: id, v: v, dir: dir, logf: logf}
	for name := range strings.SplitSeq(moves, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		k, ok := a11yKey(name)
		if !ok {
			return nil, fmt.Errorf("app: a11y: key %q: %w", name, core.ErrInvalid)
		}
		d.moves = append(d.moves, k)
	}
	d.step = d.move

	return d, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// dumpA11y reads the accessibility of the window of v: its content's
// tree and, on iOS and tvOS, the system's elements of its canvas (none
// when the view has no canvas, as under a test). Call it on the UI
// thread.
func dumpA11y(name string, v *ui.View) a11yReport {
	rep := a11yReport{Name: name, OS: runtime.GOOS, Tree: a11yTree(v.Semantics())}
	c := v.Canvas()
	if c == nil {
		rep.SystemNote = "no canvas"

		return rep
	}
	rep.Size = c.Size()
	els, err := sys.AccessibilityDump(c)
	if err != nil {
		rep.SystemNote = err.Error()

		return rep
	}
	rep.System = els

	return rep
}

// start writes the first dump, then presses the keys one by one, a dump
// after each, and logs "dfv: a11y done <id>" at the end.
func (d *devA11y) start() {
	d.write(d.id)
	d.move()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// move presses the next key and dumps a11yMoveWait later; with none left
// it logs the end.
func (d *devA11y) move() {
	if d.next > 0 {
		d.write(fmt.Sprintf("%s-m%d", d.id, d.next))
	}
	if d.next == len(d.moves) {
		d.logf("dfv: a11y done %s", d.id)

		return
	}
	k := d.moves[d.next]
	d.next++
	d.v.HandleEvent(platform.Event{Kind: platform.EventKeyDown, Key: k})
	d.v.HandleEvent(platform.Event{Kind: platform.EventKeyUp, Key: k})
	d.v.Context().AfterFunc(a11yMoveWait, d.step)
}

// write dumps the window as name into <dir>/<name>.a11y.json, logs it
// and where it went.
func (d *devA11y) write(name string) {
	rep := dumpA11y(name, d.v)
	logA11y(rep, d.logf)
	path := filepath.Join(d.dir, name+".a11y.json")
	data, err := json.MarshalIndent(rep, "", "  ")
	if err == nil {
		err = os.MkdirAll(d.dir, 0o750)
	}
	if err == nil {
		err = os.WriteFile(path, data, 0o600)
	}
	if err != nil {
		d.logf("dfv: a11y %s: %v", name, err)

		return
	}
	d.logf("dfv: a11y ready %s %s", name, path)
}

// a11yKey is the key of a move's name: an arrow, select or menu.
func a11yKey(name string) (keys.Key, bool) {
	switch strings.ToLower(name) {
	case "select":
		return keys.KeyEnter, true
	case "menu":
		return keys.KeyEscape, true
	case "up", "down", "left", "right":
		return keys.Parse(name)
	default:
		return keys.KeyUnknown, false
	}
}

// a11yTree turns the content's tree into the nodes of a dump, each with
// its depth below the window.
func a11yTree(s *platform.Semantics) []a11yNode {
	if s == nil {
		return nil
	}
	out := make([]a11yNode, len(s.Nodes))
	for i := range s.Nodes {
		n := &s.Nodes[i]
		d := a11yNode{
			Role: a11yRoles[n.Role], Label: n.Label, Value: n.Value, Level: n.Level, Bounds: n.Bounds,
			Focused: i == s.Focus,
		}
		if n.Parent >= 0 && int(n.Parent) < i {
			d.Depth = out[n.Parent].Depth + 1
			out[n.Parent].Children++
		}
		switch n.Toggle {
		case platform.ToggledOn:
			d.Toggle = "on"
		case platform.ToggledOff:
			d.Toggle = "off"
		case platform.ToggledMixed:
			d.Toggle = "mixed"
		case platform.ToggledUnknown:
		default:
		}
		if n.Role == platform.RoleSlider || n.Role == platform.RoleProgress {
			d.Range = []float64{n.Min, n.Max, n.Now}
		}
		for _, f := range a11yFlags {
			if n.Flags.Has(f.f) {
				d.Flags = append(d.Flags, f.name)
			}
		}
		for _, a := range a11yActions {
			if n.Actions.Has(a.a) {
				d.Actions = append(d.Actions, a.name)
			}
		}
		out[i] = d
	}

	return out
}

// logA11y logs the report with logf: a line of counts, then one line a
// node of the tree and one an element of the system's.
func logA11y(rep a11yReport, logf func(format string, args ...any)) {
	logf("dfv: a11y %s %s %gx%g: %d nodes, %d system elements%s", rep.Name, rep.OS, rep.Size.W, rep.Size.H,
		len(rep.Tree), len(rep.System), noteText(rep.SystemNote))
	for i, n := range rep.Tree {
		logf("dfv: a11y tree %3d %*s%s %q%s", i, 2*n.Depth, "", n.Role, n.Label, nodeExtra(n))
	}
	for i, e := range rep.System {
		logf("dfv: a11y system %3d %*s%q value=%q traits=%v frame=%.0f,%.0f %.0fx%.0f in view %.0f,%.0f %.0fx%.0f",
			i, 2*e.Depth, "", e.Label, e.Value, e.Traits, e.Frame.X, e.Frame.Y, e.Frame.W, e.Frame.H,
			e.InView.X, e.InView.Y, e.InView.W, e.InView.H)
	}
}

// nodeExtra writes what a node holds besides its role and label.
func nodeExtra(n a11yNode) string {
	s := ""
	if n.Value != "" {
		s += fmt.Sprintf(" value=%q", n.Value)
	}
	if n.Toggle != "" {
		s += " " + n.Toggle
	}
	if n.Level > 0 {
		s += fmt.Sprintf(" level=%d", n.Level)
	}
	if n.Range != nil {
		s += fmt.Sprintf(" [%g..%g now %g]", n.Range[0], n.Range[1], n.Range[2])
	}
	if len(n.Flags) > 0 {
		s += fmt.Sprintf(" %v", n.Flags)
	}
	if len(n.Actions) > 0 {
		s += fmt.Sprintf(" act=%v", n.Actions)
	}

	return s + fmt.Sprintf(" @%.0f,%.0f %.0fx%.0f", n.Bounds.X, n.Bounds.Y, n.Bounds.W, n.Bounds.H)
}

// noteText writes the note of a report after a comma, nothing for none.
func noteText(note string) string {
	if note == "" {
		return ""
	}

	return " (" + note + ")"
}
