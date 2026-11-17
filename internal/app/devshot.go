package app

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"time"

	"dancefitvn-app/internal/ui/kit/layoutdump"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// shotFolder, under the app's cache folder, holds the layouts a
// developer's build writes: <ID>.layout.json (scripts/shot.sh reads them).
const shotFolder = "dfv-shots"

// shotPoll is how often the shot looks whether the app rests.
const shotPoll = 100 * time.Millisecond

// shotWallQuiet is the least real time the app rests before the shot, beside
// settle on the view's clock: the work ui.Go starts reaches the transport
// only once its goroutine runs, and a Tester's clock, which runs as fast as
// frames are drawn, can count a whole settle before that.
const shotWallQuiet = 300 * time.Millisecond

// shotChunk is the size of one base64 line.
const shotChunk = 3000

// devShot writes the layout of the state on show once the app has rested
// settle without a request on the view's clock, and shotWallQuiet in real
// time (docs/plan/07-testing.md §3.3). A request under way, or one answered
// since the last look, starts the rest again. The phone's and the TV's
// shells share it (root_mobile.go, root_tv.go).
type devShot struct {
	id       string
	platform string // "mobile" or "tv", layoutdump's
	v        *ui.View
	src      shotSource
	settle   time.Duration
	quiet    time.Duration
	served   int
	// since is the real time the rest began; wall reads the real clock.
	since time.Time
	wall  func() time.Time
	poll  func()
	// written tells the layout is on disk.
	written bool
}

// shotSource is what the shot reads of the services: their requests, their
// cache folder and their log.
type shotSource struct {
	// idle tells whether no request is under way and how many were
	// answered (devTransport.idle); nil is always idle.
	idle     func() (idle bool, served int)
	cacheDir func() (string, error)
	logf     func(format string, args ...any)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDevShot returns the shot of state id of the platform's layout on v.
func newDevShot(id, platform string, v *ui.View, src shotSource, settle time.Duration) *devShot {
	d := &devShot{id: id, platform: platform, v: v, src: src, settle: settle, served: -1, wall: time.Now}
	d.poll = d.check

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// arm starts waiting for the rest: the state's last step ran.
func (d *devShot) arm() {
	d.since = d.wall()
	d.v.Context().AfterFunc(shotPoll, d.poll)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// check counts the time without a request, and writes once it is settle on
// the view's clock and shotWallQuiet in real time.
func (d *devShot) check() {
	idle, served := true, 0
	if d.src.idle != nil {
		idle, served = d.src.idle()
	}
	if !idle || served != d.served {
		d.quiet, d.served, d.since = 0, served, d.wall()
	} else {
		d.quiet += shotPoll
	}
	if d.quiet < d.settle || d.wall().Sub(d.since) < shotWallQuiet {
		d.v.Context().AfterFunc(shotPoll, d.poll)

		return
	}
	d.write()
}

// write dumps the layout of the window into <cache>/dfv-shots and logs
// where.
func (d *devShot) write() {
	root := d.v.Root()
	r := root.Node().WindowFrame()
	lay := layoutdump.Dump(d.v.Context(), root, d.id, d.platform, geom.Sz(r.W, r.H), nil)
	logf := d.src.logf
	if devEnv(shotLogEnv) == "1" {
		logLayout(lay, logf)
	}
	cache, err := d.src.cacheDir()
	if err != nil {
		logf("dfv: shot %s: %v", d.id, err)

		return
	}
	dir := filepath.Join(cache, shotFolder)
	if err := lay.Write(dir, nil); err != nil {
		logf("dfv: shot %s: %v", d.id, err)

		return
	}
	d.written = true
	logf("dfv: shot ready %s %s", d.id, filepath.Join(dir, d.id+".layout.json"))
	if devBuild && devEnv(a11yEnv) == "1" { // the accessibility dump (devaccess.go)
		a, err := newDevA11y(d.id, d.v, dir, devEnv(a11yMoveEnv), logf)
		if err != nil {
			logf("dfv: a11y %s: %v", d.id, err)

			return
		}
		a.start()
	}
}

// logLayout logs the layout with logf as base64 lines of shotChunk
// characters.
func logLayout(lay layoutdump.Layout, logf func(format string, args ...any)) {
	data, err := json.Marshal(lay)
	if err != nil {
		logf("dfv: shot %s: %v", lay.ID, err)

		return
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	n := (len(b64) + shotChunk - 1) / shotChunk
	for i := range n {
		end := min((i+1)*shotChunk, len(b64))
		logf("dfv:L %s %d/%d %s", lay.ID, i+1, n, b64[i*shotChunk:end])
	}
}
