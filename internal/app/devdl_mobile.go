//go:build dancefitvndev && !tvos

// The smoke states of the downloads' wiring (P5.1c), for a developer's build
// only, run on the simulator by local/p5/wire's script with scripts/net.sh
// and the variables DFV_DL_KBPS, DFV_BADSHA and DFV_NETWORK:
//
//   - SMOKE-DL signs in, opens Set 35 and confirms the download of lesson
//     DFV_DL_LESSON (5 when unset) as sheet 09's "Tải xuống" does
//     (Shell.StartDownloads: its toasts), then stays on the detail. Every
//     event of the downloads, every change of the network and of the
//     app's online state is logged with the toast on show.
//   - SMOKE-OFFLINE signs in, opens Set 35 and waits for the offline screen
//     (the script cuts the network), then plays the audio of lesson
//     DFV_DL_LESSON from 13 (`offTap`): the file on the device.
//   - SMOKE-M11-RUNNING is SMOKE-DL on screen 11 (the running card; the
//     prefix keeps it out of the shot states the tests check).
//
// They add themselves to the shot states here, so that the states of the
// tests (built without the tag) stay as they are.

package app

import (
	"log"
	"strconv"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// dlLessonEnv names the lesson number of Set 35 the download smokes take;
// 5 when unset.
const dlLessonEnv = "DFV_DL_LESSON"

// The ids of the download smokes.
const (
	smokeDLState      = "SMOKE-DL"
	smokeOfflineState = "SMOKE-OFFLINE"
	smokeRunningState = "SMOKE-M11-RUNNING"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

func init() {
	open := []stateStep{homeLoaded(), openDetail(demoCourse), detailLoaded(), dlSpy()}
	start := stateStep{what: "start the download", act: func(sh *shell.Shell, _ *mobileWire) {
		c, ok := sh.Env().CachedCourse(demoCourse)
		l, found := dlLesson(c)
		if !ok || !found {
			log.Printf("dancefitvn: smoke: DL no lesson %d in %s", dlLessonNo(), demoCourse)

			return
		}
		log.Printf("dancefitvn: smoke: DL confirm %s (no %d), network %s, online %t", l.ID, l.No, sh.Env().Services.(*mobileWire).netKind(), sh.Env().Online())
		sh.StartDownloads(c, []model.Lesson{l})
		dlToast(sh, "after confirm")
	}}
	shotStates[smokeDLState] = shotState{prepare: signedIn(false), steps: append(open, start)}
	shotStates[smokeRunningState] = shotState{prepare: signedIn(false), steps: append(open, start,
		goRoute(env.RouteDownloads, nil), routeIs(env.RouteDownloads))}

	offline := append(open,
		routeIs(env.RouteOffline),
		stateStep{what: "play the file from 13", act: func(sh *shell.Shell, w *mobileWire) {
			c, _ := sh.Env().CachedCourse(demoCourse)
			l, _ := dlLesson(c)
			path, ok := "", false
			if m := w.downloads(); m != nil {
				path, ok = m.Path(l.ID)
			}
			log.Printf("dancefitvn: smoke: DL offline: %s on the device %t (%s), online %t", l.ID, ok, path, w.Online())
			sh.OffTap(demoCourse, l.ID, l.No)
		}},
		smokeUntil("the audio plays 3 s", player.KindAudio, 3),
		stateStep{what: "log", act: func(sh *shell.Shell, w *mobileWire) {
			smokeRoute(sh, w, "DL offline audio playing")
		}},
	)
	shotStates[smokeOfflineState] = shotState{prepare: signedIn(false), steps: offline}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dlSpy logs every event of the downloads (the progress at each tenth),
// the network's kinds and the online state, with the toast on show.
func dlSpy() stateStep {
	return stateStep{what: "watch the downloads", act: func(sh *shell.Shell, w *mobileWire) {
		last := -1
		w.dls.Events().Subscribe(func(ev env.DownloadEvent) {
			if ev.Kind == env.DownloadEventProgress {
				if ev.Total <= 0 || int(ev.Done*10/ev.Total) == last {
					return
				}
				last = int(ev.Done * 10 / ev.Total)
			}
			log.Printf("dancefitvn: smoke: DL event %d %s no %d %d/%d err %v", ev.Kind, ev.LessonID, ev.No, ev.Done, ev.Total, ev.Err)
			w.dispatch(func() { dlToast(sh, "after event") }) // after the shell's own listener
		})
		w.WatchOnline(func(on bool) {
			log.Printf("dancefitvn: smoke: DL online %t, network %s, screen %s", on, w.netKind(), sh.Nav().Current().Screen)
		})
		log.Printf("dancefitvn: smoke: DL network %s at the start, online %t", w.netKind(), w.Online())
	}}
}

// dlToast logs the toast on show.
func dlToast(sh *shell.Shell, when string) {
	if t := sh.Nav().Toasts(); t != nil && t.Visible() {
		log.Printf("dancefitvn: smoke: DL toast %s: %q", when, t.Message())
	}
}

// dlLessonNo is DFV_DL_LESSON, 5 when unset.
func dlLessonNo() int {
	if n, err := strconv.Atoi(devEnv(dlLessonEnv)); err == nil && n > 0 {
		return n
	}

	return 5
}

// dlLesson returns the lesson of c the smokes take.
func dlLesson(c model.Course) (model.Lesson, bool) {
	no := dlLessonNo()
	for _, l := range c.Lessons {
		if l.No == no {
			return l, true
		}
	}

	return model.Lesson{}, false
}
