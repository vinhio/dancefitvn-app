// Adapted from jplayer-mobile/internal/player/player.go at 49de3c2.

package player

import (
	"context"
	"dancefitvn-app/internal/media"
	"math"
	"slices"
	"sync"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Kind tells which media of a lesson is played.
type Kind uint8

// The kinds of media.
const (
	KindUnknown Kind = iota
	KindVideo
	KindAudio
)

// State is what the player is doing.
type State uint8

// The states of the player.
const (
	StateUnknown State = iota
	// StateIdle is nothing loaded.
	StateIdle
	// StateLoading is the address being asked for, or the engine loading.
	StateLoading
	StatePlaying
	StatePaused
	// StateEnded is the media played to its end.
	StateEnded
	StateError
)

// ChangeKind tells what a Change is about.
type ChangeKind uint8

// The kinds of Change.
const (
	ChangeUnknown ChangeKind = iota
	// ChangeLesson is another lesson or kind loaded.
	ChangeLesson
	// ChangeState is a new State.
	ChangeState
	// ChangeTime is a new position; it is the 250 ms tick and allocates nothing.
	ChangeTime
	// ChangeDuration is a new length (the engine's, once known).
	ChangeDuration
	// ChangeRate is a new speed.
	ChangeRate
	// ChangeOptions is a new Mirror, Loop, Beat, A or B.
	ChangeOptions
	// ChangeSeek is a jump of the position; it follows the ChangeTime of the jump.
	ChangeSeek
	// ChangeEnded is the media played to its end: the shell shows the end of lesson.
	ChangeEnded
	// ChangeError is a failure; Err tells which.
	ChangeError
)

// RepeatMode is what the player does at the end of what it plays (K38): nothing
// (the lesson ends and the next may follow), the same lesson again, or the A–B
// loop. The audio screen's button goes through the three in turn; the options
// sheet's "Lặp đoạn A–B" switch is between RepeatAB and RepeatOff.
type RepeatMode uint8

// The modes. RepeatUnknown, the zero, is read as RepeatOff.
const (
	RepeatUnknown RepeatMode = iota
	// RepeatOff plays to the end, which ends the lesson.
	RepeatOff
	// RepeatOne starts the lesson again at 0 when it ends: no end, no next lesson.
	RepeatOne
	// RepeatAB plays from A to B and goes back to A (Loop).
	RepeatAB
)

// Change is what the player tells its listeners.
type Change struct {
	Kind ChangeKind
	// Err is set by ChangeError.
	Err error
}

// Lesson is what the player needs to know of a lesson; the shell builds it
// from model.Course and model.Lesson.
type Lesson struct {
	CourseID, CourseName, CourseShort, Tagline string
	ID, Title, Tag, CoverURL                   string
	// No is the number in the course, Count the lessons of the course.
	No, Count int
	// DurationSec is the length the API gives; the engine's replaces it once known.
	DurationSec int
	// ResumeSec is progress.position_sec of the course data.
	ResumeSec int
	Watched   bool
	// NoAudio is a lesson the course data give no audio file (has_audio false):
	// the screens offer no download for it.
	NoAudio bool
}

// Resolver finds the address to play; the app implements it over api.Client
// and download.Manager. Both block, and are called off the UI thread with a
// context that ends when the lesson loaded changes.
type Resolver interface {
	// VideoURL is GET /lessons/{id}/stream, .hls_url.
	VideoURL(ctx context.Context, lessonID string) (string, error)
	// AudioURL is the file of a downloaded lesson, else GET /lessons/{id}/audio, .url.
	AudioURL(ctx context.Context, lessonID string) (string, error)
}

// Saver keeps the position; *progress.Tracker implements it.
type Saver interface {
	Update(lessonID, courseID string, posSec, durSec float64, playing bool)
	Pause()
	Leave()
}

// SeekNoter is a Saver that is told of the seeks the user makes (the bar, the
// ±10 s buttons, a screen reader's step), before the position they move to is
// reported: the *progress.Tracker does not take a jump into the last tenth of
// the lesson for having watched it (K38). A Saver that is not one hears only the
// positions.
type SeekNoter interface {
	Seeked(lessonID string, posSec, durSec float64)
}

// Options configures New.
type Options struct {
	// Engine plays; required.
	Engine media.Engine
	// Session, when set, is activated before each load and listened to.
	Session media.Session
	// Resolver finds the addresses; required.
	Resolver Resolver
	// Saver, when set, is told of the position.
	Saver Saver
	// AutoNext tells the preference "Tự phát bài tiếp theo"; nil is on.
	AutoNext func() bool
	// Dispatch runs fn on the UI thread; required.
	Dispatch func(func())
	// Go runs work on another goroutine with a context that ends when the
	// app shuts down; nil starts a goroutine that Close waits for.
	Go func(work func(ctx context.Context))
	// Logf receives diagnostics; nil drops them. Never pass secrets to it:
	// the player logs lesson ids and error kinds only.
	Logf func(format string, args ...any)
}

// Player plays one lesson at a time over an engine. Every method runs on the
// UI thread, as do the callbacks it takes.
type Player struct {
	o      Options
	reader media.TimeReader

	ctx    context.Context //nolint:containedctx // lifetime of the goroutines of the player
	cancel context.CancelFunc
	wg     sync.WaitGroup

	changes event.Topic[Change]
	unsubs  [2]func()
	closed  bool

	lesson Lesson
	list   []Lesson
	kind   Kind
	state  State
	err    error
	// want says the media should play: set by Play, cleared by Pause and by
	// the engine telling it paused, ended or failed.
	want     bool
	pos, dur float64

	// The options of the prototype's S.pm; they stay across lessons.
	rate         float64
	mirror, loop bool
	// repeatOne is the mode Repeat One: the media starts again at its end. It is
	// never on with loop (the A-B loop): SetRepeat keeps one of them.
	repeatOne bool
	beat      bool
	a, b      float64

	// Load bookkeeping.
	gen     uint64 // bumped by every load; an answer of an older one is dropped
	stopGen context.CancelFunc
	token   uint64
	loaded  bool // the engine has the load of token
	url     string
	// start is the position the load begins at; checkStart asks to look at it
	// once the engine's length is known (the resume position of the course
	// data may not fit it).
	start      float64
	checkStart bool
	retried    bool
	// interrupted: an interruption paused the media that played.
	interrupted bool
	background  bool
	// detached: Leave told the Saver to forget the lesson; ticks of the
	// paused video do not bring it back.
	detached bool

	eventFn   func(media.Event)
	sessionFn func(media.SessionEvent)
}

// Speeds of the prototype.
var (
	// SheetRates are the speeds of the options sheet (M07).
	SheetRates = [...]float64{0.5, 0.75, 1, 1.25, 1.5}
	// CycleRates are the speeds the button of screen 08 goes through ($P:609).
	CycleRates = [...]float64{0.75, 1, 1.25}
)

// Numbers of the prototype ($P:595-596) and of 03 §5.
const (
	// InitialRate is the speed of a fresh app: 1x (K41; the prototype's S.pm of
	// $P:280 was 0.75).
	InitialRate = 1.0
	// WatchedRatio is the share of a lesson that makes it watched.
	WatchedRatio = 0.9
	// SkipSeconds is the step of the ‹10 and 10› buttons.
	SkipSeconds = 10
	// LoopLength is how far B is from A when the loop turns on, and the least it
	// can be (K38; the prototype's A = round(dur · .33) and B = A + 28, $P:595,
	// is replaced): B = min(A + 30, dur), and B moves in steps of this many seconds.
	LoopLength = 30
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns a Player with the options of a fresh app: speed 1x, mirror
// off, beat off (K41), loop off, A and B of the first lesson (Q-P4-1). Close ends it.
func New(o Options) *Player {
	p := &Player{o: o, state: StateIdle, rate: InitialRate}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.reader, _ = o.Engine.(media.TimeReader)
	p.eventFn = p.onEvent
	p.sessionFn = p.onSession
	p.unsubs[0] = o.Engine.Events().Subscribe(p.eventFn)
	if o.Session != nil {
		p.unsubs[1] = o.Session.Events().Subscribe(p.sessionFn)
	}

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Changes is the topic the player publishes to, on the UI thread.
func (p *Player) Changes() *event.Topic[Change] { return &p.changes }

// Video is the surface of the picture of the video.
func (p *Player) Video() *media.Surface { return p.o.Engine.Video() }

// Lesson returns the lesson loaded.
func (p *Player) Lesson() Lesson { return p.lesson }

// Kind returns the media loaded.
func (p *Player) Kind() Kind { return p.kind }

// State returns what the player is doing.
func (p *Player) State() State { return p.state }

// Err returns the failure of StateError, nil otherwise.
func (p *Player) Err() error { return p.err }

// Playing reports whether the media plays or is about to.
func (p *Player) Playing() bool { return p.want }

// Pos returns the position in seconds, as the last tick told it.
func (p *Player) Pos() float64 { return p.pos }

// SyncPos reads the true position from the engine and keeps it as Pos. In
// the background the engine stops its tick while the media plays on, so Pos
// falls behind; call it before showing the position (Now Playing). It tells
// no one and allocates nothing.
func (p *Player) SyncPos() { p.syncTime() }

// Interrupted reports whether an interruption (a call) paused the media that
// played and has not ended; a Play or a Pause of the user ends it.
func (p *Player) Interrupted() bool { return p.interrupted }

// Dur returns the length in seconds: the API's until the engine tells its own.
func (p *Player) Dur() float64 { return p.dur }

// Rate returns the speed.
func (p *Player) Rate() float64 { return p.rate }

// Mirror returns whether the picture is flipped.
func (p *Player) Mirror() bool { return p.mirror }

// Loop returns whether A–B repeats.
func (p *Player) Loop() bool { return p.loop }

// Repeat returns the mode: RepeatAB while the A–B loop is on, RepeatOne while the
// lesson repeats, else RepeatOff.
func (p *Player) Repeat() RepeatMode {
	switch {
	case p.loop:
		return RepeatAB
	case p.repeatOne:
		return RepeatOne
	default:
		return RepeatOff
	}
}

// Beat returns whether the 8-beat bar shows.
func (p *Player) Beat() bool { return p.beat }

// A returns the start of the A–B loop in seconds.
func (p *Player) A() float64 { return p.a }

// B returns the end of the A–B loop in seconds.
func (p *Player) B() float64 { return p.b }

// BeatIndex is the beat of the 8-beat bar at 120 BPM: floor(pos · 2) mod 8
// (K16). The position is the media's, so a speed changes how fast the beats
// pass and never their place.
func (p *Player) BeatIndex() int {
	i := int(math.Floor(p.pos*2)) % 8
	if i < 0 {
		i += 8
	}

	return i
}

// HasPrev reports whether the lesson has one before it in the list.
func (p *Player) HasPrev() bool { return p.index() > 0 }

// HasNext reports whether the lesson has one after it in the list.
func (p *Player) HasNext() bool {
	i := p.index()

	return i >= 0 && i+1 < len(p.list)
}

// Close stops the engine and the goroutines. It is idempotent.
func (p *Player) Close() {
	if p.closed {
		return
	}
	p.saveLeave()
	p.closed = true
	p.gen++
	p.cancel()
	for _, u := range p.unsubs {
		if u != nil {
			u()
		}
	}
	_ = p.o.Engine.Stop()
	p.wg.Wait()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// index is the place of the lesson in the list, -1 when it is not there.
func (p *Player) index() int {
	return slices.IndexFunc(p.list, func(l Lesson) bool { return l.ID == p.lesson.ID })
}

// emit tells the listeners.
func (p *Player) emit(k ChangeKind) { p.changes.Publish(Change{Kind: k}) }

// setState changes the state and tells it.
func (p *Player) setState(s State) {
	if p.state == s {
		return
	}
	p.state = s
	p.emit(ChangeState)
}

// logf writes a diagnostic when the app gave a Logf.
func (p *Player) logf(format string, args ...any) {
	if p.o.Logf != nil {
		p.o.Logf(format, args...)
	}
}

// syncTime reads the true position from the engine: in the background no
// tick told it (F1 of M9).
func (p *Player) syncTime() {
	if p.reader == nil || !p.loaded || p.atEnd() {
		return
	}
	if t, ok := p.reader.ReadTime(p.token); ok {
		p.pos = t
	}
}

// atEnd reports whether the media ended and nobody asked to play again: the
// position is the length then, and the engine's own reading (11.97 of 12) or
// a late tick must never take it back, or the last save would be a lower one.
func (p *Player) atEnd() bool { return p.state == StateEnded && !p.want }

// saveUpdate tells the Saver the position and whether it plays.
func (p *Player) saveUpdate() {
	if p.o.Saver == nil || p.detached || p.lesson.ID == "" {
		return
	}
	p.o.Saver.Update(p.lesson.ID, p.lesson.CourseID, p.pos, p.dur, p.state == StatePlaying)
}

// savePause saves the position now.
func (p *Player) savePause() {
	if p.o.Saver == nil || p.detached || p.lesson.ID == "" {
		return
	}
	p.o.Saver.Pause()
}

// saveLeave saves the true position and makes the Saver forget the lesson.
func (p *Player) saveLeave() {
	if p.o.Saver == nil || p.lesson.ID == "" {
		return
	}
	if !p.detached {
		p.syncTime()
		p.o.Saver.Update(p.lesson.ID, p.lesson.CourseID, p.pos, p.dur, false)
	}
	p.o.Saver.Leave()
}
