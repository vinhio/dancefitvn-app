//go:build dancefitvndev && tvos

// The performance state of the Apple TV (local/plans/P8.md P8.4a), for a
// developer's build only: PERF-VIDEO10 pairs as TV04 does, plays the
// video of lesson 3 of Set 35 through the system's engine (avftv, the
// native plane) in an A–B loop over the whole media, DFV_PERF_MIN minutes
// (10), with a mem line a second (DFV_FRAMELOG=1: scripts/perf-video.sh
// tvos).

package app

import (
	"time"

	"dancefitvn-app/internal/player"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// perfVideoState is the id of the TV's performance state.
const perfVideoState = "PERF-VIDEO10"

func init() { //nolint:gochecknoinits // the developer's state joins the table
	tvShotStates[perfVideoState] = tvShotState{knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"),
		steps: tvAfterHome(tvPerfVideoSteps()...), shows: []string{"p2", "pn0"}}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tvPerfVideoSteps read the course, play lesson 3 as a video on TV07, loop
// it over the whole media and keep it playing DFV_PERF_MIN minutes,
// logging where it stands every perfVideoEvery: "dfv: perf video
// start|at|end t=<unix ms> …".
func tvPerfVideoSteps() []tvStep {
	var t0, last time.Time
	minutes := 0
	say := func(r *tvStateRunner, what string) {
		sayPerfVideo(r.w().deps.Logf, r.w().pb.player, what, "", time.Now())
	}

	return append(tvReadCourse(),
		tvPlay("play video 3", func(r *tvStateRunner, l player.Lesson, list []player.Lesson) {
			r.w().pb.PlayVideo(l, list)
			r.sh().Go(nav.Route{Screen: tvenv.RoutePlayer}, false)
		}),
		tvRouteIs(tvenv.RoutePlayer), tvLoaded(),
		tvStep{what: "playing 0.5 s", until: func(r *tvStateRunner) bool {
			p := r.w().pb.player

			return p.State() == player.StatePlaying && p.Pos() >= 0.5
		}},
		tvStep{what: "loop on", act: func(r *tvStateRunner) {
			p := r.w().pb.player
			p.SetLoop(true)
			p.SetAB(0, p.Dur()) // after the loop is on: it begins at the position, K38
			minutes = envCount(perfMinEnv, defaultVideoMin)
			t0, last = time.Now(), time.Now()
			r.w().deps.Logf("dfv: perf video min=%d", minutes)
			say(r, "start")
		}},
		tvStep{what: "play", until: func(r *tvStateRunner) bool {
			if time.Since(last) >= perfVideoEvery {
				last = time.Now()
				say(r, "at")
			}
			if time.Since(t0) < time.Duration(minutes)*time.Minute {
				return false
			}
			say(r, "end")

			return true
		}},
	)
}
