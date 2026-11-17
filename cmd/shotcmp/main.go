package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Exit codes.
const (
	exitPass  = 0
	exitFail  = 1
	exitUsage = 2
)

// Config is one run: where the shots are and how to compare them.
type Config struct {
	ProtoDir, AppDir, DiffDir string
	Layout, Crop, Full        bool
	Opts                      Options
	Masks                     Masks
	VMove                     map[string]float64 // "ID:key" to share, from -vmove
	StateMasks                map[string][]Rect  // by state ID, from "-mask ID:x,y,w,h"
	ProtoScale, AppScale      float64            // flags; 0: from the PNG width
	ProtoOrigin, AppOrigin    Origin             // pixel of layout (0, 0); zero value: 0,0
	MinCrop                   float64            // crops with a side below this many units are skipped
	// ProtoPNGDir, when set, is where the prototype's PNGs are read from (the
	// 3x captures, `mobile@3x`) while the layout still comes from ProtoDir.
	ProtoPNGDir string
	// TextMax is the share of differing pixels a crop of an element that shows
	// text may have (the rasterizers of Core Text and Chrome differ); the other
	// crops keep pixelMax.
	TextMax float64
	// Waived lists "ID:key" elements whose failures are reported as waived.
	Waived map[string]bool
}

// StateResult is the outcome of one state.
type StateResult struct {
	ID    string    `json:"id"`
	Pass  bool      `json:"pass"`
	Notes []string  `json:"notes,omitempty"`
	Rows  []Row     `json:"rows"`
	Full  *PixelRow `json:"full,omitempty"`
	Diff  string    `json:"diff,omitempty"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the exit, for the tests.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("shotcmp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	cfg := Config{StateMasks: map[string][]Rect{}}
	var masks multi
	var (
		states    = fs.String("states", "", "comma-separated state IDs (default: every <ID>.layout.json of -proto)")
		mode      = fs.String("mode", "layout,crop", "comma-separated: layout (boxes + style), crop (pixel diff per matched element), full (pixel diff of the whole screen)")
		report    = fs.String("report", "local/shots/report.md", "markdown report file (empty: none)")
		jsonOut   = fs.String("json", "", "also write the report as JSON to this file")
		platform  = fs.String("platform", "", "mobile or tv (default: the prototype layout's platform)")
		tol       = fs.Float64("tol", 0, "box tolerance in layout units (0: 1 for mobile, 2 for tv)")
		statusBar = fs.Float64("status-bar", -1, "height of the status-bar band to mask in units (-1: 54 mobile, 0 tv)")
		corner    = fs.Float64("corner", -1, "corner radius of the device frame to mask in units (-1: 49 mobile, 0 tv)")
		maskName  = fs.String("mask-name", "", "regexp: boxes of prototype elements with these names are masked (video area, QR, clock)")
		include   = fs.String("include", "", "regexp: compare only elements whose name matches")
		exclude   = fs.String("exclude", "", "regexp: skip elements whose name matches")
		noText    = fs.Bool("no-text", false, "do not compare the visible text")
	)
	fs.StringVar(&cfg.ProtoDir, "proto", "local/shots/prototype-kit/mobile", "folder of the prototype's <ID>.png and <ID>.layout.json")
	fs.StringVar(&cfg.AppDir, "app", "local/shots/kit/ios", "folder of the app's <ID>.png and <ID>.layout.json")
	fs.StringVar(&cfg.DiffDir, "diff", "local/shots/diff", "folder of the diff pictures <ID>.png (prototype | app | difference)")
	fs.Float64Var(&cfg.Opts.Margin, "margin", 24, "stretch rule: an element edge within this distance of the screen edge is anchored to it")
	fs.Float64Var(&cfg.ProtoScale, "proto-scale", 0, "pixels per unit of the prototype PNG (0: PNG width over layout screen width)")
	fs.Float64Var(&cfg.AppScale, "app-scale", 0, "pixels per unit of the app PNG (0: PNG width over layout screen width)")
	fs.StringVar(&cfg.ProtoPNGDir, "proto-3x", "", "folder of the prototype's 3x PNGs (`mobile@3x`, 3 px frame column): pixels are compared at 3x against an app picture at 3x, the layout still comes from -proto")
	fs.Float64Var(&cfg.TextMax, "text-max", 0.08, "share of pixels (0-1) that may differ in the crop of an element that shows text; other crops keep 0.015")
	bottomInset := fs.Float64("bottom-inset", 34, "the system's bottom inset of the app's device, in DIPs (the prototype draws 34): the tab bar grows by the difference (Android 3-button 48, gesture 24)")
	var vmoves multi
	fs.Var(&vmoves, "vmove", "`ID:key=share` of an element the Android stretch rule moves by that share of the extra height: 0.5 for a child of a flex column centred on the screen, 1 for one positioned from the bottom (repeatable; documented causes only)")
	var excepts multi
	fs.Var(&excepts, "except", "`ID:key` of an element whose failures are waived (repeatable), e.g. M15:b#1; documented causes only")
	exceptFile := fs.String("except-file", "", "file of waived elements, one `ID:key` per line, lines starting with # are comments")
	fs.Float64Var(&cfg.MinCrop, "min-crop", 4, "skip the pixel crop of elements with a side smaller than this")
	fs.BoolVar(&cfg.Opts.RequireAll, "require-all", false, "prototype elements missing in the app fail (default: reported only)")
	bottoms := fs.String("bottoms", "38,100", "vertical stretch rule: how far above the bottom edge (comma separated) elements positioned from it sit: the sticky button 38, the toast 100")
	fs.BoolVar(&cfg.Opts.NoStretch, "no-stretch", false, "do not apply the Android stretch rule when the screen widths differ")
	fs.Var(&masks, "mask", "rectangle to mask in prototype units, `[ID:]x,y,w,h` (repeatable; without ID it applies to every state)")
	protoOrigin := fs.String("proto-origin", "auto", "pixel of the prototype PNG where layout (0, 0) lies: auto (PNG size minus screen size, up to 2 px: the 1 px frame of mobile shots) or x,y")
	appOrigin := fs.String("app-origin", "auto", "same for the app PNG")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitPass
		}
		return exitUsage
	}
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "shotcmp:", err)
		return exitUsage
	}
	for _, m := range strings.Split(*mode, ",") {
		switch strings.TrimSpace(m) {
		case "layout":
			cfg.Layout = true
		case "crop":
			cfg.Crop = true
		case "full":
			cfg.Full = true
		default:
			return fail(fmt.Errorf("unknown mode %q", m))
		}
	}
	if cfg.ProtoPNGDir != "" {
		// mobile@3x PNGs: 3x scale, 3 px frame column (before the origin is parsed)
		cfg.ProtoScale = 3
		if *protoOrigin == "auto" {
			*protoOrigin = "3,0"
		}
	}
	var err error
	if cfg.ProtoOrigin, err = parseOrigin(*protoOrigin); err != nil {
		return fail(err)
	}
	if cfg.AppOrigin, err = parseOrigin(*appOrigin); err != nil {
		return fail(err)
	}
	if cfg.Masks.Names, err = compileOpt(*maskName); err != nil {
		return fail(err)
	}
	if cfg.Opts.Include, err = compileOpt(*include); err != nil {
		return fail(err)
	}
	if cfg.Opts.Exclude, err = compileOpt(*exclude); err != nil {
		return fail(err)
	}
	cfg.Opts.CheckText = !*noText
	cfg.Opts.BottomExtra = *bottomInset - 34
	cfg.VMove = map[string]float64{}
	for _, v := range vmoves {
		k, share, ok := strings.Cut(strings.TrimSpace(v), "=")
		f, err := strconv.ParseFloat(share, 64)
		if !ok || err != nil {
			return fail(fmt.Errorf("-vmove %q: want ID:key=share", v))
		}
		cfg.VMove[k] = f
	}
	cfg.Waived = map[string]bool{}
	for _, e := range excepts {
		cfg.Waived[strings.TrimSpace(e)] = true
	}
	if *exceptFile != "" {
		b, err := os.ReadFile(*exceptFile)
		if err != nil {
			return fail(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				cfg.Waived[l] = true
			}
		}
	}
	for _, f := range strings.Split(*bottoms, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return fail(fmt.Errorf("-bottoms: %w", err))
		}
		cfg.Opts.Bottoms = append(cfg.Opts.Bottoms, v)
	}
	for _, m := range masks {
		id, r, err := parseMask(m)
		if err != nil {
			return fail(err)
		}
		if id == "" {
			cfg.Masks.Rects = append(cfg.Masks.Rects, r)
		} else {
			cfg.StateMasks[id] = append(cfg.StateMasks[id], r)
		}
	}
	ids := splitList(*states)
	if len(ids) == 0 {
		if ids, err = stateIDs(cfg.ProtoDir); err != nil {
			return fail(err)
		}
	}
	tv := *platform == "tv"
	var results []StateResult
	for _, id := range ids {
		c := cfg
		if *platform == "" {
			if pl, err := readLayout(filepath.Join(cfg.ProtoDir, id+".layout.json")); err == nil {
				tv = pl.Platform == "tv"
			}
		}
		c.Opts.Tol, c.Masks.StatusBar, c.Masks.CornerR = *tol, *statusBar, *corner
		if c.Opts.Tol == 0 {
			c.Opts.Tol = map[bool]float64{false: 1, true: 2}[tv]
		}
		if c.Masks.StatusBar < 0 {
			c.Masks.StatusBar = map[bool]float64{false: 54, true: 0}[tv]
		}
		if c.Masks.CornerR < 0 {
			c.Masks.CornerR = map[bool]float64{false: 49, true: 0}[tv]
		}
		r, err := CompareState(c, id)
		if err != nil {
			return fail(err)
		}
		results = append(results, r)
	}
	text := FormatReport(results)
	_, _ = io.WriteString(stdout, text)
	if *report != "" {
		if err := writeFile(*report, []byte(text)); err != nil {
			return fail(err)
		}
	}
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(results, "", " ")
		if err := writeFile(*jsonOut, append(b, '\n')); err != nil {
			return fail(err)
		}
	}
	for _, r := range results {
		if !r.Pass {
			return exitFail
		}
	}
	return exitPass
}

// CompareState compares one state: layout boxes and styles, the pixels of
// each matched element's crop, the pixels of the whole screen. The pixel
// modes are skipped (with a note) when the screens differ in width: the
// app is the Android 412-wide one, compared by layout only (07 section 3.3).
func CompareState(c Config, id string) (StateResult, error) {
	res, err := compareState(c, id)
	if err != nil || len(c.Waived) == 0 {
		return res, err
	}
	pass := true
	for i := range res.Rows {
		r := &res.Rows[i]
		if r.Status == "FAIL" && c.Waived[id+":"+r.Key] {
			r.Status = "waived"
		}
		if r.Status == "FAIL" {
			pass = false
		}
	}
	if res.Full != nil && res.Full.Status == "FAIL" {
		pass = false
	}
	res.Pass = pass || !c.Layout && !c.Crop && !c.Full

	return res, nil
}

// compareState is CompareState before the waivers.
func compareState(c Config, id string) (StateResult, error) {
	res := StateResult{ID: id, Pass: true}
	proto, err := readLayout(filepath.Join(c.ProtoDir, id+".layout.json"))
	if err != nil {
		return res, err
	}
	app, err := readLayout(filepath.Join(c.AppDir, id+".layout.json"))
	if err != nil {
		return res, err
	}
	opts := c.Opts
	for k, f := range c.VMove {
		if i, key, ok := strings.Cut(k, ":"); ok && i == id {
			if opts.Moved == nil {
				opts.Moved = map[string]float64{}
			}
			opts.Moved[key] = f
		}
	}
	res.Rows, res.Pass = CompareLayout(proto, app, opts)
	if !c.Layout {
		res.Pass = true
		// Matched pairs are still needed for the crops.
		res.Rows = matchedOnly(proto, app, c.Opts)
	}
	if !c.Crop && !c.Full {
		return res, nil
	}
	if proto.Screen.W != app.Screen.W || proto.Screen.H != app.Screen.H {
		res.Notes = append(res.Notes, fmt.Sprintf("pixel compare skipped: screen %gx%g (app) differs from %gx%g (prototype); layout only", app.Screen.W, app.Screen.H, proto.Screen.W, proto.Screen.H))
		return res, nil
	}
	pngDir := c.ProtoDir
	if c.ProtoPNGDir != "" {
		pngDir = c.ProtoPNGDir
	}
	pp, err := readPicture(filepath.Join(pngDir, id+".png"), proto.Screen, c.ProtoScale, c.ProtoOrigin)
	if err != nil {
		return res, err
	}
	ap, err := readPicture(filepath.Join(c.AppDir, id+".png"), app.Screen, c.AppScale, c.AppOrigin)
	if err != nil {
		return res, err
	}
	rects := append(c.Masks.maskRects(proto), c.StateMasks[id]...)
	b := pp.Img.Bounds()
	bad := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	var fails []Box
	texts := map[string]bool{}
	for _, e := range proto.Elements {
		texts[e.match()] = e.Text != ""
	}
	if c.Crop {
		for i := range res.Rows {
			r := &res.Rows[i]
			if r.Proto == nil || r.App == nil || r.Proto.W < c.MinCrop || r.Proto.H < c.MinCrop {
				continue
			}
			limit := pixelMax
			if texts[r.Key] && c.TextMax > 0 {
				limit = c.TextMax
			}
			px := ComparePixels(pp, ap, *r.Proto, *r.App, rects, c.Masks.CornerR, bad, limit)
			r.Pixel = &px
			if px.Status == "FAIL" {
				r.Status = "FAIL"
				r.Issues = append(r.Issues, fmt.Sprintf("pixels: %.2f %% differ (max %.1f %%)", 100*px.Share, 100*limit))
				fails = append(fails, *r.Proto)
				res.Pass = false
			}
		}
	}
	if c.Full {
		whole := Box{0, 0, proto.Screen.W, proto.Screen.H}
		px := ComparePixels(pp, ap, whole, Box{0, 0, app.Screen.W, app.Screen.H}, rects, c.Masks.CornerR, bad, pixelMax)
		res.Full = &px
		if px.Status == "FAIL" {
			res.Pass = false
		}
	}
	if c.DiffDir != "" {
		res.Diff = filepath.Join(c.DiffDir, id+".png")
		if err := writeTriptych(res.Diff, pp, ap, bad, rects, c.Masks.CornerR, fails); err != nil {
			return res, err
		}
	}
	return res, nil
}

// FormatReport renders the results as markdown: one summary table, then one
// table of the elements that are not ok per state.
func FormatReport(rs []StateResult) string {
	var sb strings.Builder
	sb.WriteString("# Shots: app against prototype\n\nBoxes within the tolerance (1 pt mobile, 2 px tv), same text style, crops with at most 1.5 % of pixels off by more than 32/255 (text crops: -text-max). Elements listed with -except are `waived`.\n\n| state | result | elements | failing | missing | extra | full-screen diff |\n|---|---|---|---|---|---|---|\n")
	for _, r := range rs {
		var n, f, m, x int
		for _, row := range r.Rows {
			switch row.Status {
			case "ok", "waived":
				n++
			case "FAIL":
				n++
				f++
			case "missing":
				m++
			case "extra":
				x++
			default:
				// other statuses are not counted
			}
		}
		full := "-"
		if r.Full != nil {
			full = fmt.Sprintf("%.2f %% (%s)", 100*r.Full.Share, r.Full.Status)
		}
		fmt.Fprintf(&sb, "| %s | %s | %d | %d | %d | %d | %s |\n", r.ID, verdict(r.Pass), n, f, m, x, full)
	}
	for _, r := range rs {
		fmt.Fprintf(&sb, "\n## %s %s\n\n", r.ID, verdict(r.Pass))
		for _, n := range r.Notes {
			fmt.Fprintf(&sb, "- %s\n", n)
		}
		if r.Diff != "" {
			fmt.Fprintf(&sb, "- diff: %s\n", r.Diff)
		}
		sb.WriteString("\n| element | prototype x,y,w,h | app x,y,w,h | max delta | pixels | result |\n|---|---|---|---|---|---|\n")
		rows := append([]Row(nil), r.Rows...)
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Status == "FAIL" && rows[j].Status != "FAIL" })
		for _, row := range rows {
			if row.Status == "ok" {
				continue
			}
			px := "-"
			if row.Pixel != nil {
				px = fmt.Sprintf("%.2f %%", 100*row.Pixel.Share)
			}
			fmt.Fprintf(&sb, "| %s | %s | %s | %.2f | %s | %s %s |\n", row.Key, fmtBox(row.Proto), fmtBox(row.App), row.MaxDelta, px, row.Status, strings.Join(row.Issues, "; "))
		}
	}
	return sb.String()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

const usage = `shotcmp compares the layout.json and PNG of app states with the
prototype's (docs/plan/07-testing.md section 3.3).

  shotcmp -proto <dir> -app <dir> -states K-M02,K-M03 [-mode layout,crop,full]

Both folders hold <ID>.layout.json and <ID>.png. Elements match by (name,
index). Layout: x, y, w, h within the tolerance; fontFamily, fontSize,
fontWeight, color, letterSpacing, lineHeight (elements with text), background,
radius, border, opacity (when the app records them) equal; the text equal.
When the app screen is wider or taller than the prototype's (Android 412 x
892 against 393 x 852), boxes are compared after the stretch rule (widths: both
side margins keep their size; heights: bottom-anchored elements, and what lies
in them, move down) and the pixel modes are skipped. Exit code: 0 pass, 1 differences, 2 usage or read error.

Flags:
`

// multi is a repeatable string flag.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, " ") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// parseMask reads "[ID:]x,y,w,h".
func parseMask(s string) (id string, r Rect, err error) {
	spec := s
	if i := strings.LastIndex(s, ":"); i >= 0 {
		id, spec = s[:i], s[i+1:]
	}
	parts := strings.Split(spec, ",")
	if len(parts) != 4 {
		return "", r, fmt.Errorf("mask %q: want [ID:]x,y,w,h", s)
	}
	var v [4]float64
	for i, p := range parts {
		if v[i], err = strconv.ParseFloat(strings.TrimSpace(p), 64); err != nil {
			return "", r, fmt.Errorf("mask %q: %w", s, err)
		}
	}
	return id, Rect{v[0], v[1], v[2], v[3]}, nil
}

func compileOpt(s string) (*regexp.Regexp, error) {
	if s == "" {
		return nil, nil
	}
	return regexp.Compile(s)
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// stateIDs lists the IDs that have a layout.json in dir.
func stateIDs(dir string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(dir, "*.layout.json"))
	if err != nil || len(m) == 0 {
		return nil, fmt.Errorf("no *.layout.json in %s", dir)
	}
	var ids []string
	for _, p := range m {
		ids = append(ids, strings.TrimSuffix(filepath.Base(p), ".layout.json"))
	}
	sort.Strings(ids)
	return ids, nil
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func verdict(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func fmtBox(b *Box) string {
	if b == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f, %.1f, %.1f, %.1f", b.X, b.Y, b.W, b.H)
}

// matchedOnly returns the rows of the elements present in both layouts
// without comparing anything (the crops need the pairs when -mode has no
// layout).
func matchedOnly(proto, app Layout, o Options) []Row {
	byKey := map[string]Element{}
	for _, e := range app.Elements {
		byKey[e.match()] = e
	}
	var rows []Row
	for _, p := range proto.Elements {
		a, ok := byKey[p.match()]
		if !ok || !o.selected(p.Name) {
			continue
		}
		pb, ab := Box{p.X, p.Y, p.W, p.H}, Box{a.X, a.Y, a.W, a.H}
		rows = append(rows, Row{Key: p.match(), Status: "ok", Proto: &pb, App: &ab})
	}
	return rows
}
