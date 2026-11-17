// Adapted from jplayer-tv/internal/media/avf/engine_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of the engine's calls, made once (as jplayer-mobile's avf).
var (
	errNotLoaded = fmt.Errorf("avftv: nothing loaded: %w", core.ErrInvalid)
	errClosed    = fmt.Errorf("avftv: engine closed: %w", core.ErrClosed)
	errFailed    = fmt.Errorf("avftv: the media failed to load: %w", core.ErrInvalid)
	// errNoSlot reports that too many engines live at once.
	errNoSlot = errors.New("avftv: too many engines at once")
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Engine plays one media at a time with one AVPlayer, its picture on the
// native plane under the view (Place). It is a media.Engine and a
// media.TimeReader. Its methods, and its events, run on the UI thread.
type Engine struct {
	cfg  Config
	log  *slog.Logger
	post func(fn func()) bool

	player objc.ID // owned
	item   objc.ID // owned (retained), 0 when nothing is loaded
	obs    objc.ID // owned observer instance
	slot   int     // the observer's registry slot
	plane  *nativePlane
	video  bool // the last Load has a picture (jplayer-tv's "TV item")

	events event.Topic[media.Event]
	m      machine
	limits Quality

	// The load: its token, source, and what the drain learnt of it.
	token                 uint64
	src                   media.Source
	ready, ended, failed  bool
	autoRun               bool    // play once ready (Auto, or Play before ready)
	seekGoalAtReady       float64 // where to seek once ready, when hasSeek
	hasSeek               bool
	starting              bool // onReady telling the length: a Seek is where the item starts
	pictured              bool // the load's first frame was told (EventVideoSize, checkPicture)
	back                  bool // the app is in the background (SetBackground)
	control               int  // the timeControlStatus last read, -1 before any
	dur, buffered         float64
	rate, volume          float64
	muted                 bool
	inv                   invocations
	seeks                 *seekWatch
	goal                  float64 // where the last seek goes, in the load goalToken
	goalToken             uint64
	ticker                *enginekit.Ticker
	tickInPoolFn          func()
	ranges                [1]enginekit.CMTimeRange // the tick's scratch, off the stack
	ticks                 uint64
	sessionSet, sessionOn bool
	// warming is true while warmSession activates the audio session off
	// the UI thread: Play then leaves the activation to it.
	warming bool
	nsErr   objc.ID // an NSError out-parameter: a field of a heap object (docs/conventions.md §7)
	// setActive activates or deactivates the audio session, its NSError
	// in nsErr: sessionSetActive, which a test replaces to make it fail.
	setActive func(session objc.ID, on bool) bool

	observations int
	loads        uint64
	closed       bool

	// sessionObserved tells that the observer receives the audio
	// session's interruptions (Config.OnInterruption).
	sessionObserved bool
	// intr keeps the interruptions until the UI thread's intrFn,
	// e.drainInterruptions built once, takes them.
	intr   interruptQueue
	intrFn func()

	// Set from any thread by the callbacks.
	bits      atomic.Uint32
	pending   atomic.Bool
	errorLogs atomic.Uint64
	drainFn   func() // e.drain, built once so that posting allocates nothing here
}

var (
	_ media.Engine       = (*Engine)(nil)
	_ media.TimeReader   = (*Engine)(nil)
	_ media.Backgrounder = (*Engine)(nil)
)

// New makes the engine: its AVPlayer, observed, its invocations, its time
// tick and the native picture plane. It wraps core.ErrInvalid for a bad
// Config and core.ErrUnsupported when AVFoundation cannot be reached. UI
// thread.
func New(cfg Config) (*Engine, error) {
	cfg, err := cfg.normalize()
	if err != nil {
		return nil, err
	}
	if err := resolved(); err != nil {
		return nil, err
	}
	e, err := newEngine(cfg)
	if err != nil {
		return nil, err
	}
	if e.plane, err = newNativePlane(cfg.Underlay, e.obs, e.log); err != nil {
		_ = e.Close()

		return nil, err
	}
	e.plane.setPlayer(e.player)
	if cfg.OnInterruption != nil {
		e.observeSession()
	}
	e.warmSession()

	return e, nil
}

// newEngine makes an engine without a picture plane: the invocations, the
// player, the observer, the registry slot, the tick. cfg is normalized;
// resolve ran.
func newEngine(cfg Config) (*Engine, error) {
	e := &Engine{cfg: cfg, log: cfg.Log, post: cfg.Post, slot: -1, rate: 1, volume: 1, control: -1}
	if e.log == nil {
		e.log = slog.Default()
	}
	e.drainFn = e.drain
	e.intrFn = e.drainInterruptions
	e.setActive = e.sessionSetActive
	e.tickInPoolFn = e.tickInPool
	e.ticker = enginekit.NewTicker(cfg.Clock, enginekit.TickEvery, e.tick)
	if err := e.inv.make(); err != nil {
		e.inv.release()

		return nil, err
	}
	e.obs = objc.New(av.observer)
	if e.obs == 0 {
		e.inv.release()

		return nil, fmt.Errorf("avftv: new engine: no observer: %w", core.ErrUnsupported)
	}
	slot, ok := register(e, e.obs)
	if !ok {
		objc.Release(e.obs)
		e.inv.release()

		return nil, fmt.Errorf("avftv: new engine: %w: %w", core.ErrUnsupported, errNoSlot)
	}
	e.slot = slot
	if err := e.makePlayer(); err != nil {
		unregister(e.slot)
		objc.Release(e.obs)
		e.inv.release()

		return nil, err
	}

	return e, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load makes src's item the player's: the limits and the pitch apply to it
// before the player gets it, its observations start, and it plays once
// ready if opts.Auto, from opts.Start. What was loaded is dropped first;
// its events are stale (the token). A source that is not a URL wraps
// core.ErrInvalid (the text holds no URL); after Close, core.ErrClosed.
// The length, the start position and the paused or playing state come
// as events once the item is ready; a failure as an EventError. For a
// source with a picture, its first frame on the screen comes as one
// EventVideoSize (checkPicture), once the view placed the picture.
func (e *Engine) Load(_ context.Context, src media.Source, opts media.LoadOptions) (uint64, error) {
	if e.closed {
		return e.token, errClosed
	}
	// A failed AVPlayer cannot play again (AVPlayer.h:101-103, R-T4-10);
	// one that could not be made after a failure is tried again.
	if e.player == 0 || objc.Call(e.player, sel.status) == playerFailed {
		e.log.Warn("avftv: player failed, making a new one")
		e.dropItem()
		e.releasePlayer()
		if err := e.makePlayer(); err != nil {
			e.m.stop()

			return e.token, fmt.Errorf("avftv: load: %w", err)
		}
		if e.plane != nil {
			e.plane.setPlayer(e.player)
		}
	}
	e.ticker.Stop()
	e.token++
	e.seeks.Rebase() // a completion an earlier load lost pins nothing
	// The old item stops being observed and leaves the plane before the
	// new one is made; the player lets go of it at the replace, and it is
	// released after.
	old := e.item
	if old != 0 {
		e.unobserveItem(old)
		if e.plane != nil {
			e.plane.detach(old)
		}
		e.item = 0
	}
	video := !src.AudioOnly
	var (
		item objc.ID
		err  error
	)
	objc.WithPool(func() { item, err = e.newItem(src.URL, video) })
	if err != nil {
		e.m.stop()
		if old != 0 {
			objc.Call(e.player, sel.replaceItem, 0)
			objc.Release(old)
		}
		e.bits.Store(0)

		return e.token, fmt.Errorf("avftv: load: %w", err)
	}
	e.observeItem(item)
	e.item, e.video, e.src = item, video, src
	e.ready, e.ended, e.failed, e.autoRun, e.pictured = false, false, false, false, false
	e.dur, e.buffered, e.control = 0, 0, -1
	start := max(opts.Start, 0)
	if src.Live { // a stream without end plays from its live edge
		start = 0
	}
	e.seekGoalAtReady, e.hasSeek = start, start > 0
	e.m.load()
	objc.Call(e.player, sel.replaceItem, uintptr(item))
	if old != 0 {
		objc.Release(old)
	}
	e.loads++
	e.bits.Store(0)
	if opts.Auto {
		_ = e.Play()
	}
	// The status may be ready already (a cached item): read it once.
	e.notify(flagChanged)

	return e.token, nil
}

// Play activates the audio session the first time (category playback),
// then plays at the speed set (the player's defaultRate, 04 §2.2); before
// the item is ready it plays once it is; at the end it plays from the
// start. Its events come later, on the UI thread.
func (e *Engine) Play() error {
	switch {
	case e.closed:
		return errClosed
	case e.item == 0:
		return errNotLoaded
	case e.failed:
		return errFailed
	default:
	}
	e.activateSession()
	e.m.play()
	if !e.ready {
		e.autoRun = true
		e.notify(flagChanged)

		return nil
	}
	if e.ended {
		e.ended = false
		e.seekTo(0)
	}
	objc.Call(e.player, sel.play)
	e.notify(flagChanged)

	return nil
}

// Pause pauses, keeping the position; Paused comes later, on the UI
// thread.
func (e *Engine) Pause() error {
	switch {
	case e.closed:
		return errClosed
	case e.item == 0:
		return errNotLoaded
	default:
	}
	e.m.pause()
	if !e.ready {
		e.autoRun = false
	}
	objc.Call(e.player, sel.pause)
	e.notify(flagChanged)

	return nil
}

// Stop unloads: the player pauses and lets go of its item, which is
// released with its observations; the plane hides its picture. The
// events of the load are stale; a drain already queued reads nothing.
func (e *Engine) Stop() error {
	if e.closed {
		return nil
	}
	e.m.stop()
	e.ticker.Stop()
	if e.player != 0 {
		objc.Call(e.player, sel.pause)
	}
	e.dropItem()
	if e.plane != nil {
		e.plane.hide()
	}
	e.token++
	e.ready, e.ended, e.failed, e.autoRun, e.pictured = false, false, false, false, false
	e.bits.Store(0)

	return nil
}

// Seek moves to seconds, exactly (no tolerance), and tells the time at
// once. Before the item is ready, or in answer to the EventDuration of
// onReady, it is where the item starts. A live
// stream does not move.
func (e *Engine) Seek(seconds float64) error {
	switch {
	case e.closed:
		return errClosed
	case e.item == 0:
		return errNotLoaded
	case math.IsNaN(seconds):
		return fmt.Errorf("avftv: seek to NaN: %w", core.ErrInvalid)
	case e.src.Live:
		return nil
	default:
	}
	t := max(seconds, 0)
	if e.dur > 0 {
		t = min(t, e.dur)
	}
	if !e.ready || e.starting { // onReady seeks there
		e.seekGoalAtReady, e.hasSeek = t, true

		return nil
	}
	if e.failed {
		return errFailed
	}
	e.ended = false
	e.seekTo(t)
	e.publish(media.Event{Kind: media.EventTime, Time: t})

	return nil
}

// SetVolume sets the player's volume, 0 to 1.
func (e *Engine) SetVolume(v float64) error {
	if e.closed {
		return errClosed
	}
	if math.IsNaN(v) {
		return fmt.Errorf("avftv: volume NaN: %w", core.ErrInvalid)
	}
	e.volume = min(max(v, 0), 1)
	if err := e.inv.setVolume.SetArg(0, float32(e.volume)); err != nil {
		return err
	}

	return e.inv.setVolume.Invoke(e.player)
}

// SetMuted mutes or unmutes the player.
func (e *Engine) SetMuted(on bool) error {
	if e.closed {
		return errClosed
	}
	e.muted = on
	m := uintptr(0)
	if on {
		m = 1
	}
	objc.Call(e.player, sel.setMuted, m)

	return nil
}

// SetRate sets the speed: the player's defaultRate, which play uses,
// always, and its rate only while it plays or waits to, since a rate set
// while paused would start it (as jplayer-mobile's avf, 04 §2.2). The
// item keeps its pitch (TimeDomain).
func (e *Engine) SetRate(r float64) error {
	if e.closed {
		return errClosed
	}
	if math.IsNaN(r) || r <= 0 {
		return fmt.Errorf("avftv: rate %v: %w", r, core.ErrInvalid)
	}
	e.rate = r
	if err := e.inv.setDefaultRate.SetArg(0, float32(r)); err != nil {
		return err
	}
	if err := e.inv.setDefaultRate.Invoke(e.player); err != nil {
		return err
	}
	if e.item != 0 && e.ready && e.control != controlPaused && e.control != -1 {
		if err := e.inv.setRate.SetArg(0, float32(r)); err != nil {
			return err
		}

		return e.inv.setRate.Invoke(e.player)
	}

	return nil
}

// SetLimits keeps q for the next items, and applies it to the current one
// (resolution only).
func (e *Engine) SetLimits(q Quality) {
	e.limits = q
	if e.item != 0 {
		e.applyLimits(e.item)
	}
}

// Place puts the picture at r, in DIPs of the view (the frame of the
// widget showing it in the window); an empty r hides it. The first
// non-empty Place puts the layer under the view. A Paint may call it
// every frame: it allocates nothing when r is unchanged.
func (e *Engine) Place(r geom.Rect) {
	if e.closed || e.plane == nil {
		return
	}
	e.plane.Place(r)
}

// SetMirror mirrors the picture about its vertical axis, or not
// (04 §2.3): the layer's transform, which the system applies as it
// composites the video.
func (e *Engine) SetMirror(on bool) {
	if e.closed || e.plane == nil {
		return
	}
	e.plane.setMirror(on)
}

// Mirrored tells whether the picture is mirrored.
func (e *Engine) Mirrored() bool { return e.plane != nil && e.plane.mirror }

// SetBackground follows the app's lifecycle (media.Backgrounder, 04 §6):
// in the background the time tick stops, and a media with a picture that
// plays, or is asked to, pauses, telling Paused, as the phones' player
// does (Q-P4-17). tvOS stops the video of a backgrounded app by itself
// about 2 s later without a change the engine can read, so that the
// engine would go on believing it plays (Q-P4.1-2, local/p4/logs). A
// sound plays on (TV09's music, the audio background mode). Back in the
// foreground the engine tells the time at once and reads its state again;
// it does not play by itself. UI thread.
func (e *Engine) SetBackground(on bool) {
	if e.closed || e.back == on {
		return
	}
	e.back = on
	e.ticker.SetBackground(on)
	if !on {
		if e.item != 0 && e.ready && !e.failed {
			e.ticker.Tick()
		}
		e.notify(flagChanged)

		return
	}
	if e.item != 0 && e.video && !e.failed && e.m.playAsked && !e.m.paused {
		_ = e.Pause()
	}
}

// ReadTime reads the position of the load token from the player now
// (media.TimeReader). While a seek is under way it tells the position the
// seek goes to. Kept within the length. It allocates nothing.
func (e *Engine) ReadTime(token uint64) (float64, bool) {
	if e.closed || e.item == 0 || token != e.token || !e.ready || e.failed {
		return 0, false
	}
	t := e.goal
	if !e.seeks.Pending() || e.goalToken != e.token {
		var ok bool
		if t, ok = e.position(); !ok {
			return 0, false
		}
	}
	t = max(t, 0)
	if e.dur > 0 {
		t = min(t, e.dur)
	}

	return t, true
}

// Events returns the topic the engine publishes to, on the UI thread.
func (e *Engine) Events() *event.Topic[media.Event] { return &e.events }

// Video returns nil: the picture is not drawn by the view but composited
// by the system under it (Place).
func (e *Engine) Video() *media.Surface { return nil }

// Close stops, releases the player, the invocations and the plane, frees
// the registry slot, releases the observer on the next UI turn
// (releaseObserver) and deactivates the audio session. A second Close
// does nothing.
func (e *Engine) Close() error {
	if e.closed {
		return nil
	}
	_ = e.Stop()
	e.closed = true
	if e.plane != nil {
		e.plane.close()
	}
	e.releasePlayer()
	e.unobserveSession()
	unregister(e.slot)
	e.slot = -1
	e.releaseObserver()
	e.deactivateSession()
	e.inv.release()

	return nil
}

// Stats returns the live counts of what the engine holds.
func (e *Engine) Stats() Stats {
	st := Stats{Observations: e.observations, Loads: e.loads, Ticks: e.ticks}
	if e.item != 0 {
		st.Items = 1
	}
	if e.plane != nil {
		e.plane.count(&st)
	}

	return st
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// makePlayer makes the AVPlayer, without an item, observed, at full volume
// and unmuted, at the speed set, with the completion handler of its seeks.
func (e *Engine) makePlayer() error {
	e.player = objc.New(objc.Class(av.player))
	if e.player == 0 {
		return fmt.Errorf("avftv: no AVPlayer: %w", core.ErrUnsupported)
	}
	e.observePlayer()
	e.pinVolume()
	if e.inv.setDefaultRate.SetArg(0, float32(e.rate)) == nil {
		_ = e.inv.setDefaultRate.Invoke(e.player)
	}
	e.seeks.release()
	e.seeks = e.newSeekWatch()

	return nil
}

// releasePlayer stops observing the player and releases it.
func (e *Engine) releasePlayer() {
	if e.player == 0 {
		return
	}
	e.unobservePlayer()
	objc.Release(e.player)
	e.player = 0
	e.seeks.release()
}

// releaseObserver lets go of the observer once its slot is cleared. Item
// notifications may come on any thread (AVPlayerItem.h:23): one already on
// its way when unobserveItem removed them may still message the observer,
// so it is released on the next UI turn rather than now; the callback then
// finds no engine in the registry. When the app has ended (Post refuses)
// it is left to the process's end rather than freed under a late
// notification.
func (e *Engine) releaseObserver() {
	obs := e.obs
	e.obs = 0
	if obs == 0 {
		return
	}
	if !e.post(func() { objc.Release(obs) }) {
		e.log.Debug("avftv: app ended, observer left to the process's end")
	}
}

// newItem makes the item of stream, retained, with the limits and the
// pitch algorithm applied and, for an item with a picture, attached to the
// plane. In a pool.
func (e *Engine) newItem(stream string, video bool) (objc.ID, error) {
	if stream == "" {
		return 0, fmt.Errorf("no stream: %w", core.ErrInvalid)
	}
	str := objc.NSString(stream)
	defer objc.Release(str)
	u := objc.Call(av.url, sel.urlWithString, uintptr(str))
	if u == 0 {
		return 0, fmt.Errorf("stream is not a URL: %w", core.ErrInvalid)
	}
	item := objc.Retain(objc.Call(av.item, sel.itemWithURL, uintptr(u)))
	if item == 0 {
		return 0, fmt.Errorf("no player item: %w", core.ErrUnsupported)
	}
	e.applyLimits(item)
	objc.Call(item, sel.setPitchAlgorithm, uintptr(av.pitchTimeDomain))
	if video && e.plane != nil {
		if err := e.plane.attach(item); err != nil {
			objc.Release(item)

			return 0, err
		}
	}

	return item, nil
}

// dropItem lets go of the current item: its observations end, the plane
// detaches, the player lets go of it, and it is released.
func (e *Engine) dropItem() {
	if e.item == 0 {
		return
	}
	item := e.item
	e.unobserveItem(item)
	e.item = 0
	if e.plane != nil {
		e.plane.detach(item)
	}
	if e.player != 0 {
		objc.Call(e.player, sel.replaceItem, 0)
	}
	objc.Release(item)
}

// applyLimits sets the item's preferred maximum resolution and peak bit
// rate from the engine's quality limit (limitsOf).
func (e *Engine) applyLimits(item objc.ID) {
	w, h, bps := limitsOf(e.limits)
	objc.Send[objc.ID](item, sel.setMaxResolution, cgSize{W: w, H: h})
	objc.Send[objc.ID](item, sel.setPeakBitRate, bps)
}

// pinVolume sets the player's volume to the one set (1 at first) and its
// mute to the one set.
func (e *Engine) pinVolume() {
	if e.inv.setVolume.SetArg(0, float32(e.volume)) == nil {
		_ = e.inv.setVolume.Invoke(e.player)
	}
	m := uintptr(0)
	if e.muted {
		m = 1
	}
	objc.Call(e.player, sel.setMuted, m)
}

// publish tags ev with the load's token and publishes it.
func (e *Engine) publish(ev media.Event) {
	ev.Token = e.token
	e.events.Publish(ev)
}

// onReady handles AVPlayerItemStatusReadyToPlay, once a load: the length,
// the start position, then playing if asked (Auto, or Play before
// ready), else the paused state (as jplayer-mobile's avf).
//
// The start is read after EventDuration: a Seek made in answer to it
// (starting) replaces the load's, so that the player's seek to 0 for a
// resume position past the media's end is not undone by a seek to the end
// (P4.9 D2). A Load or Stop made in answer ends here.
func (e *Engine) onReady() {
	e.ready = true
	if d, ok := e.itemDuration(); ok {
		e.dur = d
		token := e.token
		e.starting = true
		e.publish(media.Event{Kind: media.EventDuration, Duration: d})
		e.starting = false
		if e.token != token || !e.ready || e.failed {
			return
		}
	}
	start := 0.0
	if e.hasSeek {
		start = e.seekGoalAtReady
		if e.dur > 0 {
			start = min(start, e.dur)
		}
		e.seekTo(start)
	}
	e.publish(media.Event{Kind: media.EventTime, Time: start})
	if e.autoRun {
		objc.Call(e.player, sel.play)
		e.notify(flagChanged)

		return
	}
	if !e.m.playAsked {
		e.m.last = media.StatePaused
		e.publish(media.Event{Kind: media.EventState, State: media.StatePaused})
	}
}

// checkPicture tells the first frame of the load on the screen, once:
// EventVideoSize with the item's presentationSize, when pictureDue says
// so. The drain calls it on every reading (the layer's readyForDisplay is
// observed, plane.watch), the tick while it has not told. The size read
// is a struct the message answers (objc.Send), at most a few times a
// load; once told it reads nothing.
func (e *Engine) checkPicture() {
	if e.pictured || e.closed || e.item == 0 || !e.video || e.plane == nil {
		return
	}
	r := pictureReading{video: e.video, failed: e.failed, attached: e.plane.showing(e.player)}
	if r.attached {
		r.ready = e.plane.readyForDisplay()
	}
	if r.ready {
		size := objc.Send[cgSize](e.item, sel.presentationSize)
		r.w, r.h = size.W, size.H
	}
	w, h, due := pictureDue(r)
	if !due {
		return
	}
	e.pictured = true
	e.publish(media.Event{Kind: media.EventVideoSize, Width: w, Height: h})
}

// onEnded handles AVPlayerItemDidPlayToEndTimeNotification: the last
// time, then Ended, once. The machine forgets the Playing it told, so that
// playing again tells it again.
func (e *Engine) onEnded() {
	if !e.ready || e.failed || e.ended {
		return
	}
	e.ended = true
	e.ticker.Stop()
	t := e.dur
	if p, ok := e.position(); ok && (t <= 0 || p < t) {
		t = p
	}
	// Ended until Play: the reading of this drain, or of a drain already
	// queued, may still say playing (timeControlStatus turns paused after
	// the end's notification), which would tell Playing after Ended and
	// leave the engine believing it plays on (P4.1, tvOS 27 simulator).
	e.m.last, e.m.playAsked = media.StateEnded, false
	e.publish(media.Event{Kind: media.EventTime, Time: t})
	e.publish(media.Event{Kind: media.EventEnded})
}

// emitted publishes the state s the machine read: a failure as an
// EventError carrying the error read from the item or the player
// (streamError); the tick runs while the media plays.
func (e *Engine) emitted(s media.PlayState, r reading) {
	e.log.Debug("avftv: state", "state", stateName(s))
	if s == media.StateError {
		e.failed = true
		e.ticker.Stop()
		e.publish(media.Event{Kind: media.EventError, Err: e.failure(r)})

		return
	}
	if s == media.StatePlaying {
		e.ended = false
		e.ticker.Start()
	} else {
		e.ticker.Stop()
	}
	e.publish(media.Event{Kind: media.EventState, State: s})
}

// failure reads why the stream failed: the item's error, else the
// player's, else (failed to play to the end) the last entry of the item's
// error log.
func (e *Engine) failure(r reading) error {
	var err error
	objc.WithPool(func() {
		switch {
		case r.item == itemFailed:
			err = errorOf(objc.Call(e.item, sel.errorOf))
		case r.player == playerFailed:
			err = errorOf(objc.Call(e.player, sel.errorOf))
		default:
			err = e.lastErrorLogged()
		}
	})

	return err
}

// lastErrorLogged is the error of the item's last error log entry. In a
// pool.
func (e *Engine) lastErrorLogged() error {
	log := objc.Call(e.item, sel.errorLog)
	if log == 0 {
		return streamError("unknown", 0, "", 0)
	}
	ev := objc.Call(objc.Call(log, sel.events), sel.lastObject)
	if ev == 0 {
		return streamError("unknown", 0, "", 0)
	}

	return streamError(objc.GoString(objc.Call(ev, sel.errorDomain)), int(objc.Call(ev, sel.errorStatusCode)), "", 0)
}

// errorOf is the error of an NSError: its domain and code, and those of
// its innermost underlying error. In a pool.
func errorOf(nsErr objc.ID) error {
	if nsErr == 0 {
		return streamError("unknown", 0, "", 0)
	}
	domain, code := objc.GoString(objc.Call(nsErr, sel.domain)), int(objc.Call(nsErr, sel.code))
	under, underCode := "", 0
	for u, depth := underlying(nsErr), 0; u != 0 && depth < 8; u, depth = underlying(u), depth+1 {
		under, underCode = objc.GoString(objc.Call(u, sel.domain)), int(objc.Call(u, sel.code))
	}

	return streamError(domain, code, under, underCode)
}

// underlying is the NSUnderlyingErrorKey of nsErr's userInfo, 0 for none.
func underlying(nsErr objc.ID) objc.ID {
	info := objc.Call(nsErr, sel.userInfo)
	if info == 0 {
		return 0
	}

	return objc.Call(info, sel.objectForKey, uintptr(av.underlyingKey))
}
