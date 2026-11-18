//go:build dancefitvndev && !tvos

// The performance states of the phone (local/plans/P8.md P8.4a), for a
// developer's build only, as the smoke states (devsmoke_mobile.go):
// PERF-SCROLL flings the list of Courses (M04), PERF-SCROLL-M05 the
// detail of Set 35 (M05), DFV_PERF_FLINGS times (20), with the frame log
// on (DFV_FRAMELOG=1 GDESK_FRAMESTATS=1: scripts/perf-scroll.sh);
// PERF-VIDEO10 plays the video of lesson 4 of Set 35 through the system's
// engine in an A–B loop over the whole media, DFV_PERF_MIN minutes (10),
// with a mem line a second (DFV_FRAMELOG=1: scripts/perf-video.sh).

package app

import (
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The ids of the performance states.
const (
	perfScrollState    = "PERF-SCROLL"
	perfScrollM05State = "PERF-SCROLL-M05"
	perfVideoState     = "PERF-VIDEO10"
)

func init() { //nolint:gochecknoinits // the developer's states join the table
	shotStates[perfScrollState] = shotState{knobs: demo, prepare: signedIn(false),
		steps: append(coursesSteps(func(courses) {}), perfFlings(perfScrollState, flingLong)...)}
	shotStates[perfScrollM05State] = shotState{knobs: demo, prepare: signedIn(true),
		steps: append([]stateStep{homeLoaded(), openDetail(demoCourse), detailLoaded()}, perfFlings(perfScrollM05State, flingShort)...)}
	shotStates[perfVideoState] = shotState{prepare: signedIn(false), steps: perfVideoSteps()}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// perfFlings rests a second on the list (its pictures in), then flings it
// DFV_PERF_FLINGS times, each drag dist of the window's height, and waits
// for the last glide.
func perfFlings(id string, dist float32) []stateStep {
	var f *perfFlinger
	var t0 time.Time

	return []stateStep{
		{what: "rest", act: func(*shell.Shell, *mobileWire) { t0 = time.Now() }},
		{what: "rested", until: func(*shell.Shell, *mobileWire) bool { return time.Since(t0) >= time.Second }},
		{what: "fling", act: func(_ *shell.Shell, w *mobileWire) {
			w.mu.Lock()
			v := w.view
			w.mu.Unlock()
			if v == nil {
				return
			}
			f = newPerfFlinger(id, v, envCount(perfFlingsEnv, defaultFlings), dist, w.deps.Logf)
			f.start()
		}},
		{what: "flings done", until: func(*shell.Shell, *mobileWire) bool { return f == nil || f.Done() }},
	}
}

// perfVideoSteps play the video of lesson 4 of Set 35 on the system's
// engine, loop it over the whole media (A at 0, B at its end) and keep it
// playing DFV_PERF_MIN minutes, logging where it stands every
// perfVideoEvery: "dfv: perf video start|at|end t=<unix ms> …".
func perfVideoSteps() []stateStep {
	var t0, last time.Time
	minutes := 0
	say := func(w *mobileWire, what string) {
		sayPerfVideo(w.deps.Logf, w.pb.player, what, w.pb.sys.name, time.Now())
	}

	return []stateStep{
		homeLoaded(), openDetail(demoCourse), detailLoaded(),
		{what: "play the video", act: func(sh *shell.Shell, _ *mobileWire) {
			sh.PlayVideo(demoCourse, smokeVideoID, smokeVideoNo)
		}},
		routeIs(env.RouteVideo),
		smokeUntil("the video plays 0.5 s", player.KindVideo, 0.5),
		{what: "loop on", act: func(_ *shell.Shell, w *mobileWire) {
			p := w.pb.player
			p.SetLoop(true)
			p.SetAB(0, p.Dur()) // after the loop is on: it begins at the position, K38
			minutes = envCount(perfMinEnv, defaultVideoMin)
			t0, last = time.Now(), time.Now()
			w.deps.Logf("dfv: perf video min=%d", minutes)
			say(w, "start")
		}},
		{what: "play", until: func(_ *shell.Shell, w *mobileWire) bool {
			if time.Since(last) >= perfVideoEvery {
				last = time.Now()
				say(w, "at")
			}
			if time.Since(t0) < time.Duration(minutes)*time.Minute {
				return false
			}
			say(w, "end")

			return true
		}},
	}
}
