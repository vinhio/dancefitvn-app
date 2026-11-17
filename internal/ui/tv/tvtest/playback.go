//go:build tvos

package tvtest

import (
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Playback is a fake env.Playback: plain state a test sets, the calls
// recorded, no engine. The screens of P7.4 drive it with the numbers of the
// prototype's states (72 of 220 seconds, paused).
type Playback struct {
	LessonV               player.Lesson
	KindV                 player.Kind
	StateV                player.State
	ErrV                  error
	PlayingV              bool
	PosV, DurV, RateV     float64
	MirrorV, LoopV, BeatV bool
	AV, BV                float64
	BeatIdx               int
	Prev_, Next_          bool
	Picture               bool

	// Calls counts the calls by name ("PlayVideo", "Pause", ...); Placed is the
	// last rectangle of Place; OnPlace, when set, is told of each Place (the
	// plane of AttachPlane).
	Calls   map[string]int
	Placed  geom.Rect
	OnPlace func(r geom.Rect)

	changes event.Topic[player.Change]
}

var _ env.Playback = (*Playback)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Start makes the playback playing, as a screen starting a lesson does.
func (p *Playback) Start() { p.PlayingV = true }

// call records the call of name.
func (p *Playback) call(name string) {
	if p.Calls == nil {
		p.Calls = map[string]int{}
	}
	p.Calls[name]++
}

// PlayVideo records the call and starts.
func (p *Playback) PlayVideo(l player.Lesson, _ []player.Lesson) {
	p.call("PlayVideo")
	p.LessonV, p.KindV, p.PlayingV = l, player.KindVideo, true
}

// PlayAudio records the call and starts at at.
func (p *Playback) PlayAudio(l player.Lesson, _ []player.Lesson, at float64) {
	p.call("PlayAudio")
	p.LessonV, p.KindV, p.PlayingV, p.PosV = l, player.KindAudio, true, at
}

// Play records the call and plays.
func (p *Playback) Play() { p.call("Play"); p.PlayingV = true }

// Pause records the call and pauses.
func (p *Playback) Pause() { p.call("Pause"); p.PlayingV = false }

// Toggle records the call and flips; on a failed load it plays again, the load
// made again (Player.Play), so the state is Loading.
func (p *Playback) Toggle() {
	p.call("Toggle")
	p.PlayingV = !p.PlayingV
	if p.StateV == player.StateError && p.PlayingV {
		p.StateV, p.ErrV = player.StateLoading, nil
	}
}

// SetRate records the call and sets the rate.
func (p *Playback) SetRate(r float64) { p.call("SetRate"); p.RateV = r }

// SetMirror records the call and sets the option.
func (p *Playback) SetMirror(on bool) { p.call("SetMirror"); p.MirrorV = on }

// SetLoop records the call and sets the option.
func (p *Playback) SetLoop(on bool) { p.call("SetLoop"); p.LoopV = on }

// SetBeat records the call and sets the option.
func (p *Playback) SetBeat(on bool) { p.call("SetBeat"); p.BeatV = on }

// Prev records the call.
func (p *Playback) Prev() { p.call("Prev") }

// Next records the call.
func (p *Playback) Next() { p.call("Next") }

// Leave records the call.
func (p *Playback) Leave() { p.call("Leave") }

// Lesson returns the lesson set.
func (p *Playback) Lesson() player.Lesson { return p.LessonV }

// Kind returns the kind set.
func (p *Playback) Kind() player.Kind { return p.KindV }

// State returns the state set.
func (p *Playback) State() player.State { return p.StateV }

// Err returns the error set.
func (p *Playback) Err() error { return p.ErrV }

// Playing returns whether the playback is playing.
func (p *Playback) Playing() bool { return p.PlayingV }

// Pos returns the position set.
func (p *Playback) Pos() float64 { return p.PosV }

// Dur returns the duration set.
func (p *Playback) Dur() float64 { return p.DurV }

// Rate returns the rate set, 1 when unset.
func (p *Playback) Rate() float64 {
	if p.RateV == 0 {
		return 1
	}

	return p.RateV
}

// Mirror returns the option set.
func (p *Playback) Mirror() bool { return p.MirrorV }

// Loop returns the option set.
func (p *Playback) Loop() bool { return p.LoopV }

// Beat returns the option set.
func (p *Playback) Beat() bool { return p.BeatV }

// A returns the loop's start set.
func (p *Playback) A() float64 { return p.AV }

// B returns the loop's end set.
func (p *Playback) B() float64 { return p.BV }

// BeatIndex returns the lit cell set.
func (p *Playback) BeatIndex() int { return p.BeatIdx }

// HasPrev returns whether a lesson comes before.
func (p *Playback) HasPrev() bool { return p.Prev_ }

// HasNext returns whether a lesson comes after.
func (p *Playback) HasNext() bool { return p.Next_ }

// Changes returns the topic a test publishes on.
func (p *Playback) Changes() *event.Topic[player.Change] { return &p.changes }

// Place records the rectangle and tells OnPlace.
func (p *Playback) Place(r geom.Rect) {
	p.Placed = r
	if p.OnPlace != nil {
		p.OnPlace(r)
	}
}

// Publish tells the listeners of Changes about a change of kind k, as the player
// does on the UI thread.
func (p *Playback) Publish(k player.ChangeKind) { p.changes.Publish(player.Change{Kind: k}) }

// Fail makes the load fail with err as Player.fail does: nothing is wanted, the
// state is Error, and the listeners hear the ChangeState and then the ChangeError.
func (p *Playback) Fail(err error) {
	p.PlayingV, p.StateV, p.ErrV = false, player.StateError, err
	p.Publish(player.ChangeState)
	p.changes.Publish(player.Change{Kind: player.ChangeError, Err: err})
}

// Tick moves the position to pos and tells the listeners, as the 250 ms tick of
// the player does.
func (p *Playback) Tick(pos float64) {
	p.PosV = pos
	p.Publish(player.ChangeTime)
}

// End makes the playback end, as a video played to its end: the position at the
// length, not playing, and ChangeEnded.
func (p *Playback) End() {
	p.PosV, p.PlayingV, p.StateV = p.DurV, false, player.StateEnded
	p.Publish(player.ChangeState)
	p.Publish(player.ChangeEnded)
}

// PictureReady returns whether the first frame is on the plane.
func (p *Playback) PictureReady() bool { return p.Picture }
