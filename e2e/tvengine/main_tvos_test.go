// Adapted from jplayer-tv/internal/media/avf/main_tvos_test.go at 781bee5 (TestMain, the transparent view, ask, measure, percentiles, waitUntil) and jplayer-tv/e2e/avfcheck/main_tvos_test.go at 781bee5 (recPlane, the underlay adapter).

//go:build ios && tvos

package tvengine

import (
	"context"
	"dancefitvn-app/e2e/e2emedia"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/media/avftv"

	"github.com/gdeskdev/desk/bridge/gpu"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/require"

	_ "github.com/gdeskdev/desk/kits/mobile" // the uikit driver
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The tests run in an app of the Apple TV simulator
// (scripts/tvos-avf-test.sh e2e). They serve the bundle's media (the
// mock's, e2e/prepare-media.sh) on 127.0.0.1 themselves (e2emedia). A test
// asks the script for what only the Mac can do by printing a line
// "avf-test: <request>" (screenshot NAME, background, foreground); the
// script answers by creating avf-ack-<request> (spaces as dashes) in the
// app's temporary directory, which the test waits for. A screenshot is
// also copied there as avf-shot-NAME.png, for the test to read.

// The window: the Apple TV's 1920×1080 points, the video pane in its
// middle at the lesson's 9:16.
var (
	full     = geom.Rect{W: 1920, H: 1080}
	paneRect = geom.XYWH(960-270, 60, 540, 960)
)

// tv is the app the tests run in.
var tv struct {
	app   *host.App
	view  *ui.View
	root  *rootView
	media *e2emedia.Server
	mu    sync.Mutex
	// planes are the underlays the engines made, newest last.
	planes []*recPlane
}

// rootView is the window's content: the video pane alone, on a view left
// transparent, as the TV app's watch screen over the native plane.
type rootView struct {
	ui.Base
	pane *layerPane
}

// layerPane shows the native plane of the engine: it paints nothing,
// leaving the transparent view's hole the system composites the video in,
// and places the plane at its frame in the window whenever that changes
// (cmd/avspike's layerPane). UI thread.
type layerPane struct {
	ui.Base
	eng    *avftv.Engine
	placed geom.Rect
	has    bool
}

// recPlane is the underlay an engine got, with the calls it received.
type recPlane struct {
	inner   platform.Underlay
	layer   uintptr
	frame   geom.Rect
	hidden  bool
	frames  int
	hides   int
	shows   int
	removed bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMain runs the tests in the app UIKit launches, in a goroutine once
// it has: UIApplicationMain never returns, the tests' end ends the
// process.
func TestMain(m *testing.M) {
	app, err := host.New(host.WithName("DanceFITVN TV e2e"), host.WithID("com.jivecode.dancefitvn.tvengine"))
	if err != nil {
		fmt.Println("tvengine:", err)
		os.Exit(1)
	}
	app.OnLaunch(func(context.Context) error {
		srv, err := e2emedia.Start(mediaDir(), log.Printf)
		if err != nil {
			return err
		}
		w, err := app.OpenWindow(host.WindowOptions{Title: "tvengine", Width: full.W, Height: full.H})
		if err != nil {
			return err
		}
		root := &rootView{pane: &layerPane{}}
		root.pane.Name = "video"
		root.Add(root.pane)
		// Transparent, as the app's view is over the native plane: what
		// the pane leaves unpainted shows the underlays.
		opts := []ui.ViewOption{ui.WithTheme(ui.DarkTheme()), ui.WithTransparentBackground()}
		if g, gerr := gpu.New(); gerr == nil {
			opts = append(opts, ui.WithGPU(g))
		}
		view, err := ui.Mount(w, root, opts...)
		if err != nil {
			return err
		}
		tv.app, tv.view, tv.root, tv.media = app, view, root, srv
		go func() {
			code := m.Run()
			time.Sleep(time.Second) // lets the console take the verdict
			os.Exit(code)
		}()

		return nil
	})
	if err := app.Run(); err != nil {
		fmt.Println("tvengine:", err)
		os.Exit(1)
	}
}

// Layout takes the whole window.
func (r *rootView) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(r.pane, ui.Tight(paneRect.Size()))
	c.Place(r.pane, geom.Pt(paneRect.X, paneRect.Y))

	return cs.Constrain(full.Size())
}

// Layout takes the size it is given.
func (v *layerPane) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Constrain(cs.Max) }

// Paint places the engine's plane when its place changed, and paints
// nothing. It allocates nothing.
func (v *layerPane) Paint(*ui.Context, *ui.Painter) {
	if v.eng == nil {
		return
	}
	if r := v.WindowFrame(); !v.has || r != v.placed {
		v.eng.Place(r)
		v.placed, v.has = r, true
	}
}

// show has the pane place eng's plane (nil: none). UI thread.
func (v *layerPane) show(eng *avftv.Engine) {
	if v.eng != nil && v.eng != eng {
		v.eng.Place(geom.Rect{})
	}
	v.eng, v.has = eng, false
	v.Repaint()
}

// SetFrame implements avftv.Plane.
func (p *recPlane) SetFrame(r geom.Rect) {
	p.frame = r
	p.frames++
	p.inner.SetFrame(r)
}

// SetHidden implements avftv.Plane.
func (p *recPlane) SetHidden(h bool) {
	if h != p.hidden {
		if h {
			p.hides++
		} else {
			p.shows++
		}
	}
	p.hidden = h
	p.inner.SetHidden(h)
}

// Remove implements avftv.Plane.
func (p *recPlane) Remove() {
	p.removed = true
	p.inner.Remove()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// underlay is the app's adapter of avftv.Config.Underlay (cmd/avspike's
// underlayOn), with a recorder around the Underlay it gets.
func underlay(layer uintptr) (avftv.Plane, error) {
	if tv.view == nil {
		return nil, fmt.Errorf("tvengine: no view yet: %w", core.ErrInvalid)
	}
	uc, ok := tv.view.Canvas().(platform.UnderlayCanvas)
	if !ok {
		return nil, fmt.Errorf("tvengine: the canvas takes no underlay: %w", core.ErrUnsupported)
	}
	u, err := uc.AddUnderlay(layer)
	if err != nil {
		return nil, fmt.Errorf("tvengine: underlay: %w", err)
	}
	p := &recPlane{inner: u, layer: layer, hidden: true}
	tv.mu.Lock()
	tv.planes = append(tv.planes, p)
	tv.mu.Unlock()

	return p, nil
}

// lastPlane returns the newest underlay an engine made, nil before any.
func lastPlane() *recPlane {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	if len(tv.planes) == 0 {
		return nil
	}

	return tv.planes[len(tv.planes)-1]
}

// mediaDir is where the script put the test media: the app bundle's media
// folder.
func mediaDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "media"
	}

	return filepath.Join(filepath.Dir(exe), "media")
}

// onUI runs fn on the UI thread and waits for it.
func onUI(t *testing.T, fn func()) {
	t.Helper()
	require.NoError(t, tv.app.Invoke(fn))
}

// ask prints a request to the script and waits up to d for its answer;
// false when none came (the tests ran another way).
func ask(request string, d time.Duration) bool {
	ack := filepath.Join(os.TempDir(), "avf-ack-"+strings.ReplaceAll(request, " ", "-"))
	_ = os.Remove(ack)
	fmt.Println("avf-test: " + request)
	for deadline := time.Now().Add(d); time.Now().Before(deadline); {
		if _, err := os.Stat(ack); err == nil {
			_ = os.Remove(ack)

			return true
		}
		wait(50 * time.Millisecond) // polling a file the script creates
	}
	// Once more: a suspended app (an Apple TV, the video paused in the
	// background) wakes past its deadline with the answer there.
	if _, err := os.Stat(ack); err == nil {
		_ = os.Remove(ack)

		return true
	}

	return false
}

// measure prints a measurement line for the report.
func measure(name string, kv ...any) {
	var b strings.Builder
	b.WriteString("avf-measure: " + name)
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	fmt.Println(b.String())
}

// percentiles returns the median and the 95th percentile of ds.
func percentiles(ds []time.Duration) (p50, p95 time.Duration) {
	if len(ds) == 0 {
		return 0, 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)

	return s[len(s)/2], s[min(len(s)-1, len(s)*95/100)]
}

// waitUntil polls cond on the UI thread until it holds, failing after d.
func waitUntil(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()

	for deadline := time.Now().Add(d); ; {
		var ok bool
		onUI(t, func() { ok = cond() })
		if ok {
			return
		}
		require.True(t, time.Now().Before(deadline), "waited %v for %s", d, what)
		wait(50 * time.Millisecond) // polling the player on a real clock
	}
}

// wait lets d of real time pass: the engine plays on the wall clock.
func wait(d time.Duration) {
	timer := time.NewTimer(d)
	<-timer.C
}
