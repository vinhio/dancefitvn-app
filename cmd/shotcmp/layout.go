package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Options are the thresholds and switches of one comparison.
type Options struct {
	Tol         float64            // box tolerance in layout units: 1 pt mobile, 2 px tv
	Margin      float64            // stretch rule: an edge within Margin of the screen edge is anchored to it
	BottomExtra float64            // vertical stretch rule: the system's bottom inset minus the prototype's 34 (-bottom-inset)
	Moved       map[string]float64 // vertical stretch rule: keys moved by a share of the extra height (-vmove)
	Bottoms     []float64          // vertical stretch rule: how far above the bottom edge elements positioned from it sit (the sticky button 38, the toast 100)
	CheckText   bool               // compare the visible text
	Include     *regexp.Regexp     // when set, only elements whose name matches are compared
	Exclude     *regexp.Regexp     // elements whose name matches are skipped
	RequireAll  bool               // elements of the prototype missing in the app fail (else only reported)
	NoStretch   bool               // compare boxes as they are even when the screen widths differ
	StyleFilter map[string]bool
}

// Row is the verdict on one element.
type Row struct {
	Key      string    `json:"key"`
	Status   string    `json:"status"` // ok, FAIL, missing, extra, skipped
	Proto    *Box      `json:"proto,omitempty"`
	App      *Box      `json:"app,omitempty"`
	Expected *Box      `json:"expected,omitempty"` // prototype box after the stretch rule
	MaxDelta float64   `json:"maxDelta"`
	Issues   []string  `json:"issues,omitempty"`
	Pixel    *PixelRow `json:"pixel,omitempty"`
}

// Box is x, y, w, h.
type Box struct{ X, Y, W, H float64 }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// CompareLayout matches the elements of proto and app by (name, index) and
// returns one row per element; ok is false when any row failed.
func CompareLayout(proto, app Layout, o Options) (rows []Row, ok bool) {
	ok = true
	st := Stretch{ProtoW: proto.Screen.W, Margin: o.Margin, ProtoH: proto.Screen.H, Bottoms: o.Bottoms, Moved: o.Moved, BottomExtra: o.BottomExtra}
	if !o.NoStretch {
		st.DW = app.Screen.W - proto.Screen.W
		st.DH = app.Screen.H - proto.Screen.H
	}
	expect := st.plan(proto.Elements)
	byKey := map[string]Element{}
	for _, e := range app.Elements {
		byKey[e.match()] = e
	}
	seen := map[string]bool{}
	for _, p := range proto.Elements {
		if !o.selected(p.Name) {
			continue
		}
		k := p.match()
		seen[k] = true
		pb := Box{p.X, p.Y, p.W, p.H}
		a, found := byKey[k]
		if !found {
			row := Row{Key: k, Status: "missing", Proto: &pb}
			if o.RequireAll {
				row.Status = "FAIL"
				row.Issues = []string{"element missing in the app"}
				ok = false
			}
			rows = append(rows, row)
			continue
		}
		ab := Box{a.X, a.Y, a.W, a.H}
		exp := expect[k]
		row := Row{Key: k, Status: "ok", Proto: &pb, App: &ab, Expected: &exp}
		row.MaxDelta = math.Max(math.Max(math.Abs(ab.X-exp.X), math.Abs(ab.Y-exp.Y)), math.Max(math.Abs(ab.W-exp.W), math.Abs(ab.H-exp.H)))
		if row.MaxDelta > o.Tol+1e-9 {
			row.Issues = append(row.Issues, fmt.Sprintf("box off by %.2f (tol %.2f)", row.MaxDelta, o.Tol))
		}
		row.Issues = append(row.Issues, compareStyle(p, a, o)...)
		if len(row.Issues) > 0 {
			row.Status = "FAIL"
			ok = false
		}
		rows = append(rows, row)
	}
	for _, a := range app.Elements {
		if o.selected(a.Name) && !seen[a.match()] {
			ab := Box{a.X, a.Y, a.W, a.H}
			rows = append(rows, Row{Key: a.match(), Status: "extra", App: &ab})
		}
	}
	return rows, ok
}

// selected tells whether the element name takes part in the comparison.
func (o Options) selected(name string) bool {
	if o.Include != nil && !o.Include.MatchString(name) {
		return false
	}
	return o.Exclude == nil || !o.Exclude.MatchString(name)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// compareStyle lists the style fields (and the text) that differ. A field
// the app did not record is not compared; the text styles (font, color,
// letter spacing) only count for elements that show text.
func compareStyle(p, a Element, o Options) []string {
	var out []string
	add := func(field, pv, av string) {
		out = append(out, fmt.Sprintf("%s: prototype %s, app %s", field, pv, av))
	}
	ps, as := p.Style, a.Style
	if o.CheckText && p.Text != "" && a.Text != "" && !sameText(p.Text, a.Text) {
		add("text", strconv.Quote(p.Text), strconv.Quote(a.Text))
	}
	if p.Text != "" {
		if as.FontFamily != "" && firstFamily(ps.FontFamily) != firstFamily(as.FontFamily) {
			add("fontFamily", ps.FontFamily, as.FontFamily)
		}
		if as.FontSize != "" && !pxEqual(ps.FontSize, as.FontSize, 0.1) {
			add("fontSize", ps.FontSize, as.FontSize)
		}
		if as.FontWeight != "" && weight(ps.FontWeight) != weight(as.FontWeight) {
			add("fontWeight", ps.FontWeight, as.FontWeight)
		}
		if as.Color != "" && !colorEqual(ps.Color, as.Color) {
			add("color", ps.Color, as.Color)
		}
		if as.LetterSpacing != "" && !pxEqual(ps.LetterSpacing, as.LetterSpacing, 0.1) {
			add("letterSpacing", ps.LetterSpacing, as.LetterSpacing)
		}
		if as.LineHeight != "" && ps.LineHeight != "normal" && as.LineHeight != "normal" && !pxEqual(ps.LineHeight, as.LineHeight, 0.5) {
			add("lineHeight", ps.LineHeight, as.LineHeight)
		}
	}
	if as.Background != "" && !colorEqual(ps.Background, as.Background) {
		add("background", ps.Background, as.Background)
	}
	if as.Radius != "" && !pxEqual(ps.Radius, as.Radius, 0.5) {
		add("radius", ps.Radius, as.Radius)
	}
	if as.Border != "" && !borderEqual(ps.Border, as.Border) {
		add("border", ps.Border, as.Border)
	}
	if as.Opacity != "" && !numEqual(ps.Opacity, as.Opacity, 0.01) {
		add("opacity", ps.Opacity, as.Opacity)
	}
	return out
}

// captureTextCap is how many UTF-16 units of an element's text the
// prototype's capture keeps (local/tools/capture_kit.py: `.trim().slice(0, 80)`).
const captureTextCap = 80

// sameText compares the visible texts, folding case and white space; a
// prototype text of the capture's cap is a cut one, so the app's is cut the
// same way first.
func sameText(proto, app string) bool {
	if normText(proto) == normText(app) {
		return true
	}
	if len(utf16.Encode([]rune(proto))) < captureTextCap {
		return false
	}
	u := utf16.Encode([]rune(strings.TrimSpace(app)))
	if len(u) > captureTextCap {
		u = u[:captureTextCap]
	}

	return normText(proto) == normText(string(utf16.Decode(u)))
}

// normText folds case (text-transform) and white space.
func normText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// firstFamily is the first family of a CSS font-family list, unquoted and
// lower case.
func firstFamily(s string) string {
	f, _, _ := strings.Cut(s, ",")
	return strings.ToLower(strings.Trim(strings.TrimSpace(f), `"'`))
}

// weight maps a CSS font-weight to its number.
func weight(s string) int {
	switch strings.TrimSpace(s) {
	case "normal", "":
		return 400
	case "bold":
		return 700
	default:
	}
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// pxValue parses "14px", "14", "normal" (0).
func pxValue(s string) (float64, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "px"))
	if s == "normal" || s == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func pxEqual(a, b string, tol float64) bool {
	av, aok := pxValue(a)
	bv, bok := pxValue(b)
	if !aok || !bok {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return math.Abs(av-bv) <= tol+1e-9
}

func numEqual(a, b string, tol float64) bool {
	av, e1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	bv, e2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if e1 != nil || e2 != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return math.Abs(av-bv) <= tol
}

var numRe = regexp.MustCompile(`[0-9]*\.?[0-9]+`)

// parseColor reads rgb(), rgba() and #rrggbb[aa]; a fully transparent
// color is the zero value whatever its channels.
func parseColor(s string) (c [4]float64, ok bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") && (len(s) == 7 || len(s) == 9) {
		c[3] = 255
		for i := 0; i < (len(s)-1)/2; i++ {
			v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
			if err != nil {
				return c, false
			}
			c[i] = float64(v)
		}
		c[3] /= 255
	} else if strings.HasPrefix(s, "rgb") {
		n := numRe.FindAllString(s, -1)
		if len(n) < 3 {
			return c, false
		}
		for i := 0; i < 3; i++ {
			c[i], _ = strconv.ParseFloat(n[i], 64)
		}
		c[3] = 1
		if len(n) > 3 {
			c[3], _ = strconv.ParseFloat(n[3], 64)
		}
	} else {
		return c, false
	}
	if c[3] == 0 {
		return [4]float64{}, true
	}
	return c, true
}

// colorEqual compares two CSS colors: channels within 1, alpha within 0.01.
func colorEqual(a, b string) bool {
	ac, aok := parseColor(a)
	bc, bok := parseColor(b)
	if !aok || !bok {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(ac[i]-bc[i]) > 1 {
			return false
		}
	}
	return math.Abs(ac[3]-bc[3]) <= 0.01
}

// borderEqual compares "1px rgb(...)": the widths, and the colors when the
// border is visible.
func borderEqual(a, b string) bool {
	aw, ac, _ := strings.Cut(strings.TrimSpace(a), " ")
	bw, bc, _ := strings.Cut(strings.TrimSpace(b), " ")
	if !pxEqual(aw, bw, 0.5) {
		return false
	}
	if v, _ := pxValue(aw); v == 0 {
		return true
	}
	return colorEqual(ac, bc)
}
