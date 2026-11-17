package kit

import (
	"strconv"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .prog (index.html:137).
const (
	// progHeight, progRadius are `.prog{height:5px;border-radius:3px}` and
	// `.prog i{border-radius:3px}`.
	progHeight, progRadius float32 = 5, 3
	// progWidth is the width of a bar given no width limit (`width:120px` of
	// the splash, :334).
	progWidth float32 = 120
)

// nameProg is the key layout.json gives the bar: tag and class.
const nameProg = "div.prog"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Progress is `.prog` (index.html:137): a 5 tall, 3 rounded track of --sf2 with a
// --pr fill as wide as Value of it. The track and fill colors and the height
// can be replaced (`background:rgba(255,255,255,.3)`, :359; `height:8px`, :452).
type Progress struct {
	ui.Base
	// Value is how much of the track the fill covers, 0 to 1; change it with
	// SetValue.
	Value float32
	// Height is the track's height; 0 reads as 5.
	Height float32
	// Track and Fill replace --sf2 and --pr when not zero.
	Track, Fill color.Color
	// Label names the bar for assistive technologies ("Bộ nhớ máy").
	Label string
	// Spoken is the reading a screen reader gives of the bar ("01:12 / 03:40"),
	// made by the caller when it changes; empty reads as the percent of Value.
	Spoken string
}

var _ ui.Describer = (*Progress)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetValue moves the fill to v of the track, clamped to 0 and 1; the same value
// changes nothing. It does not allocate.
func (p *Progress) SetValue(v float32) {
	v = min(max(v, 0), 1)
	if v != p.Value {
		p.Value = v
		p.Repaint()
	}
}

// Describe describes the bar as a progress bar at Value, valued by Spoken, else by
// the percent (DescribeBar).
func (p *Progress) Describe(_ *ui.Context, n *platform.SemanticNode) {
	DescribeBar(n, p.Label, p.Spoken, p.Value)
}

// SetSpoken changes the reading of the bar and tells assistive technologies; the
// same text changes nothing.
func (p *Progress) SetSpoken(s string) {
	if s != p.Spoken {
		p.Spoken = s
		p.ValueChanged()
	}
}

// Layout takes the width it gets and the height of the track.
func (p *Progress) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	if p.Name == "" {
		p.Name = nameProg
	}
	w := progWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	h := p.Height
	if h <= 0 {
		h = progHeight
	}

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the track and the fill.
func (p *Progress) Paint(c *ui.Context, pt *ui.Painter) {
	t := &theme.From(c).T
	r := p.Bounds()
	pt.Fill(r, scene.Solid(orColor(p.Track, t.Surface2)), capRadius(r, progRadius))
	if w := r.W * min(max(p.Value, 0), 1); w > 0 {
		f := geom.XYWH(0, 0, w, r.H)
		pt.Fill(f, scene.Solid(orColor(p.Fill, t.Primary)), capRadius(f, progRadius))
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// percentText holds "0%" to "100%", so that the reading of a bar at a share
// allocates nothing.
var percentText = func() (t [101]string) {
	for i := range t {
		t[i] = strconv.Itoa(i) + "%"
	}

	return t
}()

// Percent returns share, 0 to 1, as whole percent ("42%"); it allocates nothing.
func Percent(share float32) string {
	return percentText[min(max(int(share*100+.5), 0), 100)]
}

// DescribeBar describes a bar that shows how far something is, 0 to 1 at share, as
// the screen readers' text of a name and a value: spoken when it is not empty
// ("01:12 / 03:40"), else the percent. The role is RoleText and not RoleProgress
// because the iOS and tvOS bridge gives a progress bar no trait, and an element
// with a value but no trait is not announced as one (P8.3d dump; gDesk
// proposal in the P8.3d report); Min, Max and Now keep the range for a bridge
// that reads them.
func DescribeBar(n *platform.SemanticNode, label, spoken string, share float32) {
	n.Role, n.Label, n.Min, n.Max, n.Now = platform.RoleText, label, 0, 1, float64(share)
	n.Value = spoken
	if spoken == "" {
		n.Value = Percent(share)
	}
}
