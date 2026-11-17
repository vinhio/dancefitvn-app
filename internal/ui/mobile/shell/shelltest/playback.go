//go:build !tvos

package shelltest

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// FakePlayback is env.Playback without an engine: it keeps the state the
// player's rules of 03 section 3 give (start position, A and B, clamps, the
// speed cycle, prev/next) and writes down every command, so a screen's test
// checks what the screen asked and the Shell's what it did, with the state the
// screen shows. Commands that load (PlayVideo, Next...) put it straight in the
// playing state; nothing moves on by itself: a test moves the clock with Tick
// and the end with End. The Changes are published as the player does, on the
// caller's thread. For the real player over media/fake use Fake.SetPlayback.
type FakePlayback struct {
	mu    sync.Mutex
	calls []string

	changes event.Topic[player.Change]
	video   media.Surface
	prefs   env.Prefs

	lesson   player.Lesson
	list     []player.Lesson
	kind     player.Kind
	state    player.State
	err      error
	want     bool
	pos, dur float64
	rate     float64
	mirror   bool
	loop     bool
	beat     bool
	a, b     float64
}

// FakePrefs is env.Prefs in memory, with the knob of the auto-next preference.
type FakePrefs struct {
	mu        sync.Mutex
	autoNext  bool
	quality   store.Quality
	cycles    int
	noQuality bool
	wifiOff   bool
}

var (
	_ env.Playback  = (*FakePlayback)(nil)
	_ env.Prefs     = (*FakePrefs)(nil)
	_ env.WiFiPrefs = (*FakePrefs)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewFakePlayback returns a player in the state of a fresh app (player.New): idle,
// speed 0.75, mirror on, beat on, loop off. prefs says whether AutoAdvance
// goes on; nil is on.
func NewFakePlayback(prefs env.Prefs) *FakePlayback {
	return &FakePlayback{state: player.StateIdle, rate: player.InitialRate, mirror: true, beat: true, prefs: prefs}
}

// NewFakePrefs returns the preferences of a first launch: auto next on, quality auto.
func NewFakePrefs() *FakePrefs {
	return &FakePrefs{autoNext: true, quality: store.QualityAuto}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Calls returns the commands received, in order, as "Name" or "Name:arg":
// PlayVideo:<lesson id>, PlayAudio:<id>, Play, Pause, Toggle, Seek:<sec>,
// SeekBy:<sec>, SetRate:<r>, CycleRate, SetMirror:<on>, SetLoop:<on>,
// SetBeat:<on>, Prev, Next, AutoAdvance, Leave.
func (f *FakePlayback) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.calls...)
}

// Count returns how many commands were received that are, or begin with, name
// followed by ":".
func (f *FakePlayback) Count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == name || strings.HasPrefix(c, name+":") {
			n++
		}
	}

	return n
}

// ClearCalls forgets the commands received.
func (f *FakePlayback) ClearCalls() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

// Set puts the fake in a state without a command and without a change: the
// lesson (its list is the one lesson), the kind, the position and the length.
func (f *FakePlayback) Set(l player.Lesson, k player.Kind, st player.State, pos, dur float64) {
	f.lesson, f.kind, f.state, f.pos, f.dur = l, k, st, pos, dur
	f.want = st == player.StatePlaying || st == player.StateLoading
	if f.list == nil {
		f.list = []player.Lesson{l}
	}
	if f.a == 0 && f.b == 0 {
		f.a, f.b = defaultAB(dur)
	}
}

// SetList sets the lessons Prev and Next walk.
func (f *FakePlayback) SetList(list []player.Lesson) { f.list = append([]player.Lesson(nil), list...) }

// SetAB sets the loop's ends.
func (f *FakePlayback) SetAB(a, b float64) { f.a, f.b = a, b }

// Tick moves the position to pos and publishes ChangeTime, as the engine's tick
// does.
func (f *FakePlayback) Tick(pos float64) {
	f.pos = pos
	f.emit(player.ChangeTime)
}

// End is the media playing to its end: the state Ended, the position at the
// length, ChangeState and ChangeEnded.
func (f *FakePlayback) End() {
	f.pos, f.want, f.state = f.dur, false, player.StateEnded
	f.emit(player.ChangeState)
	f.emit(player.ChangeEnded)
}

// Fail is the load failing with err, as Player.fail tells it: the state Error
// (ChangeState, when it was another one) and then ChangeError.
func (f *FakePlayback) Fail(err error) {
	f.want, f.err = false, err
	f.setState(player.StateError)
	f.changes.Publish(player.Change{Kind: player.ChangeError, Err: err})
}

// SetState puts the state and publishes ChangeState.
func (f *FakePlayback) SetState(s player.State) {
	f.state = s
	f.want = s == player.StatePlaying || s == player.StateLoading
	f.emit(player.ChangeState)
}

// PlayVideo starts the lesson as player.PlayVideo does: another lesson at its
// resume position (0 at or over 90 %, or watched), A = round(dur*.33), B = A+28;
// the same lesson goes on where it is.
func (f *FakePlayback) PlayVideo(l player.Lesson, list []player.Lesson) {
	f.note("PlayVideo:" + l.ID)
	f.start(l, list, player.KindVideo)
}

// PlayAudio starts the sound of the lesson as player.PlayAudio does: another
// lesson at 0, A and B left alone.
func (f *FakePlayback) PlayAudio(l player.Lesson, list []player.Lesson) {
	f.note("PlayAudio:" + l.ID)
	f.start(l, list, player.KindAudio)
}

// Play plays.
func (f *FakePlayback) Play() {
	f.note("Play")
	f.play()
}

// play is Play and Toggle's start.
func (f *FakePlayback) play() {
	f.want = true
	if f.state == player.StateError { // a failed load is made again (Player.Play: reload)
		f.err = nil
		f.setState(player.StateLoading)

		return
	}
	if f.state == player.StateEnded {
		f.pos = 0
	}
	f.setState(player.StatePlaying)
}

// Pause pauses.
func (f *FakePlayback) Pause() {
	f.note("Pause")
	f.want = false
	f.setState(player.StatePaused)
}

// Toggle is the play button.
func (f *FakePlayback) Toggle() {
	f.note("Toggle")
	if f.want {
		f.want = false
		f.setState(player.StatePaused)

		return
	}
	f.play()
}

// Seek moves to sec, kept in [0, Dur].
func (f *FakePlayback) Seek(sec float64) {
	f.notef("Seek", sec)
	f.pos = max(0, min(sec, f.dur))
	f.emit(player.ChangeSeek)
}

// SeekBy moves by d, kept in [0, Dur-1].
func (f *FakePlayback) SeekBy(d float64) {
	f.notef("SeekBy", d)
	f.pos = max(0, min(f.dur-1, f.pos+d))
	f.emit(player.ChangeSeek)
}

// SetRate sets the speed.
func (f *FakePlayback) SetRate(r float64) {
	f.notef("SetRate", r)
	f.rate = r
	f.emit(player.ChangeRate)
}

// CycleRate goes .75, 1, 1.25, .75; any other speed goes to .75.
func (f *FakePlayback) CycleRate() {
	f.note("CycleRate")
	next := player.CycleRates[0]
	for i, r := range player.CycleRates {
		if r == f.rate {
			next = player.CycleRates[(i+1)%len(player.CycleRates)]
		}
	}
	f.rate = next
	f.emit(player.ChangeRate)
}

// SetMirror sets the mirror.
func (f *FakePlayback) SetMirror(on bool) {
	f.noteb("SetMirror", on)
	f.mirror = on
	f.emit(player.ChangeOptions)
}

// SetLoop sets the loop; on it seeks A.
func (f *FakePlayback) SetLoop(on bool) {
	f.noteb("SetLoop", on)
	f.loop = on
	if on {
		f.pos = f.a
	}
	f.emit(player.ChangeOptions)
}

// SetBeat sets the beat bar.
func (f *FakePlayback) SetBeat(on bool) {
	f.noteb("SetBeat", on)
	f.beat = on
	f.emit(player.ChangeOptions)
}

// Prev plays the lesson before, at 0, as it was playing or paused.
func (f *FakePlayback) Prev() {
	f.note("Prev")
	f.step(-1, f.want)
}

// Next plays the lesson after, at 0, as it was playing or paused.
func (f *FakePlayback) Next() {
	f.note("Next")
	f.step(1, f.want)
}

// AutoAdvance plays the next lesson when the preference is on and there is one.
func (f *FakePlayback) AutoAdvance() bool {
	f.note("AutoAdvance")
	if f.prefs != nil && !f.prefs.AutoNext() {
		return false
	}

	return f.step(1, true)
}

// Leave pauses a video.
func (f *FakePlayback) Leave() {
	f.note("Leave")
	if f.kind == player.KindVideo {
		f.want = false
		if f.state == player.StatePlaying || f.state == player.StateLoading {
			f.setState(player.StatePaused)
		}
	}
}

// Lesson is the lesson loaded.
func (f *FakePlayback) Lesson() player.Lesson { return f.lesson }

// Kind is video or audio.
func (f *FakePlayback) Kind() player.Kind { return f.kind }

// State is the state of the load.
func (f *FakePlayback) State() player.State { return f.state }

// Err is the error of the state Error.
func (f *FakePlayback) Err() error { return f.err }

// Playing says the media should play.
func (f *FakePlayback) Playing() bool { return f.want }

// Pos is the position.
func (f *FakePlayback) Pos() float64 { return f.pos }

// Dur is the length.
func (f *FakePlayback) Dur() float64 { return f.dur }

// Rate is the speed.
func (f *FakePlayback) Rate() float64 { return f.rate }

// Mirror is the mirror option.
func (f *FakePlayback) Mirror() bool { return f.mirror }

// Loop is the A-B loop option.
func (f *FakePlayback) Loop() bool { return f.loop }

// Beat is the beat bar option.
func (f *FakePlayback) Beat() bool { return f.beat }

// A is the loop's start.
func (f *FakePlayback) A() float64 { return f.a }

// B is the loop's end.
func (f *FakePlayback) B() float64 { return f.b }

// BeatIndex is floor(pos*2) mod 8.
func (f *FakePlayback) BeatIndex() int { return int(math.Floor(f.pos*2)) & 7 }

// HasPrev tells whether there is a lesson before the one loaded.
func (f *FakePlayback) HasPrev() bool { return f.index() > 0 }

// HasNext tells whether there is a lesson after the one loaded.
func (f *FakePlayback) HasNext() bool {
	i := f.index()

	return i >= 0 && i+1 < len(f.list)
}

// Changes is the topic the fake publishes to.
func (f *FakePlayback) Changes() *event.Topic[player.Change] { return &f.changes }

// Video is an empty surface: no frame is ever published.
func (f *FakePlayback) Video() *media.Surface { return &f.video }

// AutoNext is the preference of the fake.
func (p *FakePrefs) AutoNext() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.autoNext
}

// SetAutoNext turns the preference on or off.
func (p *FakePrefs) SetAutoNext(on bool) {
	p.mu.Lock()
	p.autoNext = on
	p.mu.Unlock()
}

// Quality is the quality chosen.
func (p *FakePrefs) Quality() store.Quality {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.quality
}

// CycleQuality goes to the next quality of the phone's cycle.
func (p *FakePrefs) CycleQuality() {
	p.mu.Lock()
	p.quality = p.quality.NextMobile()
	p.cycles++
	p.mu.Unlock()
}

// QualitySupported is true unless SetQualitySupported(false) (an Android
// phone, K21).
func (p *FakePrefs) QualitySupported() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return !p.noQuality
}

// SetQualitySupported says whether the quality has an effect.
func (p *FakePrefs) SetQualitySupported(on bool) {
	p.mu.Lock()
	p.noQuality = !on
	p.mu.Unlock()
}

// WiFiOnly is "Chỉ tải qua Wi‑Fi": on at first.
func (p *FakePrefs) WiFiOnly() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return !p.wifiOff
}

// SetWiFiOnly sets "Chỉ tải qua Wi‑Fi".
func (p *FakePrefs) SetWiFiOnly(on bool) {
	p.mu.Lock()
	p.wifiOff = !on
	p.mu.Unlock()
}

// Cycles returns how many times CycleQuality was called.
func (p *FakePrefs) Cycles() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.cycles
}

// Playback is the player of the Fake: a FakePlayback until SetPlayback.
func (f *Fake) Playback() env.Playback {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.pb
}

// WatchedLocal tells whether SetWatchedLocal named the lesson.
func (f *Fake) WatchedLocal(lessonID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.watched[lessonID]
}

// SetWatchedLocal marks the lessons as watched in this run.
func (f *Fake) SetWatchedLocal(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watched == nil {
		f.watched = map[string]bool{}
	}
	for _, id := range ids {
		f.watched[id] = true
	}
}

// Prefs are the preferences of the Fake.
func (f *Fake) Prefs() env.Prefs { return f.prefs }

// FakePlayback returns the Fake's own FakePlayback, nil when SetPlayback put
// another player in its place.
func (f *Fake) FakePlayback() *FakePlayback {
	f.mu.Lock()
	defer f.mu.Unlock()
	pb, _ := f.pb.(*FakePlayback)

	return pb
}

// FakePrefs returns the Fake's preferences to turn their knobs.
func (f *Fake) FakePrefs() *FakePrefs { return f.prefs }

// SetPlayback puts pb in place of the FakePlayback (the real *player.Player
// over media/fake, for a flow test); do it before the Shell is mounted.
func (f *Fake) SetPlayback(pb env.Playback) {
	f.mu.Lock()
	f.pb = pb
	f.mu.Unlock()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// note writes a command down; notef and noteb add a number or a bool.
func (f *FakePlayback) note(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
}

func (f *FakePlayback) notef(name string, v float64) {
	f.note(name + ":" + trimFloat(v))
}

func (f *FakePlayback) noteb(name string, v bool) {
	if v {
		f.note(name + ":true")

		return
	}
	f.note(name + ":false")
}

// emit publishes a change of kind k.
func (f *FakePlayback) emit(k player.ChangeKind) { f.changes.Publish(player.Change{Kind: k}) }

// setState changes the state and tells it.
func (f *FakePlayback) setState(s player.State) {
	if f.state != s {
		f.state = s
		f.emit(player.ChangeState)
	}
}

// index is the place of the lesson loaded in the list, -1 when it is not there.
func (f *FakePlayback) index() int {
	return env.LessonIndex(f.list, f.lesson.ID)
}

// start is PlayVideo and PlayAudio.
func (f *FakePlayback) start(l player.Lesson, list []player.Lesson, k player.Kind) {
	f.list = append([]player.Lesson(nil), list...)
	if l.ID == f.lesson.ID && k == f.kind && f.state != player.StateIdle && f.state != player.StateError {
		f.lesson = l
		f.want = true
		f.setState(player.StatePlaying)

		return
	}
	f.lesson, f.kind, f.err = l, k, nil
	f.dur = float64(l.DurationSec)
	f.pos = 0
	if k == player.KindVideo {
		f.pos = resumeOf(l)
		f.a, f.b = defaultAB(f.dur)
	} else if f.a == 0 && f.b == 0 {
		f.a, f.b = defaultAB(f.dur)
	}
	f.want = true
	f.state = player.StatePlaying
	f.emit(player.ChangeLesson)
	f.emit(player.ChangeState)
}

// step is Prev, Next and AutoAdvance.
func (f *FakePlayback) step(delta int, play bool) bool {
	i := f.index()
	j := i + delta
	if i < 0 || j < 0 || j >= len(f.list) {
		return false
	}
	f.lesson = f.list[j]
	f.dur, f.pos, f.err = float64(f.lesson.DurationSec), 0, nil
	f.want = play
	if play {
		f.state = player.StatePlaying
	} else {
		f.state = player.StatePaused
	}
	f.emit(player.ChangeLesson)
	f.emit(player.ChangeState)

	return true
}

// trimFloat writes v as short as it can be read back.
func trimFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// defaultAB is A = round(dur*.33), B = A + 28 (index.html:595).
func defaultAB(dur float64) (a, b float64) {
	a = math.Round(dur * .33)

	return a, a + 28
}

// resumeOf is the position a video of l starts at.
func resumeOf(l player.Lesson) float64 {
	r := float64(l.ResumeSec)
	if l.Watched || r <= 0 || (l.DurationSec > 0 && r >= player.WatchedRatio*float64(l.DurationSec)) {
		return 0
	}

	return r
}
