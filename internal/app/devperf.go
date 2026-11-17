package app

import (
	"strconv"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The performance hooks of a developer's build (local/plans/P8.md P8.4a,
// docs/plan/07-testing.md §5): the mark of the first frame of Home
// (scripts/perf-start.sh), the flings of PERF-SCROLL (perf-scroll.sh) and
// the long play of PERF-VIDEO10 (perf-video.sh), whose states live in
// devperf_mobile.go and devperf_tv.go. The build that ships has none of
// them: every use is behind devBuild.

// appInit is when the app package was initialised: the start of a launch
// where the system's start of the process cannot be read.
var appInit = time.Now()

// Flings of PERF-SCROLL: a finger drags a share of the window's height
// (flingLong on a long list, flingShort on one that scrolls a few hundred
// DIPs, which the long drag alone would take to its end) in flingMoves
// moves of flingStep each (a quick flick), then lifts; the next fling, the
// other way, comes flingGap later, once the glide has ended.
const (
	flingMoves = 6
	flingStep  = 8 * time.Millisecond
	flingLong  = 0.42
	flingShort = 0.12
	flingGap   = 1500 * time.Millisecond
	// defaultFlings is how many flings PERF-SCROLL makes without
	// DFV_PERF_FLINGS.
	defaultFlings = 20
	// defaultVideoMin is how many minutes PERF-VIDEO10 plays without
	// DFV_PERF_MIN.
	defaultVideoMin = 10
	// perfVideoEvery is how often PERF-VIDEO10 logs where the player
	// stands.
	perfVideoEvery = 30 * time.Second
)

// homeMark logs, once, "dancefitvn: mark home <ms> t=<unix ms> from=<start>"
// after the first frame that shows Home with its data: ms since the
// process started (from=process) or, where its start cannot be read,
// since the app package's initialisation (from=init). Its after is the
// view's AfterFrame; once marked it does nothing, allocating nothing.
type homeMark struct {
	shown func() bool
	start time.Time
	from  string
	now   func() time.Time
	logf  func(format string, args ...any)
	done  bool
}

// perfFlinger flings the list under the middle of the window, up then
// down, n times, flingGap apart, logging "dfv: perf scroll start|end <id>
// t=<unix ms>" around them. UI thread.
type perfFlinger struct {
	id    string
	v     *ui.View
	n     int
	dist  float32 // the share of the window's height a drag covers
	logf  func(format string, args ...any)
	epoch time.Time // the events' clock: their Time is the time since
	next  int
	done  bool
	step  func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newHomeMark returns the mark of the first frame shown tells, logged
// with logf.
func newHomeMark(shown func() bool, logf func(format string, args ...any)) *homeMark {
	m := &homeMark{shown: shown, now: time.Now, logf: logf, from: "process"}
	start, err := sys.ProcessStart()
	if err != nil {
		start, m.from = appInit, "init"
	}
	m.start = start

	return m
}

// newPerfFlinger returns n flings of state id on v, each drag covering
// dist of the window's height.
func newPerfFlinger(id string, v *ui.View, n int, dist float32, logf func(format string, args ...any)) *perfFlinger {
	f := &perfFlinger{id: id, v: v, n: n, dist: dist, logf: logf, epoch: time.Now()}
	f.step = f.fling

	return f
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// after marks the frame just drawn when it is the first to show Home.
func (m *homeMark) after() {
	if m.done || !m.shown() {
		return
	}
	m.done = true
	now := m.now()
	m.logf("dancefitvn: mark home %d t=%d from=%s", now.Sub(m.start).Milliseconds(), now.UnixMilli(), m.from)
}

// start logs the start and makes the first fling.
func (f *perfFlinger) start() {
	f.logf("dfv: perf scroll start %s t=%d flings=%d", f.id, time.Now().UnixMilli(), f.n)
	f.fling()
}

// Done tells whether every fling was made and the last one's glide had
// its time.
func (f *perfFlinger) Done() bool { return f.done }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// fling makes the next fling, or ends the run.
func (f *perfFlinger) fling() {
	if f.next == f.n {
		f.done = true
		f.logf("dfv: perf scroll end %s t=%d", f.id, time.Now().UnixMilli())

		return
	}
	r := f.v.Root().Node().WindowFrame()
	lo, hi := geom.Pt(r.W/2, r.H*0.72), geom.Pt(r.W/2, r.H*(0.72-f.dist))
	if f.next%2 == 1 {
		lo, hi = hi, lo
	}
	flingWindow(f.v, lo, hi, time.Since(f.epoch))
	f.next++
	f.v.Context().AfterFunc(flingGap, f.step)
}

// flingWindow drags a finger on v from `from` to `to` in flingMoves
// moves flingStep apart, timed by the events from t0 on (a scroll
// measures the finger's speed on them, PointerEvent.Time), then lifts it.
func flingWindow(v *ui.View, from, to geom.Point, t0 time.Duration) {
	ev := platform.Event{Kind: platform.EventPointerDown, X: from.X, Y: from.Y, Button: platform.ButtonPrimary,
		Count: 1, Pointer: platform.PointerTouch, PointerID: 1, Time: t0}
	v.HandleEvent(ev)
	ev.Count = 0
	ev.Kind = platform.EventPointerMove
	for i := 1; i <= flingMoves; i++ {
		k := float32(i) / flingMoves
		ev.X, ev.Y = from.X+(to.X-from.X)*k, from.Y+(to.Y-from.Y)*k
		ev.Time = t0 + time.Duration(i)*flingStep
		v.HandleEvent(ev)
	}
	ev.Kind = platform.EventPointerUp
	ev.Count = 1
	v.HandleEvent(ev)
}

// chainFrames returns one AfterFrame running each of fns that is not nil,
// in order; nil when none is.
func chainFrames(fns ...func()) func() {
	var list []func()
	for _, fn := range fns {
		if fn != nil {
			list = append(list, fn)
		}
	}
	switch len(list) {
	case 0:
		return nil
	case 1:
		return list[0]
	default:
		return func() {
			for _, fn := range list {
				fn()
			}
		}
	}
}

// sayPerfVideo logs where PERF-VIDEO10's player stands: "dfv: perf
// video <what> t=<unix ms> pos=<s> dur=<s> state=<n> loop=<bool>
// tick=<s>", and " media=<name>" when media is not empty. pos is the
// engine's own reading, which Player.SyncPos keeps (as Now Playing reads
// it, nowplaying.go); tick is the position the last time event told, the
// one the video screen shows, read before. The two part when the ticks
// stop or tell a stale time (local/p8/perf/report.md note 14).
func sayPerfVideo(logf func(format string, args ...any), p *player.Player, what, media string, now time.Time) {
	tick := p.Pos()
	p.SyncPos()
	line := "dfv: perf video %s t=%d pos=%.2f dur=%.2f state=%d loop=%t tick=%.2f"
	args := []any{what, now.UnixMilli(), p.Pos(), p.Dur(), p.State(), p.Loop(), tick}
	if media != "" {
		line += " media=%s"
		args = append(args, media)
	}
	logf(line, args...)
}

// envCount is the developer's variable key as a whole number above 0, or
// def when unset or not one.
func envCount(key string, def int) int {
	n, err := strconv.Atoi(devEnv(key))
	if err != nil || n <= 0 {
		return def
	}

	return n
}
