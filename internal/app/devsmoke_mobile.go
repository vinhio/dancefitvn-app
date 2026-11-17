//go:build dancefitvndev && !tvos

// The smoke states of the playback's wiring (P4.6a), for a developer's
// build only. DFV_STATE=SMOKE-PLAY signs in with taps on the window, opens
// Set 35, plays the video of lesson 4 through the system's engine, goes
// back (the video pauses and its position is saved), then plays the sound
// of lesson 3. The script around it sends the app to the background
// and back (local/p4/wire/). DFV_STATE=SMOKE-RESUME plays the video of
// lesson 3, whose resume position lies past the end of its mock stream
// (P4.9 D2). They add themselves to the shot states here, so that the
// states of the tests (built without the tag) stay as they are.
//
// P4.9 D3 adds the proofs of the progress saves on the device
// (local/p4/device-proof/): SMOKE-SAVE (saves at the 10 s cadence of a
// looped video, on Back, on the system's background), SMOKE-WATCHED (a video
// played to its end: the watched save) and SMOKE-AB (the A–B loop through
// the player). Each logs every lessons.progress request with its body.

package app

import (
	"context"
	"log"
	"time"

	"dancefitvn-app/internal/api"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The ids of the smoke states.
const (
	smokeState       = "SMOKE-PLAY"
	smokeResumeState = "SMOKE-RESUME"
	smokeSaveState   = "SMOKE-SAVE"
	smokeWatchState  = "SMOKE-WATCHED"
	smokeABState     = "SMOKE-AB"
)

// The video of the smoke: lesson 4 of Set 35, at 0 in the course data.
// Lesson 3's resume position (72 s) lies past the end of its 12 s mock
// stream: SMOKE-RESUME plays it.
const (
	smokeVideoID = "l_3504"
	smokeVideoNo = 4
)

func init() {
	steps := append(typeAndTapSignIn(), homeLoaded(), openDetail(demoCourse), detailLoaded(),
		stateStep{what: "play the video", act: func(sh *shell.Shell, _ *mobileWire) {
			sh.PlayVideo(demoCourse, smokeVideoID, smokeVideoNo)
		}},
		routeIs(env.RouteVideo),
		smokeUntil("the video plays 3 s", player.KindVideo, 3),
		stateStep{what: "back to the detail", act: func(sh *shell.Shell, w *mobileWire) {
			smokeLog(w, "video before back")
			sh.Back()
		}},
		routeIs(env.RouteDetail),
		stateStep{what: "play the audio", act: func(sh *shell.Shell, w *mobileWire) {
			smokeLog(w, "video after back")
			sh.PlayAudio(demoCourse, demoLessonID, demoLessonNo)
		}},
		smokeUntil("the audio plays 2 s", player.KindAudio, 2),
		stateStep{what: "log", act: func(_ *shell.Shell, w *mobileWire) { smokeLog(w, "audio playing") }},
	)
	shotStates[smokeState] = shotState{prepare: signedOutFresh, steps: steps}

	// The resume position past the media's end: the player drops it once
	// the engine tells the length, and the video plays from 0 on 06; it
	// must not jump to the end and on to 10.
	resume := append(typeAndTapSignIn(), homeLoaded(), openDetail(demoCourse), detailLoaded(),
		stateStep{what: "play the video of lesson 3", act: func(sh *shell.Shell, _ *mobileWire) {
			sh.PlayVideo(demoCourse, demoLessonID, demoLessonNo)
		}},
		routeIs(env.RouteVideo),
		smokeUntil("the video plays", player.KindVideo, 0.1),
		stateStep{what: "log the start", act: func(sh *shell.Shell, w *mobileWire) { smokeRoute(sh, w, "resume started") }},
		smokeUntil("the video plays 3 s", player.KindVideo, 3),
		stateStep{what: "log 3 s", act: func(sh *shell.Shell, w *mobileWire) { smokeRoute(sh, w, "resume at 3 s") }},
		smokeUntil("the video plays 6 s", player.KindVideo, 6),
		stateStep{what: "log 6 s", act: func(sh *shell.Shell, w *mobileWire) { smokeRoute(sh, w, "resume at 6 s") }},
	)
	shotStates[smokeResumeState] = shotState{prepare: signedOutFresh, steps: resume}

	registerProofStates()
}

// registerProofStates adds the states of the progress proofs (P4.9 D3).
func registerProofStates() {
	play := func(what string) []stateStep {
		return append(typeAndTapSignIn(), homeLoaded(), openDetail(demoCourse), detailLoaded(),
			stateStep{what: what, act: func(sh *shell.Shell, _ *mobileWire) {
				sh.PlayVideo(demoCourse, smokeVideoID, smokeVideoNo)
			}},
			routeIs(env.RouteVideo),
			smokeUntil("the video plays 0.5 s", player.KindVideo, 0.5),
		)
	}
	var t0 time.Time
	mark := func() stateStep {
		return stateStep{what: "mark", act: func(*shell.Shell, *mobileWire) { t0 = time.Now() }}
	}
	wall := func(what string, d time.Duration) stateStep {
		return stateStep{what: what, until: func(*shell.Shell, *mobileWire) bool { return time.Since(t0) >= d }}
	}
	loopOn := func(a, b float64) stateStep {
		return stateStep{what: "A-B loop on", act: func(_ *shell.Shell, w *mobileWire) {
			p := w.pb.player
			p.SetA(a)
			p.SetB(b)
			p.SetLoop(true)
			smokeLog(w, "loop on")
		}}
	}
	rate := func(r float64) stateStep {
		return stateStep{what: "rate", act: func(_ *shell.Shell, w *mobileWire) { w.pb.player.SetRate(r) }}
	}
	spy := stateStep{what: "watch the player", act: func(_ *shell.Shell, w *mobileWire) { smokeWatch(w) }}
	say := func(msg string) stateStep {
		return stateStep{what: msg, act: func(_ *shell.Shell, w *mobileWire) { smokeLog(w, msg) }}
	}

	// SAVE: the video of lesson 4 plays at 0.5 and loops inside A=0.5 .. B=11.5
	// for 36 s of wall time (saves at the 10 s cadence, at positions that
	// differ: the tracker sends no position twice in a row); Back (a save); play again and wait
	// for the script to send the app to the background and back.
	save := append(play("play the video"), spy, rate(0.5), loopOn(0.5, 11.5), mark(), say("PHASE 1 saves every 10 s: begin"),
		wall("36 s of play", 36*time.Second), say("PHASE 1 end"),
		stateStep{what: "back", act: func(sh *shell.Shell, w *mobileWire) {
			smokeLog(w, "PHASE 2 Back pressed")
			sh.Back()
		}},
		routeIs(env.RouteDetail), mark(), wall("2 s after Back", 2*time.Second),
		stateStep{what: "play again", act: func(sh *shell.Shell, _ *mobileWire) {
			sh.PlayVideo(demoCourse, smokeVideoID, smokeVideoNo)
		}},
		routeIs(env.RouteVideo),
		smokeUntil("the video plays 2 s", player.KindVideo, 2), rate(0.5),
		say("PHASE 3 ready for the background"), mark(), wall("hold 25 s for the script", 25*time.Second),
		say("PHASE 3 end"),
	)
	shotStates[smokeSaveState] = shotState{prepare: smokePrepare, steps: save}

	// WATCHED: the video plays to its end (12 s at the saved rate).
	watched := append(play("play the video"), spy, rate(1), mark(), say("watched: begin"),
		wall("past the end and the countdown", 24*time.Second), say("watched: end"))
	shotStates[smokeWatchState] = shotState{prepare: smokePrepare, steps: watched}

	// AB: A=2, B=6 inside the 12 s stream: the loop jumps back at B.
	ab := append(play("play the video"), spy, rate(1), loopOn(2, 6), mark(), say("ab: begin"),
		wall("20 s of loop", 20*time.Second), say("ab: end"))
	shotStates[smokeABState] = shotState{prepare: smokePrepare, steps: ab}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// smokeUntil waits until the player plays a media of kind past sec
// seconds of it.
func smokeUntil(what string, kind player.Kind, sec float64) stateStep {
	return stateStep{what: what, when: func(sh *shell.Shell) bool {
		pb := sh.Env().Playback()

		return pb != nil && pb.Kind() == kind && pb.State() == player.StatePlaying && pb.Pos() >= sec
	}}
}

// smokeRoute writes the screen shown and where the player stands.
func smokeRoute(sh *shell.Shell, w *mobileWire, what string) {
	smokeLog(w, what+", screen "+sh.Nav().Current().Screen)
}

// smokeLog writes where the player stands, for the smoke's evidence.
func smokeLog(w *mobileWire, what string) {
	p := w.pb.player
	log.Printf("dancefitvn: smoke: %s: media %s, lesson %s, kind %d, state %d, pos %.2f of %.2f, rate %.2f",
		what, w.pb.sys.name, p.Lesson().ID, p.Kind(), p.State(), p.Pos(), p.Dur(), p.Rate())
}

// smokePrepare signs out as signedOutFresh does and logs every progress
// request of the API (with its answer) from now on.
func smokePrepare(ctx context.Context, w *mobileWire) error {
	if err := signedOutFresh(ctx, w); err != nil {
		return err
	}
	w.tr.mu.Lock()
	w.tr.inner = smokeTransport{inner: w.tr.inner}
	w.tr.mu.Unlock()

	return nil
}

// smokeTransport logs the requests of lessons.progress and progress.batch.
type smokeTransport struct{ inner api.Transport }

// Do hands the request on and logs it with its status.
func (t smokeTransport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	res, err := t.inner.Do(ctx, req)
	if req.RouteID == "lessons.progress" || req.RouteID == "progress.batch" {
		st, answer := 0, []byte(nil)
		if res != nil {
			st, answer = res.Status, res.Body
		}
		log.Printf("dancefitvn: smoke: WIRE %s %s status %d err %v body %s -> %s", req.Method, req.Path, st, err, req.Body, answer)
	}

	return res, err
}

// smokeWatch logs the player's state changes, its seeks (an A–B jump with
// the last tick before it and the loop's points) and its end.
func smokeWatch(w *mobileWire) {
	p := w.pb.player
	var prev float64
	p.Changes().Subscribe(func(c player.Change) {
		switch c.Kind {
		case player.ChangeTime:
			if p.Pos() != p.A() { // the jump itself reports A
				prev = p.Pos()
			}
		case player.ChangeSeek:
			log.Printf("dancefitvn: smoke: EVENT seek to %.2f (loop %t, A %.2f, B %.2f, previous tick %.2f, state %d)",
				p.Pos(), p.Loop(), p.A(), p.B(), prev, p.State())
		case player.ChangeState:
			log.Printf("dancefitvn: smoke: EVENT state %d at %.2f", p.State(), p.Pos())
		case player.ChangeEnded:
			log.Printf("dancefitvn: smoke: EVENT ended at %.2f of %.2f", p.Pos(), p.Dur())
		case player.ChangeUnknown, player.ChangeLesson, player.ChangeDuration, player.ChangeRate,
			player.ChangeOptions, player.ChangeError:
		default:
		}
	})
}
