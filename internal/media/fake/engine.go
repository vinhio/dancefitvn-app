// Adapted from jplayer-mobile/internal/media/fake/engine.go at 49de3c2.

package fake

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Default lengths and tick (07 §3.1, 04 §2.1).
const (
	DefaultAudioDuration = 40.0 // tone.mp3
	DefaultVideoDuration = 20.0 // clip.mp4
	DefaultTick          = 250 * time.Millisecond
)

// The size of the pictures a fake video draws.
const (
	FrameWidth  = 160
	FrameHeight = 90
)

// ErrNotLoaded is what a call that needs a loaded media returns without one.
var ErrNotLoaded = fmt.Errorf("fake: nothing loaded: %w", core.ErrInvalid)

// Op names a call the engine received.
type Op string

// The calls an Engine records.
const (
	OpLoad   Op = "load"
	OpPlay   Op = "play"
	OpPause  Op = "pause"
	OpStop   Op = "stop"
	OpSeek   Op = "seek"
	OpVolume Op = "volume"
	OpMuted  Op = "muted"
	OpRate   Op = "rate"
	OpClose  Op = "close"
	// OpSetBackground is SetBackground; Flag tells whether the app went to
	// the background.
	OpSetBackground Op = "background"
)

// The log of calls: its first capacity, and the most it keeps.
const (
	callsStart = 64
	maxCalls   = 4096
)

// Call is one call the engine received.
type Call struct {
	Op Op
	// Src and Load are set by OpLoad.
	Src  media.Source
	Load media.LoadOptions
	// Value is the argument of OpSeek, OpVolume, OpRate; Flag that of OpMuted
	// and OpSetBackground.
	Value float64
	Flag  bool
}

// Options configures an Engine.
type Options struct {
	// AudioDuration and VideoDuration are the lengths of the media without
	// an entry in Durations; 0 takes 40 and 20 seconds.
	AudioDuration, VideoDuration float64
	// Durations gives the length by URL.
	Durations map[string]float64
	// Fail makes the load of a URL fail with the error.
	Fail map[string]error
	// LoadDelay is how long a load takes; 0 is the next call of the clock.
	LoadDelay time.Duration
	// Tick is the period of the time events; 0 is 250 ms.
	Tick time.Duration
	// StartAfterDuration orders the end of a load as avf and avftv do:
	// EventDuration is published once ready, and only then does the load
	// go to its start (LoadOptions.Start, or a Seek made before ready),
	// kept within the length; a Seek made in answer to EventDuration
	// replaces that start. Without it the position is the start from Load
	// on, unclamped.
	StartAfterDuration bool
}

// Engine is a media.Engine for tests: it plays nothing, but keeps time with
// the clock it is given, so a test moves it with clock.Fake.Advance. It runs
// on the goroutine that drives the clock, which is the UI thread of a test.
// It is a media.Backgrounder and a media.TimeReader too. As the systems'
// engines do, it plays on in the background without telling the time: the
// position moves with the clock, read by ReadTime and Position, and the
// media still ends then (one timer at its end, as the system's
// notification of the end; no tick); a video's picture is not decoded
// there (VideoOff) and its images are freed, made again with the first
// frame back. An audio load frees them too, as avf does.
type Engine struct {
	cfg  media.Config
	opts Options

	events event.Topic[media.Event]
	video  media.Surface

	token   uint64
	src     media.Source
	loaded  bool // Load was called and Stop was not
	ready   bool // the load finished
	autoRun bool // play as soon as the load finishes
	// start is where the load goes once its length is told
	// (StartAfterDuration); starting is that length being told.
	start    float64
	starting bool
	state    media.PlayState
	pos      float64
	dur      float64
	vol      float64
	muted    bool
	rate     float64
	frame    uint8
	back     bool // the app is in the background: no tick, no frames
	// backAt is when pos was last brought up to date in the background,
	// while the media plays.
	backAt time.Time

	loadTimer, tickTimer, endTimer clock.Timer
	readyFn, tickFn, endFn         func()

	calls []Call

	spec    *enginekit.Spectrum // made at the first Frequencies
	samples [enginekit.SpectrumSize]float32
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns an Engine. cfg.Clock is required.
func New(cfg media.Config, o Options) *Engine {
	if o.AudioDuration == 0 {
		o.AudioDuration = DefaultAudioDuration
	}
	if o.VideoDuration == 0 {
		o.VideoDuration = DefaultVideoDuration
	}
	if o.Tick <= 0 {
		o.Tick = DefaultTick
	}
	e := &Engine{cfg: cfg, opts: o, vol: 1, rate: 1, calls: make([]Call, 0, callsStart)}
	e.readyFn = e.onReady
	e.tickFn = e.onTick
	e.endFn = e.onBackEnd

	return e
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// UnreachablePrefix starts the URL of a file that Load cannot reach: it
// fails with media.ErrUnreachable, as a linked file out of reach does.
const UnreachablePrefix = "unreachable://"

// Load starts a new load: the events of the last one are stale from now on.
func (e *Engine) Load(_ context.Context, src media.Source, opts media.LoadOptions) (uint64, error) {
	e.record(Call{Op: OpLoad, Src: src, Load: opts})
	if strings.HasPrefix(src.URL, UnreachablePrefix) {
		e.stopTimers()
		e.loaded = false
		e.state = media.StateError

		return 0, media.ErrUnreachable
	}
	e.stopTimers()
	e.video.SetActive(false)
	e.video.Reset()
	e.token++
	if e.spec != nil {
		e.spec.Reset()
	}
	if src.AudioOnly {
		e.video.Free()
	}
	e.src, e.loaded, e.ready, e.autoRun = src, true, false, opts.Auto
	e.state, e.pos = media.StateLoading, max(0, opts.Start)
	e.start, e.starting = e.pos, false
	if e.opts.StartAfterDuration {
		e.pos = 0
	}
	e.dur = e.durationOf(src)
	e.loadTimer = e.cfg.Clock.After(e.opts.LoadDelay, e.readyFn)

	return e.token, nil
}

// Play plays; before the load finishes it plays once it does, and at the end
// it plays from the start.
func (e *Engine) Play() error {
	e.record(Call{Op: OpPlay})
	if !e.loaded {
		return ErrNotLoaded
	}
	if !e.ready {
		e.autoRun = true

		return nil
	}
	if e.state == media.StateError {
		return media.ErrDecode
	}
	if e.state == media.StateEnded || (e.dur > 0 && e.pos >= e.dur) {
		e.pos = 0
	}
	e.run()

	return nil
}

// Pause pauses, keeping the position.
func (e *Engine) Pause() error {
	e.record(Call{Op: OpPause})
	if !e.loaded {
		return ErrNotLoaded
	}
	if !e.ready {
		e.autoRun = false

		return nil
	}
	if e.state == media.StatePlaying || e.state == media.StateLoading {
		e.catchUp()
		e.stopPlayTimers()
		e.video.SetActive(false)
		e.state = media.StatePaused
		e.publish(media.Event{Kind: media.EventState, State: media.StatePaused})
	}

	return nil
}

// Stop unloads: nothing plays and the events of the load are stale.
func (e *Engine) Stop() error {
	e.record(Call{Op: OpStop})
	e.stopTimers()
	e.video.SetActive(false)
	e.video.Free()
	e.token++
	e.loaded, e.ready, e.autoRun, e.pos, e.dur = false, false, false, 0, 0
	e.state = media.StateUnknown

	return nil
}

// Seek moves to seconds, kept between 0 and the length (a length of 0 is
// unknown and sets no limit), and tells the time.
func (e *Engine) Seek(seconds float64) error {
	e.record(Call{Op: OpSeek, Value: seconds})
	if !e.loaded {
		return ErrNotLoaded
	}
	if e.src.Live { // the live window is the system's; the fake has none to move in
		return nil
	}
	t := max(seconds, 0)
	if e.dur > 0 { // a length of 0 is unknown: nothing to clamp to
		t = min(t, e.dur)
	}
	if e.opts.StartAfterDuration && (!e.ready || e.starting) {
		e.start = t // where onReady goes, as avf's Seek before ready

		return nil
	}
	e.pos = t
	e.followBack()
	if e.ready {
		e.publish(media.Event{Kind: media.EventTime, Time: e.pos})
	}

	return nil
}

// SetVolume sets the volume.
func (e *Engine) SetVolume(v float64) error {
	e.record(Call{Op: OpVolume, Value: v})
	e.vol = v

	return nil
}

// SetMuted mutes or unmutes.
func (e *Engine) SetMuted(on bool) error {
	e.record(Call{Op: OpMuted, Flag: on})
	e.muted = on

	return nil
}

// SetRate sets the playback speed.
func (e *Engine) SetRate(r float64) error {
	e.record(Call{Op: OpRate, Value: r})
	e.catchUp()
	e.rate = r
	e.followBack()

	return nil
}

// SetBackground follows the app's lifecycle (media.Backgrounder): in the
// background the time and the frames stop, the picture is not decoded
// (VideoOff) and the video's images are freed; back in the foreground the
// engine tells the time and draws a frame at once, into images made again,
// and goes on ticking if it plays.
func (e *Engine) SetBackground(on bool) {
	e.record(Call{Op: OpSetBackground, Flag: on})
	if e.back == on {
		return
	}
	if on {
		e.back = true
		e.cfg.Clock.Stop(e.tickTimer)
		e.tickTimer = 0
		e.followBack()
		e.video.Free()

		return
	}
	e.catchUp()
	e.back = false
	e.cfg.Clock.Stop(e.endTimer)
	e.endTimer = 0
	if !e.loaded || !e.ready {
		return
	}
	e.publish(media.Event{Kind: media.EventTime, Time: e.pos})
	if !e.src.AudioOnly {
		e.draw()
	}
	if e.state == media.StatePlaying {
		e.tickTimer = e.cfg.Clock.After(e.opts.Tick, e.tickFn)
	}
}

// Backgrounded tells that the app is in the background.
func (e *Engine) Backgrounded() bool { return e.back }

// VideoOff tells that the picture of the video loaded is not decoded: the
// app is in the background, where the systems' engines decode the sound
// only (avf disables the video tracks, amp takes the player's Surface
// away), and the sound plays on.
func (e *Engine) VideoOff() bool { return e.back && e.loaded && !e.src.AudioOnly }

// ReadTime returns the position of the load token (media.TimeReader):
// false for another load, one that is not ready, or none.
func (e *Engine) ReadTime(token uint64) (float64, bool) {
	if !e.loaded || !e.ready || token != e.token {
		return 0, false
	}
	e.catchUp()

	return e.pos, true
}

// ToneRate is the sample rate of the sound Frequencies hears.
const ToneRate = 44100

// ToneHz are the two sines the fake plays to its analyser, those of
// tone.mp3 (make-testmedia.sh:8).
var ToneHz = [2]float64{330, 495}

// Frequencies is the fake's media.Analyser: while a media loaded with
// Source.Analyse plays, it hears the sines of ToneHz, as of the engine's
// position, at the amplitude .4 each, and silence while it loads or does
// not play (as avf's tap before its first buffer); the first call makes
// the tables (the only allocation). It returns false for any other media
// and a load that failed.
func (e *Engine) Frequencies(dst *[media.FreqBins]uint8, dt time.Duration) bool {
	if !e.loaded || !e.src.Analyse || e.state == media.StateError {
		return false
	}
	if e.spec == nil {
		e.spec = enginekit.NewSpectrum()
	}
	if e.state != media.StatePlaying {
		e.spec.Bytes(nil, dt, dst)

		return true
	}
	e.catchUp()
	for i := range e.samples {
		t := e.pos - float64(len(e.samples)-1-i)/ToneRate
		var v float64
		for _, hz := range ToneHz {
			v += 0.4 * math.Sin(2*math.Pi*hz*t)
		}
		e.samples[i] = float32(v)
	}
	e.spec.Bytes(&e.samples, dt, dst)

	return true
}

// Events returns the topic the engine publishes to.
func (e *Engine) Events() *event.Topic[media.Event] { return &e.events }

// Video returns the surface a video's frames are drawn to.
func (e *Engine) Video() *media.Surface { return &e.video }

// Close releases the engine.
func (e *Engine) Close() error {
	e.record(Call{Op: OpClose})
	e.stopTimers()
	e.video.SetActive(false)
	e.video.Free()
	e.loaded, e.ready = false, false

	return nil
}

// Calls returns the calls received so far, oldest first (the last 4096).
func (e *Engine) Calls() []Call { return append([]Call(nil), e.calls...) }

// ClearCalls forgets the calls recorded.
func (e *Engine) ClearCalls() { e.calls = e.calls[:0] }

// Volume returns the volume last set.
func (e *Engine) Volume() float64 { return e.vol }

// Muted returns the mute last set.
func (e *Engine) Muted() bool { return e.muted }

// Rate returns the speed last set.
func (e *Engine) Rate() float64 { return e.rate }

// Position returns the playing position in seconds.
func (e *Engine) Position() float64 {
	e.catchUp()

	return e.pos
}

// Duration returns the length of the media loaded.
func (e *Engine) Duration() float64 { return e.dur }

// Token returns the token of the last load.
func (e *Engine) Token() uint64 { return e.token }

// State returns what the engine is doing.
func (e *Engine) State() media.PlayState { return e.state }

// Finish jumps to the end: the last time, then Ended.
func (e *Engine) Finish() {
	if !e.loaded || !e.ready {
		return
	}
	e.pos = e.dur
	e.end()
}

// Fail reports a failure while playing: the error event, and no more time.
func (e *Engine) Fail(err error) {
	if !e.loaded {
		return
	}
	e.catchUp()
	e.stopPlayTimers()
	e.video.SetActive(false)
	e.state = media.StateError
	e.publish(media.Event{Kind: media.EventError, Err: err})
}

// Stall reports that the media waits for data and stops the time.
func (e *Engine) Stall() {
	if !e.loaded || !e.ready {
		return
	}
	e.catchUp()
	e.stopPlayTimers()
	e.video.SetActive(false)
	e.state = media.StateLoading
	e.publish(media.Event{Kind: media.EventState, State: media.StateLoading})
}

// Unstall reports that the data came and the media plays again.
func (e *Engine) Unstall() {
	if e.loaded && e.ready && e.state == media.StateLoading {
		e.run()
	}
}

// Buffer reports how far the media is loaded.
func (e *Engine) Buffer(seconds float64) {
	e.publish(media.Event{Kind: media.EventBuffered, Buffered: seconds})
}

// SetDuration reports a new length, as when it was unknown at first.
func (e *Engine) SetDuration(seconds float64) {
	e.catchUp()
	e.dur = seconds
	e.followBack()
	e.publish(media.Event{Kind: media.EventDuration, Duration: seconds})
}

// Emit publishes ev as it is, a stale token included.
func (e *Engine) Emit(ev media.Event) { e.events.Publish(ev) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// durationOf is the length of the media of src.
func (e *Engine) durationOf(src media.Source) float64 {
	if src.Live { // a live stream has no end
		return 0
	}
	if d, ok := e.opts.Durations[src.URL]; ok {
		return d
	}
	if src.AudioOnly {
		return e.opts.AudioDuration
	}

	return e.opts.VideoDuration
}

// record keeps a call, the last maxCalls of them.
func (e *Engine) record(c Call) {
	if len(e.calls) == maxCalls {
		e.calls = append(e.calls[:0], e.calls[maxCalls/2:]...)
	}
	e.calls = append(e.calls, c)
}

// publish tags ev with the token of the load and publishes it.
func (e *Engine) publish(ev media.Event) {
	ev.Token = e.token
	e.events.Publish(ev)
}

// stopTimers cancels the load, the tick and the end in the background.
func (e *Engine) stopTimers() {
	e.cfg.Clock.Stop(e.loadTimer)
	e.loadTimer = 0
	e.stopPlayTimers()
}

// stopPlayTimers cancels the tick and the end in the background.
func (e *Engine) stopPlayTimers() {
	e.cfg.Clock.Stop(e.tickTimer)
	e.cfg.Clock.Stop(e.endTimer)
	e.tickTimer, e.endTimer = 0, 0
}

// catchUp moves pos on by the time played in the background since backAt,
// at the speed, up to the end. In the foreground the tick moves it.
func (e *Engine) catchUp() {
	if !e.back || e.state != media.StatePlaying {
		return
	}
	now := e.cfg.Clock.Now()
	e.pos += now.Sub(e.backAt).Seconds() * e.rate
	e.backAt = now
	if e.dur > 0 {
		e.pos = min(e.pos, e.dur)
	}
}

// followBack starts the time played in the background from now, and
// arranges its end, if the media plays there: as the system tells the
// end, no tick. Call it after pos, the state, the speed or the length
// changed.
func (e *Engine) followBack() {
	if !e.back {
		return
	}
	e.backAt = e.cfg.Clock.Now()
	e.cfg.Clock.Stop(e.endTimer)
	e.endTimer = 0
	if e.state != media.StatePlaying || e.dur <= 0 || e.rate <= 0 {
		return
	}
	left := time.Duration((e.dur - e.pos) / e.rate * float64(time.Second))
	e.endTimer = e.cfg.Clock.After(max(left, 0), e.endFn)
}

// onBackEnd is the end of the media played in the background.
func (e *Engine) onBackEnd() {
	e.endTimer = 0
	e.catchUp()
	e.pos = e.dur
	e.end()
}

// onReady finishes a load: the failure, or the length and the first state.
// With StartAfterDuration it goes to the start after telling the length,
// as avf does; a Load or Stop made in answer to the length ends it.
func (e *Engine) onReady() {
	e.loadTimer = 0
	if err, ok := e.opts.Fail[e.src.URL]; ok {
		e.state, e.dur = media.StateError, 0
		e.publish(media.Event{Kind: media.EventError, Err: err})

		return
	}
	e.ready = true
	token := e.token
	e.starting = true
	e.publish(media.Event{Kind: media.EventDuration, Duration: e.dur})
	e.starting = false
	if e.token != token || !e.ready {
		return
	}
	if e.opts.StartAfterDuration {
		e.pos = e.start
		if e.dur > 0 {
			e.pos = min(e.pos, e.dur)
		}
	}
	if !e.src.AudioOnly {
		e.publish(media.Event{Kind: media.EventVideoSize, Width: FrameWidth, Height: FrameHeight})
		e.draw()
	}
	e.publish(media.Event{Kind: media.EventTime, Time: e.pos})
	if e.autoRun {
		e.run()

		return
	}
	e.state = media.StatePaused
	e.publish(media.Event{Kind: media.EventState, State: media.StatePaused})
}

// run starts the tick and tells that the media plays.
func (e *Engine) run() {
	e.cfg.Clock.Stop(e.tickTimer)
	e.tickTimer = 0
	e.state = media.StatePlaying
	e.followBack()
	e.publish(media.Event{Kind: media.EventState, State: media.StatePlaying})
	e.video.SetActive(!e.src.AudioOnly)
	if !e.back {
		e.tickTimer = e.cfg.Clock.After(e.opts.Tick, e.tickFn)
	}
}

// onTick advances the position by one period at the playback speed.
func (e *Engine) onTick() {
	e.tickTimer = 0
	e.pos += e.opts.Tick.Seconds() * e.rate
	if e.dur > 0 && e.pos >= e.dur {
		e.pos = e.dur
		e.end()

		return
	}
	e.publish(media.Event{Kind: media.EventTime, Time: e.pos})
	if !e.src.AudioOnly {
		e.draw()
	}
	e.tickTimer = e.cfg.Clock.After(e.opts.Tick, e.tickFn)
}

// end tells the last time and that the media ended.
func (e *Engine) end() {
	e.stopPlayTimers()
	e.video.SetActive(false)
	e.state = media.StateEnded
	e.publish(media.Event{Kind: media.EventTime, Time: e.pos})
	e.publish(media.Event{Kind: media.EventEnded})
}

// draw fills the next frame with a flat colour that changes at every frame,
// publishes it and tells the surface's listeners. It allocates only to make
// the images, once.
func (e *Engine) draw() {
	img := e.video.Back(FrameWidth, FrameHeight, true)
	if img == nil {
		return
	}
	e.frame += 16
	for i := 0; i+3 < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = e.frame, 0x40, 0x80, 0xff
	}
	e.video.Publish()
	e.video.Notify()
}
