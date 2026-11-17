//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/port.go (the Player interface:
// what the screens drive, a Changes topic they listen to) and
// jplayer-mobile/internal/ui/np/port at 49de3c2. The model is internal/player
// (docs/plan/04 section 4); the interface only lets the screens' tests give them
// a fake (shelltest.FakePlayback).

package env

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SheetOptions is the id of the options sheet (SH.options, screen 07): the
// prototype's `S.m.sheet === 'options'`, the `data-v` of the buttons that open
// it. Screen 06 opens it itself on Env.Nav.Sheet(); the id is what the
// layout.json of M07 and the tests call it.
const SheetOptions = "options"

// Playback is the player as the screens drive it: the subset of
// *player.Player they use, which satisfies it. Every method is for the UI
// thread. The screens never build a player.Lesson themselves: the shell does,
// from the model.Course (PlayerLessons), when it handles PlayVideo and
// PlayAudio.
//
// A screen reads the state when it is built and again on each Change of
// Changes (ChangeTime at every tick of the engine, 250 ms: update the widgets
// only when the value changed, and allocate nothing).
type Playback interface {
	// PlayVideo and PlayAudio start the lesson of a course (03 section 3). The
	// Shell calls them, not the screens.
	PlayVideo(l player.Lesson, list []player.Lesson)
	PlayAudio(l player.Lesson, list []player.Lesson)
	// Play, Pause and Toggle are the play button (`togglePlay`).
	Play()
	Pause()
	Toggle()
	// Seek moves to sec in [0, Dur] (a tap on the scrub bar); SeekBy by d seconds
	// in [0, Dur-1] (`seek:-10`, `seek:10`).
	Seek(sec float64)
	SeekBy(d float64)
	// SetRate sets the speed (a segment of the options sheet); CycleRate is
	// `cycleSpeed` of screen 08: .75, 1, 1.25, .75.
	SetRate(r float64)
	CycleRate()
	// SetMirror, SetLoop and SetBeat are `mirror`, `loop` and `beat`; SetLoop(true)
	// seeks to A.
	SetMirror(on bool)
	SetLoop(on bool)
	SetBeat(on bool)
	// Prev and Next are `prevLes` and `nextLes`: the lesson before or after, at
	// 0, A and B kept. AutoAdvance is the end-of-lesson countdown reaching 0: it
	// plays the next lesson when the preference "Tự phát bài tiếp theo" is on
	// and the course has one, and reports whether it did.
	Prev()
	Next()
	AutoAdvance() bool
	// Leave is screen 06 being left: a video pauses, its position is saved.
	Leave()

	// Lesson is the lesson loaded (the zero player.Lesson, empty ID, before
	// any), Kind whether it is video or audio, State where the load stands, Err
	// why it is player.StateError.
	Lesson() player.Lesson
	Kind() player.Kind
	State() player.State
	Err() error
	// Playing says the media should be playing (the play button shows pause).
	Playing() bool
	// Pos and Dur are the position and length in seconds; Dur is the API's
	// until the engine tells its own. Rate is the speed.
	Pos() float64
	Dur() float64
	Rate() float64
	// Mirror, Loop and Beat are the options of the sheet; A and B the loop's
	// ends in seconds.
	Mirror() bool
	Loop() bool
	Beat() bool
	A() float64
	B() float64
	// BeatIndex is the lit cell of the beat bar: floor(pos*2) mod 8 (K16).
	BeatIndex() int
	// HasPrev and HasNext tell whether the course has a lesson before and after
	// the one loaded.
	HasPrev() bool
	HasNext() bool
	// Changes tells what changed, on the UI thread.
	Changes() *event.Topic[player.Change]
	// Video is the surface the engine draws the picture of the video on.
	Video() *media.Surface
}

// Prefs are the preferences the player screens read: the app's PrefsStore
// adapted (store.Prefs). The defaults of a fresh app are the prototype's:
// auto next on, quality auto.
type Prefs interface {
	// AutoNext is "Tự phát bài tiếp theo" (`S.prefs.auto`).
	AutoNext() bool
	// Quality is "Chất lượng video" (`S.prefs.quality`).
	Quality() store.Quality
	// CycleQuality is `quality` (index.html:610): auto, 1080p, 720p, 480p, auto.
	CycleQuality()
	// QualitySupported tells whether the quality has an effect on this
	// phone: the iPhone's engine caps the HLS variant, Android's MediaPlayer
	// cannot pick one (K21, Q-P4-3); screen 07 leaves the row out where it
	// has none.
	QualitySupported() bool
}

var _ Playback = (*player.Player)(nil)

// memPrefs is the Prefs of a Services that has none: the defaults, kept in
// memory.
type memPrefs struct {
	q       store.Quality
	autoOff bool // "Tự phát bài tiếp theo" turned off
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Playback returns the player, nil when the services have none.
func (e *Env) Playback() Playback {
	return e.Services.Playback()
}

// Prefs returns the preferences: the services' when they have them, else the
// defaults kept in memory.
func (e *Env) Prefs() Prefs {
	if p := e.Services.Prefs(); p != nil {
		return p
	}
	if e.prefs == nil {
		e.prefs = &memPrefs{q: store.QualityAuto}
	}

	return e.prefs
}

// AutoNext is on until SetAutoNext turns it off: the prototype's default.
func (m *memPrefs) AutoNext() bool { return !m.autoOff }

// SetAutoNext is `pref:auto` (AutoNextPrefs).
func (m *memPrefs) SetAutoNext(on bool) { m.autoOff = !on }

// Quality is the quality chosen, auto at first.
func (m *memPrefs) Quality() store.Quality { return m.q }

// CycleQuality is `quality`.
func (m *memPrefs) CycleQuality() { m.q = m.q.NextMobile() }

// QualitySupported is true: with no engine to ask, the prototype's row shows.
func (m *memPrefs) QualitySupported() bool { return true }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// PlayerLessons returns the lessons of c as the player wants them (the list
// Prev, Next and the end-of-lesson screen walk): the number, the length the
// API gives, the resume position and whether the lesson is watched, with the
// names of the course on each (the Now Playing's artist, 06's course line,
// 10's card).
func PlayerLessons(c model.Course) []player.Lesson {
	out := make([]player.Lesson, len(c.Lessons))
	count := c.LessonCount
	if count == 0 {
		count = len(c.Lessons)
	}
	for i, l := range c.Lessons {
		out[i] = player.Lesson{
			CourseID: c.ID, CourseName: c.Name, CourseShort: c.Short(), Tagline: c.Tagline,
			ID: l.ID, Title: l.Title, Tag: l.Tag, CoverURL: c.CoverURL,
			No: l.No, Count: count, DurationSec: l.DurationSec,
			ResumeSec: l.Progress.PositionSec, Watched: l.Progress.Watched, NoAudio: !l.HasAudio,
		}
	}

	return out
}

// LessonIndex returns where lesson id is in the list PlayerLessons made, -1
// when the course has no such lesson.
func LessonIndex(list []player.Lesson, id string) int {
	for i := range list {
		if list[i].ID == id {
			return i
		}
	}

	return -1
}
