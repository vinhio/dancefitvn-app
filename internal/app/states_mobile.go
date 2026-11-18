//go:build !tvos

package app

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// demoNow is the clock of the shot states that show it: Saturday 14
// November 2026, 09:41 at +07:00, the time of the prototype's screens and
// of the mock's server_time.
var demoNow = time.Date(2026, time.November, 14, 9, 41, 0, 0, time.FixedZone("+07", 7*3600))

// The prototype's downloads (`S.dl`, index.html): lessons 1, 2, 3, 4 and 6
// of Set 35 have their audio, 7 is at 64 %; 18,2 GB of the disk are free,
// which is 71 % full (K11, M11's bar).
const (
	demoCourse   = "set-35"
	demoIng      = 7
	demoIngShare = float32(.64)
	// demoFree and demoTotal: 18 200 000 000 free of 62 758 620 690 is 71 %
	// used.
	demoFree  int64 = 18_200_000_000
	demoTotal int64 = 62_758_620_690
)

// demoDone are the lesson numbers of the demo course with their audio.
var demoDone = [...]int{1, 2, 3, 4, 6}

// seedEmail is the email the states sign in with to reach the mock's
// session; the email remembered is the one of the mock's user.
const seedEmail = "seed@example.invalid"

// seedPassword is the password of that sign-in: the mock takes any.
const seedPassword = "x"

// stepPoll is how often a state's runner looks whether its next step may
// run.
const stepPoll = 50 * time.Millisecond

// tapTries is how many times a tap that must settle is made at most.
const tapTries = 5

// shotState is a state of the shot script (docs/plan/07-testing.md §3.3):
// how the app is brought to it through the real flow over the mock (D7).
type shotState struct {
	// hold keeps the splash (shell.Options.HoldSplash).
	hold bool
	// knobs sets what the state pins before the shell starts.
	knobs func(k *wireKnobs)
	// prepare walks the mock to the state's start: a session signed in,
	// signed out with the biometric copy kept, a scenario, a held route.
	// It runs off the UI thread, before the session is read.
	prepare func(ctx context.Context, w *mobileWire) error
	// steps run on the UI thread once the shell is mounted, in order.
	steps []stateStep
	// diskOverlay keeps the mock's overlay in <data>/mock-overlay.json, as the
	// app does: a state whose changes the next launch must read (INFO-SAVE).
	// The others keep it in memory, empty at launch.
	diskOverlay bool
}

// stateStep is one step of a state: once when holds, act runs (nil acts
// nothing: the step only waits).
type stateStep struct {
	what string
	when func(sh *shell.Shell) bool
	// until is when with the services at hand; both must hold.
	until func(sh *shell.Shell, w *mobileWire) bool
	act   func(sh *shell.Shell, w *mobileWire)
	// target, when set, is the widget act taps: the step waits until it is laid
	// out (a frame of some size) and its frame has not moved since the last poll,
	// so a tap never lands at a stale (0, 0).
	target func(sh *shell.Shell) ui.Widget
	// settled, when set, tells whether the tap had its effect; if not, act runs
	// again, at most tapTries times in all.
	settled func(sh *shell.Shell) bool
}

// stateRunner runs the steps of a state on the UI thread.
type stateRunner struct {
	id   string
	st   shotState
	sh   *shell.Shell
	w    *mobileWire
	c    *ui.Context
	next int
	done bool
	// acted and tries follow the step with settled; seen is the frame of its
	// target at the last poll.
	acted bool
	tries int
	seen  geom.Rect
	tick  func()
	onEnd func()
}

// The demo video of the states of screen 06: lesson 3 of Set 35, 220 s long
// (`duration_sec`), with its audio on the device; the loop of the prototype's
// S.pm (index.html:280) runs from 01:12 to 01:40.
const (
	demoVideoSeconds = 220
	demoLessonID     = "l_3503"
	demoLessonNo     = 3
	demoLoopA        = 72
	demoLoopB        = 100
)

// demoDownloads is the prototype's demo state of the audio files of Set 35:
// env.DownloadManager for the shot states (M03, M05, M09, M11, M13, M14),
// empty for M12. Fill reads the course. What a state's tap changes (a
// download forgotten, a file deleted) changes the demo, in memory.
type demoDownloads struct {
	mu      sync.Mutex
	lessons map[string]int // lesson id to its number
	// empty is the state of a device with no file (M12).
	empty bool
	// items and jobs are the demo's files and downloads, made by fill.
	items, jobs []env.DownloadItem
	events      event.Topic[env.DownloadEvent]
	// mediaClock is the frozen clock of the fake engine of the playback
	// states (demoVideo), one per run. It sits here, in the one per-run
	// value of wireKnobs the steps reach (w.knobs.demo), until wireKnobs has a
	// field of its own for it.
	mediaClock *clock.Fake
}

var _ env.DownloadManager = (*demoDownloads)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// shotStates are the states by id. The pixel and layout states are those of
// local/shots/prototype-kit (M01 to M05B); the others are for a developer
// (and the flow tests) only, with no prototype shot. LOGIN and FORGOT drive
// the login form with taps on the window, as a finger does: a sign-in with
// the mock's credentials to Home, and the "Quên mật khẩu?" link to the
// system's browser (F7). M09 and M11 to M14 are P5's. The table is
// filled at init (baseShotStates) in every build but the shipped live
// one, which links none of them (devParts).
var shotStates = map[string]shotState{}

func init() { //nolint:gochecknoinits // the base states join the table the smoke files add to
	if !devParts {
		return
	}
	maps.Copy(shotStates, baseShotStates())
}

// baseShotStates returns the states of the shots and of the flow tests.
func baseShotStates() map[string]shotState {
	return map[string]shotState{
		// Screens 09 and 11 to 14 (P5): the prototype's downloads (the demo).
		"M09": {knobs: demo, prepare: signedIn(true), steps: []stateStep{
			homeLoaded(), openDetail(demoCourse), detailLoaded(), tapNamed("dlTap:set35-5"), sheetOpen(),
		}},
		"M11": {knobs: demo, prepare: signedIn(true), steps: []stateStep{homeLoaded(), goRoute(env.RouteDownloads, nil), routeIs(env.RouteDownloads)}},
		"M12": {knobs: demoEmpty, prepare: signedIn(true), steps: []stateStep{homeLoaded(), goRoute(env.RouteDownloads, nil), routeIs(env.RouteDownloads)}},
		"M13": {knobs: demo, prepare: offlineWithCache, steps: []stateStep{homeLoaded(), openDetail(demoCourse), routeIs(env.RouteOffline)}},
		"M14": {knobs: demo, prepare: signedIn(true), steps: []stateStep{
			homeLoaded(), goRoute(env.RouteDownloads, nil), routeIs(env.RouteDownloads), tapNamed("askDelete:set35-3"), sheetOpen(),
		}},
		"M01": {hold: true, steps: []stateStep{bootDone()}},
		"M02": {
			knobs: faceID, prepare: signedOutWithBio(""),
			steps: []stateStep{routeIs(env.RouteLogin)},
		},
		"M02B":       loginError("auth.login", "invalid_credentials"),
		"M02-locked": loginError("auth.login", "account_locked"),
		// K31: the 409 opens the sheet of devices over the banner; M02-device-pick has a device chosen
		"M02-device":      withSteps(loginError("auth.login", "device_limit"), sheetOpen()),
		"M02-device-pick": withSteps(loginError("auth.login", "device_limit"), sheetOpen(), tapNamed("devChoice:d_102")),
		"M02-offline":     loginError("", ""),
		"M02-expired":     {prepare: signedIn(false), steps: []stateStep{homeLoaded(), signOutByServer(), loginBanner()}},
		"M03B":            {prepare: signedInHolding("me.home"), steps: []stateStep{routeIs(env.RouteHome)}},
		"M03":             {knobs: demo, prepare: signedIn(true), steps: []stateStep{homeLoaded()}},
		"M03-offline":     {knobs: demo, prepare: offlineWithCache, steps: []stateStep{homeLoaded()}},
		"M04":             {knobs: demo, prepare: signedIn(false), steps: coursesSteps(func(courses) {})},
		"M04-special":     {knobs: demo, prepare: signedIn(false), steps: coursesSteps(func(k courses) { k.SetFilter(coursesSpecial) })},
		"M04-empty":       {knobs: demo, prepare: signedIn(false), steps: coursesSteps(func(k courses) { k.SetQuery("zzz") })},
		"M05":             {knobs: demo, prepare: signedIn(true), steps: []stateStep{homeLoaded(), openDetail(demoCourse), detailLoaded()}},
		"M05B":            {knobs: demo, prepare: signedIn(false), steps: []stateStep{homeLoaded(), openDetail("set-36"), detailLoaded()}},
		"UPGRADE":         {prepare: scenario("app.config", "upgrade_required"), steps: []stateStep{routeIs(env.RouteUpgrade)}},
		"LOGIN":           {prepare: signedOutFresh, steps: append(typeAndTapSignIn(), homeLoaded())},
		"FORGOT":          {prepare: signedOutFresh, steps: []stateStep{configBack(), tapNamed("toast")}},
		"M04-offline":     {knobs: demo, prepare: offlineWithCache, steps: []stateStep{homeLoaded(), goRoute(env.RouteCourses, nil)}},
		// Screens 06 to 10 (Q-P4-12): lesson 3 of Set 35 over the real player and a
		// fake engine whose media are 220 s long and show no picture, so that the
		// stage stays; the picture is masked in the comparison.
		"M06":         {knobs: demoVideo, prepare: signedIn(true), steps: videoSteps(false)},
		"M06-playing": {knobs: demoVideo, prepare: signedIn(true), steps: videoSteps(true)},
		"M06-hidden":  {knobs: demoVideo, prepare: signedIn(true), steps: append(videoSteps(true), controlsHidden())},
		"M07":         {knobs: demoVideo, prepare: signedIn(true), steps: append(videoSteps(false), tapNamed("sheet:options"), sheetOpen())},
		"M08":         {knobs: demoVideo, prepare: signedIn(true), steps: audioSteps()},
		"M10":         {knobs: demoVideo, prepare: signedIn(true), steps: doneSteps()},
	}
}

// newStateRunner returns the runner of state id over the shell and the
// services; onEnd runs once the last step ran.
func newStateRunner(id string, st shotState, sh *shell.Shell, w *mobileWire, onEnd func()) *stateRunner {
	r := &stateRunner{id: id, st: st, sh: sh, w: w, onEnd: onEnd}
	r.tick = r.step

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// lookupState returns the state of id; core.ErrNotFound names the ones
// there are.
func lookupState(id string) (shotState, error) {
	st, ok := shotStates[id]
	if !ok {
		return shotState{}, fmt.Errorf("app: state %q: one of %s: %w", id, strings.Join(stateIDs(), ", "), core.ErrNotFound)
	}

	return st, nil
}

// stateIDs returns the ids of the states, sorted.
func stateIDs() []string {
	ids := make([]string, 0, len(shotStates))
	for id := range shotStates {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return ids
}

// wireKnobsOf returns the knobs of st.
func wireKnobsOf(st shotState) wireKnobs {
	var k wireKnobs
	if st.knobs != nil {
		st.knobs(&k)
	}
	k.prepare = st.prepare

	return k
}

// start runs the steps from now, on the UI thread of c.
func (r *stateRunner) start(c *ui.Context) {
	r.c = c
	r.step()
}

// Done tells whether every step ran.
func (r *stateRunner) Done() bool { return r.done }

// Waiting names the step the runner waits for, "" once done.
func (r *stateRunner) Waiting() string {
	if r.done {
		return ""
	}

	return r.st.steps[r.next].what
}

// State is the download of a lesson of the demo course.
func (d *demoDownloads) State(lessonID string) (kit.LessonDownload, float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lessons[lessonID] == 0 {
		return kit.DlNone, 0
	}
	if i := demoIndex(d.items, lessonID); i >= 0 {
		return kit.DlDone, 1
	}
	if i := demoIndex(d.jobs, lessonID); i >= 0 {
		return kit.DlIng, d.jobs[i].Progress
	}

	return kit.DlNone, 0
}

// DoneCount is how many lessons have their audio.
func (d *demoDownloads) DoneCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.items)
}

// Items are the demo's files, in the prototype's order.
func (d *demoDownloads) Items() []env.DownloadItem {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.items)
}

// Jobs are the demo's downloads under way: lesson 7 at 64 %.
func (d *demoDownloads) Jobs() []env.DownloadItem {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.jobs)
}

// TotalBytes is the sum of the sizes of the files (48,4 MB).
func (d *demoDownloads) TotalBytes() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	var n int64
	for _, it := range d.items {
		n += it.Bytes
	}

	return n
}

// FreeSpace is the prototype's: 18,2 GB free, 71 % used.
func (*demoDownloads) FreeSpace() (free, total int64, ok bool) { return demoFree, demoTotal, true }

// RefreshSpace tells the same space again.
func (d *demoDownloads) RefreshSpace() {
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventSpace})
}

// Start adds the lessons not done or under way as downloads at 0 %, which
// never move.
func (d *demoDownloads) Start(c model.Course, lessons []model.Lesson) error {
	d.mu.Lock()
	for _, l := range lessons {
		if demoIndex(d.items, l.ID) >= 0 || demoIndex(d.jobs, l.ID) >= 0 || !l.HasAudio {
			continue
		}
		it := demoItem(c, l)
		it.State = kit.DlIng
		d.jobs = append(d.jobs, it)
	}
	d.mu.Unlock()
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})

	return nil
}

// Pause marks the downloads paused.
func (d *demoDownloads) Pause() { d.setPaused(true) }

// Resume marks them going on.
func (d *demoDownloads) Resume() { d.setPaused(false) }

// Forget drops a download of the demo.
func (d *demoDownloads) Forget(lessonID string) {
	d.mu.Lock()
	if i := demoIndex(d.jobs, lessonID); i >= 0 {
		d.jobs = slices.Delete(d.jobs, i, i+1)
	}
	d.mu.Unlock()
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})
}

// Delete drops a file of the demo.
func (d *demoDownloads) Delete(lessonID string) error {
	d.mu.Lock()
	if i := demoIndex(d.items, lessonID); i >= 0 {
		d.items = slices.Delete(d.items, i, i+1)
	}
	d.mu.Unlock()
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})

	return nil
}

// Events is the topic of the demo's changes.
func (d *demoDownloads) Events() *event.Topic[env.DownloadEvent] { return &d.events }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// step runs the steps whose condition holds, then looks again later.
func (r *stateRunner) step() {
	if r.w.noMock.Load() { // a scenario of the mock over a live API (apimode.go)
		r.w.deps.Logf("dfv: state %s stopped: it needs the mock in the process, not a live API", r.id)

		return
	}
	for r.next < len(r.st.steps) {
		s := r.st.steps[r.next]
		if s.when != nil && !s.when(r.sh) {
			r.c.AfterFunc(stepPoll, r.tick)

			return
		}
		if s.until != nil && !s.until(r.sh, r.w) {
			r.c.AfterFunc(stepPoll, r.tick)

			return
		}
		if s.target != nil && !r.acted && !r.laidOut(s) {
			r.c.AfterFunc(stepPoll, r.tick)

			return
		}
		if s.settled != nil && r.acted {
			if s.settled(r.sh) || r.tries >= tapTries {
				r.next++
				r.acted, r.tries = false, 0

				continue
			}
		}
		if s.act != nil {
			s.act(r.sh, r.w)
		}
		if s.settled != nil {
			r.acted = true
			r.tries++
			r.c.AfterFunc(stepPoll, r.tick)

			return
		}
		r.next++
	}
	if !r.done {
		r.done = true
		if r.onEnd != nil {
			r.onEnd()
		}
	}
}

// laidOut tells whether the target of s has a frame and kept it since the last
// poll.
func (r *stateRunner) laidOut(s stateStep) bool {
	wd := s.target(r.sh)
	if wd == nil {
		return false
	}
	f := wd.Node().WindowFrame()
	if f.W <= 0 || f.H <= 0 {
		return false
	}
	still := f == r.seen
	r.seen = f

	return still
}

// fill reads the demo course and makes the demo's files and downloads:
// lessons 1, 2, 3, 4 and 6 downloaded at their catalog sizes, 7 at 64 %
// of its (6,3 / 9,8 MB); none when empty.
func (d *demoDownloads) fill(ctx context.Context, w *mobileWire) error {
	c, err := w.client.Course(ctx, demoCourse)
	if err != nil {
		return fmt.Errorf("demo downloads: %w", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range c.Lessons {
		d.lessons[l.ID] = l.No
	}
	d.items, d.jobs = nil, nil
	if d.empty {
		return nil
	}
	for _, no := range demoDone {
		if i := slices.IndexFunc(c.Lessons, func(l model.Lesson) bool { return l.No == no }); i >= 0 {
			it := demoItem(c, c.Lessons[i])
			it.State, it.Done, it.Progress = kit.DlDone, it.Bytes, 1
			d.items = append(d.items, it)
		}
	}
	if i := slices.IndexFunc(c.Lessons, func(l model.Lesson) bool { return l.No == demoIng }); i >= 0 {
		it := demoItem(c, c.Lessons[i])
		it.State, it.Progress = kit.DlIng, demoIngShare
		it.Done = int64(float64(it.Bytes) * float64(demoIngShare))
		d.jobs = append(d.jobs, it)
	}

	return nil
}

// setPaused marks every download of the demo paused or not.
func (d *demoDownloads) setPaused(on bool) {
	d.mu.Lock()
	for i := range d.jobs {
		d.jobs[i].Paused = on
	}
	d.mu.Unlock()
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
}

// demoItem is the display data of lesson l of course c, its size the
// catalog's.
func demoItem(c model.Course, l model.Lesson) env.DownloadItem {
	n := c.LessonCount
	if n == 0 {
		n = len(c.Lessons)
	}

	return env.DownloadItem{
		LessonID: l.ID, CourseID: c.ID, CourseName: c.Name, CourseLessons: n,
		Title: l.Title, Tag: l.Tag, No: l.No, DurationSec: l.DurationSec, Bytes: l.AudioSizeBytes,
	}
}

// demoIndex is where the lesson is in list, -1 when it is not.
func demoIndex(list []env.DownloadItem, lessonID string) int {
	return slices.IndexFunc(list, func(it env.DownloadItem) bool { return it.LessonID == lessonID })
}

// playLesson is the step that starts lesson 3 of Set 35, as a video or as
// audio, on the real player over the fake engine.
func playLesson(what string, play func(sh *shell.Shell)) stateStep {
	return stateStep{what: what, act: func(sh *shell.Shell, _ *mobileWire) { play(sh) }}
}

// loaded waits for the lesson of the player to be loaded, moving the frozen clock
// of the fake engine by 1 ms for as long as the load is under way (the address
// is asked for off the UI thread, so the engine's load starts a poll or two
// after the play; the clock moves only then, so the position stays where the
// load put it, 01:12). It tells the video or the sound paused or playing.
func loaded() stateStep {
	return stateStep{what: "lesson loaded", until: func(sh *shell.Shell, w *mobileWire) bool {
		pb := sh.Env().Playback()
		if pb == nil {
			return false
		}
		switch pb.State() {
		case player.StateLoading:
			if k := w.knobs.demo; k != nil && k.mediaClock != nil {
				k.mediaClock.Advance(time.Millisecond) // the load ends: the media is at 01:12
			}

			return false
		case player.StatePlaying, player.StatePaused:
			return true
		default:
			return false
		}
	}}
}

// demoVideo is demo with the fake engine of the playback states: the media are
// 220 s long (`duration_sec` of lesson 3) and keep time with a frozen clock
// (demoDownloads.mediaClock) that the steps move, so the position stays where
// the load put it.
func demoVideo(k *wireKnobs) {
	demo(k)
	clk := clock.NewFake(demoNow)
	k.demo.mediaClock = clk
	k.media = func(cfg media.Config) (platformMedia, error) {
		cfg.Clock = clk

		return fakeMediaOf(fake.Options{VideoDuration: demoVideoSeconds, AudioDuration: demoVideoSeconds})(cfg)
	}
}

// openLesson3 opens the detail of Set 35 (so that the course is read, as a tap
// on its card does), then starts lesson 3 with play.
func openLesson3(what string, play func(sh *shell.Shell)) []stateStep {
	return []stateStep{homeLoaded(), openDetail(demoCourse), detailLoaded(), playLesson(what, play)}
}

// videoSteps opens the detail of Set 35 and plays lesson 3 as a video: the
// prototype's M06, paused at 01:12 (or playing, with the clock frozen there),
// speed .75, mirror and beats on, loop off, from 01:12 to 01:40, with no
// picture.
func videoSteps(playing bool) []stateStep {
	return append(openLesson3("play video 3", func(sh *shell.Shell) {
		sh.PlayVideo(demoCourse, demoLessonID, demoLessonNo)
	}),
		routeIs(env.RouteVideo), loaded(),
		stateStep{what: "set A, B", act: func(sh *shell.Shell, _ *mobileWire) {
			pl, _ := sh.Env().Playback().(*player.Player)
			if pl == nil {
				return
			}
			pl.SetAB(demoLoopA, demoLoopB) // K38: the exact ends of the prototype's state, which SetB's 30 s least would move
			if !playing {
				pl.Pause()
			}
			pl.Video().Reset() // no picture: the stage shows
		}})
}

// audioSteps plays lesson 3 as audio: the prototype's M08, 220 s long, at
// 01:12 (a seek, as the prototype's S.pos), paused, speed .75, beats on, loop
// off.
func audioSteps() []stateStep {
	return append(openLesson3("play audio 3", func(sh *shell.Shell) {
		sh.PlayAudio(demoCourse, demoLessonID, demoLessonNo)
	}),
		routeIs(env.RouteAudio), loaded(),
		stateStep{what: "seek and pause", act: func(sh *shell.Shell, _ *mobileWire) {
			pb := sh.Env().Playback()
			pb.Seek(demoLoopA) // a sound starts at 0 (the video's resume is Q-P4-13's): 01:12
			pb.Pause()
		}})
}

// doneSteps plays lesson 3 as a video to 01:12 and replaces screen 06 by screen
// 10 with its countdown held at 5: the prototype's M10 (two lessons watched,
// the card reads 2/10, "Còn 8 bài"). The lesson is not ended: the lesson would
// then count as watched.
func doneSteps() []stateStep {
	return append(openLesson3("play video 3", func(sh *shell.Shell) {
		sh.PlayVideo(demoCourse, demoLessonID, demoLessonNo)
	}),
		routeIs(env.RouteVideo), loaded(),
		stateStep{what: "done", act: func(sh *shell.Shell, _ *mobileWire) {
			sh.Nav().Replace(nav.Route{Screen: env.RouteDone})
		}},
		routeIs(env.RouteDone),
		stateStep{what: "hold the countdown", when: func(sh *shell.Shell) bool {
			d, _ := sh.Nav().Screen().(*screens.Done)

			return d != nil
		}, act: func(sh *shell.Shell, _ *mobileWire) {
			if d, _ := sh.Nav().Screen().(*screens.Done); d != nil {
				d.Hold()
			}
		}})
}

// controlsHidden waits for the controls of screen 06 to have faded away.
func controlsHidden() stateStep {
	return stateStep{what: "controls hidden", when: func(sh *shell.Shell) bool {
		v, ok := sh.Nav().Screen().(*screens.Video)

		return ok && !v.Shown() && v.Opacity() == 0
	}}
}

// sheetOpen waits for a sheet (options, download, delete) to be open and in
// place.
func sheetOpen() stateStep {
	return stateStep{what: "sheet open", when: func(sh *shell.Shell) bool {
		sheet := sh.Nav().Sheet()

		return sheet.IsOpen() && sheet.Content() != nil
	}}
}

// faceID makes the device offer Face ID (K13: the button shows).
func faceID(k *wireKnobs) { k.bio = sys.BiometryFace }

// demo pins the clock and shows the prototype's downloads.
func demo(k *wireKnobs) {
	k.now = demoNow
	k.demo = &demoDownloads{lessons: map[string]int{}}
}

// demoEmpty pins the clock and shows a device with no file (M12).
func demoEmpty(k *wireKnobs) {
	demo(k)
	k.demo.empty = true
}

// signOutFirst forgets a session a run before left in the vault: the states
// start from a device signed out (Logout with no session reads nothing of
// the network).
func signOutFirst(ctx context.Context, w *mobileWire) error {
	if err := w.mgr.Logout(ctx); err != nil {
		return fmt.Errorf("sign out: %w", err)
	}

	return nil
}

// seed signs in with the mock's session and remembers its email, as a
// sign-in with the password does.
func seed(ctx context.Context, w *mobileWire) error {
	s, err := w.mgr.Login(ctx, seedEmail, seedPassword)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	w.signedIn(s, s.User.Email)

	return nil
}

// signedIn is a device signed in (the mock's session in the vault); with
// downloads, the demo course's lessons are read for the demo downloads.
func signedIn(downloads bool) func(ctx context.Context, w *mobileWire) error {
	return func(ctx context.Context, w *mobileWire) error {
		if err := signOutFirst(ctx, w); err != nil {
			return err
		}
		if err := seed(ctx, w); err != nil {
			return err
		}
		if downloads && w.knobs.demo != nil {
			return w.knobs.demo.fill(ctx, w)
		}

		return nil
	}
}

// signedInHolding is signedIn with the requests of route held: the screen
// stays in its loading state.
func signedInHolding(route string) func(ctx context.Context, w *mobileWire) error {
	return func(ctx context.Context, w *mobileWire) error {
		if err := signedIn(false)(ctx, w); err != nil {
			return err
		}
		w.tr.hold(route)

		return nil
	}
}

// signedOutWithBio is a device whose user signed in with the password, then
// out: the email is remembered and the biometric copy kept (K13). The
// route's scenario, when given, is set after.
func signedOutWithBio(route string, name ...string) func(ctx context.Context, w *mobileWire) error {
	return func(ctx context.Context, w *mobileWire) error {
		if err := signOutFirst(ctx, w); err != nil {
			return err
		}
		if err := seed(ctx, w); err != nil {
			return err
		}
		if err := w.mgr.Logout(ctx); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
		if route != "" {
			return setScenario(w.mock, route, name[0])
		}

		return nil
	}
}

// offlineWithCache is a device signed in whose Home is in the cache, then
// the network gone.
func offlineWithCache(ctx context.Context, w *mobileWire) error {
	if err := signedIn(true)(ctx, w); err != nil {
		return err
	}
	if _, err := w.client.Home(ctx); err != nil {
		return fmt.Errorf("warm the cache: %w", err)
	}
	w.tr.setOffline(true)

	return nil
}

// scenario sets the mock's scenario of a route on a device signed out.
func scenario(route, name string) func(ctx context.Context, w *mobileWire) error {
	return func(ctx context.Context, w *mobileWire) error {
		if err := signOutFirst(ctx, w); err != nil {
			return err
		}

		return setScenario(w.mock, route, name)
	}
}

// loginError is M02 then a sign-in the mock refuses with the scenario
// (name of route), or, with no route, one the network does not carry: the
// banner shows.
func loginError(route, name string) shotState {
	prep := signedOutWithBio(route, name)
	if route == "" {
		prep = func(ctx context.Context, w *mobileWire) error {
			if err := signedOutWithBio("")(ctx, w); err != nil {
				return err
			}
			w.tr.setOffline(true)

			return nil
		}
	}

	return shotState{
		knobs: faceID, prepare: prep,
		steps: []stateStep{
			{what: "login", when: func(sh *shell.Shell) bool { return login(sh) != nil }, act: func(sh *shell.Shell, _ *mobileWire) {
				login(sh).Submit(seedPassword)
			}},
			loginBanner(),
		},
	}
}

// withSteps is the state with more steps at its end.
func withSteps(st shotState, more ...stateStep) shotState {
	st.steps = append(st.steps[:len(st.steps):len(st.steps)], more...)

	return st
}

// signedOutFresh is a device signed out that remembers no email.
func signedOutFresh(ctx context.Context, w *mobileWire) error {
	if err := signOutFirst(ctx, w); err != nil {
		return err
	}
	w.mu.Lock()
	w.last = ""
	w.mu.Unlock()
	if err := w.prefs.Remove(deviceKey); err != nil {
		return fmt.Errorf("forget the email: %w", err)
	}

	return nil
}

// typeAndTapSignIn fills the login form with the mock's credentials and taps
// "Đăng nhập".
func typeAndTapSignIn() []stateStep {
	return []stateStep{
		{what: "login", when: func(sh *shell.Shell) bool { return login(sh) != nil }, act: func(sh *shell.Shell, _ *mobileWire) {
			l := login(sh)
			l.Email().SetText(seedEmail)
			l.Password().SetText(seedPassword)
		}},
		{what: "tap sign in",
			target: func(sh *shell.Shell) ui.Widget {
				if l := login(sh); l != nil {
					return l.SubmitButton()
				}

				return nil
			},
			act: func(sh *shell.Shell, w *mobileWire) {
				if l := login(sh); l != nil {
					w.tap(l.SubmitButton())
				}
			},
			// the sign-in is under way (taps are ignored from then), or over
			settled: func(sh *shell.Shell) bool {
				l := login(sh)

				return l == nil || l.Busy()
			}},
	}
}

// configBack waits for the login with the app config the boot fetched (its
// links).
func configBack() stateStep {
	return stateStep{what: "login with config", when: func(sh *shell.Shell) bool {
		_, ok := sh.Env().Cfg()

		return ok && login(sh) != nil
	}}
}

// tapNamed taps the widget named name on show.
func tapNamed(name string) stateStep {
	return stateStep{what: "tap " + name,
		target: func(sh *shell.Shell) ui.Widget { return findNamed(sh, name) },
		act: func(sh *shell.Shell, w *mobileWire) {
			if wd := findNamed(sh, name); wd != nil {
				w.tap(wd)
			}
		}}
}

// findNamed returns the first visible widget named name under root, nil when
// there is none.
func findNamed(root ui.Widget, name string) ui.Widget {
	b := root.Node()
	if b.Hidden {
		return nil
	}
	if b.Name == name {
		return root
	}
	for ch := range b.Children() {
		if w := findNamed(ch, name); w != nil {
			return w
		}
	}

	return nil
}

// bootDone waits for the session read and the app config's answer.
func bootDone() stateStep {
	return stateStep{what: "boot", when: func(sh *shell.Shell) bool {
		b := sh.BootState()

		return b.Restored && b.ConfigDone
	}}
}

// routeIs waits for the route.
func routeIs(route string) stateStep {
	return stateStep{what: "route " + route, when: func(sh *shell.Shell) bool { return sh.Nav().Current().Screen == route }}
}

// homeLoaded waits for Home with its data.
func homeLoaded() stateStep {
	return stateStep{what: "home loaded", when: func(sh *shell.Shell) bool {
		h, ok := sh.Nav().Screen().(*screens.Home)

		return ok && h.Content() != nil
	}}
}

// goRoute opens a route on an empty stack, as a tab does.
func goRoute(route string, params any) stateStep {
	return stateStep{what: "route " + route, act: func(sh *shell.Shell, _ *mobileWire) {
		sh.Go(nav.Route{Screen: route, Params: params}, true)
	}}
}

// coursesSpecial is the index of the "Đặc biệt" filter of the Courses screen
// (S.m.filter='Đặc biệt').
const coursesSpecial = 2

// courses is what a state asks of the content of the Courses screen.
type courses interface {
	SetFilter(i int)
	SetQuery(s string)
	Rows() int
}

// coursesSteps opens Courses (as the tab does), waits for its list and then
// applies set (the prototype's S.m.filter / S.m.q).
func coursesSteps(set func(k courses)) []stateStep {
	onShow := func(sh *shell.Shell) courses {
		c, _ := sh.Nav().Screen().(*screens.Courses)
		if c == nil || c.Content().Rows() == 0 {
			return nil
		}

		return c.Content()
	}

	return []stateStep{
		homeLoaded(), goRoute(env.RouteCourses, nil),
		{what: "courses list", when: func(sh *shell.Shell) bool { return onShow(sh) != nil },
			act: func(sh *shell.Shell, _ *mobileWire) { set(onShow(sh)) }},
	}
}

// openDetail opens the course's detail, as a tap on its card does.
func openDetail(courseID string) stateStep {
	return stateStep{what: "detail " + courseID, act: func(sh *shell.Shell, _ *mobileWire) { sh.Detail(courseID) }}
}

// detailLoaded waits for the detail with its course: the column of a course
// the user is in (M05), the locked course's of one they are not (M05B).
func detailLoaded() stateStep {
	return stateStep{what: "detail loaded", when: func(sh *shell.Shell) bool {
		d, ok := sh.Nav().Screen().(*screens.Detail)

		return ok && (d.Content() != nil || d.Locked() != nil)
	}}
}

// signOutByServer ends the session as the server refusing it does.
func signOutByServer() stateStep {
	return stateStep{what: "sign out", act: func(_ *shell.Shell, w *mobileWire) { w.mgr.SignOut(nil) }}
}

// loginBanner waits for the login with its banner and no sign-in under way.
func loginBanner() stateStep {
	return stateStep{what: "login banner", when: func(sh *shell.Shell) bool {
		l := login(sh)
		if l == nil || l.Busy() {
			return false
		}
		_, _, shown := l.Banner()

		return shown
	}}
}

// login returns the login screen on show, nil when another is.
func login(sh *shell.Shell) *screens.Login {
	l, _ := sh.Nav().Screen().(*screens.Login)

	return l
}
