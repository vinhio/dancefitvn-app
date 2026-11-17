// Adapted from jplayer-mobile/internal/media/avf/engine_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"log"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of the engine's calls, made once.
var (
	errNotLoaded = fmt.Errorf("avf: nothing loaded: %w", core.ErrInvalid)
	errClosed    = fmt.Errorf("avf: engine closed: %w", core.ErrClosed)
	errFailed    = fmt.Errorf("avf: the media failed to load: %w", core.ErrInvalid)
	errNotURL    = fmt.Errorf("avf: not a link or a path: %w", media.ErrSource)
)

// The engine's timing.
const (
	// settleFor is how long the UI keeps pulling, after a seek or a pause,
	// for the frame of the new position.
	settleFor = time.Second
	// endSlack is how close to the end a pause is the end's: the player
	// stops there by itself, which the media element does not report as a
	// pause (player.js:52).
	endSlack = 0.05
	// durationSlack is how much the length must change for the tick to
	// report it again (a stream learns its length as it goes).
	durationSlack = 0.5
	// nextRetry is how many frames the old output may give while the new
	// one, of another size, gives none, before another new one replaces
	// it (followSize).
	nextRetry = 12
	// nextRetries is how many times followSize makes the next output
	// again, for a size the item presents, before it gives up on that size
	// and keeps the old output until the size changes again.
	nextRetries = 3
	// shapeSlack is how far the proportions of two frame sizes may differ
	// and still be the same shape (sameShape).
	shapeSlack = 0.015
)

// cgSize is CoreGraphics' CGSize (16 bytes).
type cgSize struct{ W, H float64 }

// cgAffineTransform is CoreGraphics' CGAffineTransform (48 bytes).
type cgAffineTransform struct{ A, B, C, D, Tx, Ty float64 }

// invocations are the engine's messages that pass or answer floats and
// structs (04 §2.1, GMF.1): made once by New, released by Close; UI thread.
type invocations struct {
	currentTime        *objc.Invocation // -[AVPlayer currentTime], AVPlayer.h:291
	duration           *objc.Invocation // -[AVPlayerItem duration], AVPlayerItem.h:206
	rangeValue         *objc.Invocation // -[NSValue CMTimeRangeValue], AVTime.h:32
	seek               *objc.Invocation // -[AVPlayer seekToTime:toleranceBefore:toleranceAfter:completionHandler:], AVPlayer.h:358
	setRate            *objc.Invocation // -[AVPlayer setRate:], AVPlayer.h:152
	setDefaultRate     *objc.Invocation // -[AVPlayer setDefaultRate:], AVPlayer.h:163
	setVolume          *objc.Invocation // -[AVPlayer setVolume:], AVPlayer.h:490
	presentationSize   *objc.Invocation // -[AVPlayerItem presentationSize], AVPlayerItem.h:217
	naturalSize        *objc.Invocation // -[AVAssetTrack naturalSize], AVAssetTrack.h:118
	preferredTransform *objc.Invocation // -[AVAssetTrack preferredTransform], AVAssetTrack.h:121
	variantSize        *objc.Invocation // -[AVAssetVariantVideoAttributes presentationSize], AVAssetVariant.h:65
	setPeak            *objc.Invocation // -[AVPlayerItem setPreferredPeakBitRate:], AVPlayerItem.h:539
	setMaxResolution   *objc.Invocation // -[AVPlayerItem setPreferredMaximumResolution:], AVPlayerItem.h:552
	maxResolution      *objc.Invocation // -[AVPlayerItem preferredMaximumResolution], AVPlayerItem.h:552
}

// Stats are an engine's counters, for the device tests (100 loads leave one
// item and one observer, say).
type Stats struct {
	// LiveItems is how many AVPlayerItems the engine holds.
	LiveItems int
	// Observers is how many load observers of the engine are registered.
	Observers int
	// Frames is how many frames the pull copied; Pulls how many pulls ran.
	Frames, Pulls int
	// Outputs counts the video outputs made again because the item's
	// frames changed size (an HLS variant of another size).
	Outputs int
	// NextOutputs counts the next outputs made for frames of another size
	// (followSize), those made again for one that got no frame included;
	// NextGiveUps how many times followSize gave up on a size after
	// nextRetries of them.
	NextOutputs, NextGiveUps int
	// Players counts the AVPlayers the engine made: one, and one more for
	// each reset of the media services it followed.
	Players int
	// VideoTracksOff is how many video tracks of the current item are
	// disabled (in the background, D4 of M4.5).
	VideoTracksOff int
	// FootprintBytes is the process's physical footprint (task_info).
	FootprintBytes int64
}

// seekWatch counts the seeks asked of one AVPlayer and those it finished:
// its completion handler is called once for each, finished or given up
// for a later one (AVPlayer.h:351-353). Until it is, the player's
// currentTime may still be the position it left, so ReadTime tells the
// position the seek goes to. Counted on the UI thread, from a baseline at
// each load (enginekit.SeekCount).
type seekWatch struct {
	enginekit.SeekCount
	// block is the completion handler given with every seek of the
	// player, owned; made with the player, released with it.
	block objc.ID
}

// engine is the media.Engine of iOS: one AVPlayer for its life, an item per
// Load. Everything runs on the UI thread, the main thread of iOS.
type engine struct {
	cfg  media.Config
	sess media.Session

	events event.Topic[media.Event]
	video  media.Surface

	player objc.ID // owned
	// seeks follows the seeks of player; goal is where the last one goes,
	// in the load goalToken.
	seeks     *seekWatch
	goal      float64
	goalToken uint64
	// scene observes the app's scene leaving the background (owned).
	scene  objc.ID
	inv    invocations
	ticker *enginekit.Ticker
	// tickInPoolFn is the tick's func, made once.
	tickInPoolFn func()
	// outputWait looks for the output of a video whose size was not known
	// when it became ready (a stream), a tick apart, playing or paused,
	// until it has one; outputInPoolFn is its func, made once.
	outputWait     *enginekit.Ticker
	outputInPoolFn func()

	token uint64
	cur   *load

	rate, volume float64
	muted        bool
	back         bool // the app is in the background
	closed       bool

	// force makes the next pull take the frame shown even if it is not new
	// (back from the background, paused).
	force bool
	// settling keeps the UI pulling until a frame comes or settleUntil.
	settling    bool
	settleUntil time.Time

	// lateOutput makes a video get its output from outputWait rather than
	// when it is ready, as a stream whose size is unknown then does
	// (SetLateOutput, for the device tests).
	lateOutput bool
	// keepTracks keeps the video tracks enabled in the background, as
	// before D4 (SetKeepVideoInBackground, for the device measures).
	keepTracks bool
	// noVariants sizes the output of an HLS stream for the frames it
	// presents first, not its largest variant (SetVariantSizing, for the
	// device tests of followSize).
	noVariants bool
	// starveNext keeps the pull from taking a next output's frames, as if
	// it got none (SetNextStarved, for the device tests of followSize).
	starveNext bool

	// maxRes is the preferredMaximumResolution of every item, in pixels
	// (SetMaxResolution, the app's quality setting); 0×0 is no limit.
	maxRes cgSize

	// resets is the count of the session's media resets the player was
	// made after: behind the session's, the next Load makes a new one.
	resets, players int

	// The tick's scratch, off the stack.
	ranges [1]enginekit.CMTimeRange

	liveItems, frames, pulls int
	// outputs counts the outputs made again for frames of another size
	// (followSize); nextMade the next outputs made, nextGaveUp the sizes
	// followSize gave up on (Stats).
	outputs, nextMade, nextGaveUp int
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns the iOS engine on AVFoundation. sess is the audio session the
// engine activates before it plays (04 §2.3). Call it on the UI thread.
func New(cfg media.Config, sess media.Session) (media.Engine, error) {
	if cfg.Dispatch == nil || cfg.Clock == nil {
		return nil, fmt.Errorf("avf: config without dispatch or clock: %w", core.ErrInvalid)
	}
	if err := initAVF(); err != nil {
		return nil, err
	}
	e := &engine{cfg: cfg, sess: sess, rate: 1, volume: 1}
	if err := e.inv.make(); err != nil {
		e.inv.release()

		return nil, err
	}
	e.resets = resetsOf(sess)
	if e.player = newPlayer(); e.player == 0 {
		e.inv.release()

		return nil, fmt.Errorf("avf: no player: %w", core.ErrNotFound)
	}
	e.players++
	e.seeks = e.newSeekWatch()
	e.tickInPoolFn = e.tickInPool
	e.ticker = enginekit.NewTicker(cfg.Clock, enginekit.TickEvery, e.tick)
	e.outputInPoolFn = e.waitOutputInPool
	e.outputWait = enginekit.NewTicker(cfg.Clock, enginekit.TickEvery, e.waitOutput)
	e.video.SetPull(e.pull)
	e.scene = objc.New(av.cls.observer)
	watchEngine(e.scene, e)
	objc.WithPool(func() {
		objc.Call(center(), av.sel.addObserver, uintptr(e.scene), uintptr(av.sel.enteringForeground), uintptr(av.willEnterForeground), 0)
	})

	return e, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load replaces what plays with src (04 §2.1). A local file that is gone is
// media.ErrMissingFile at once; any other failure comes as an EventError.
func (e *engine) Load(_ context.Context, src media.Source, opts media.LoadOptions) (uint64, error) {
	if e.closed {
		return e.token, errClosed
	}
	e.unload()
	if err := e.renew(); err != nil {
		return e.token, err
	}
	e.seeks.Rebase() // a completion an earlier load lost pins nothing (F5 of M9.R)
	e.token++
	e.video.SetActive(false)
	e.video.Reset()
	e.settling, e.force = false, false
	local := !strings.Contains(src.URL, "://")
	if err := enginekit.OpenError(enginekit.PhaseOpening, local, src.URL, nil); err != nil {
		e.video.Free()

		return e.token, err
	}
	if src.AudioOnly {
		e.video.Free()
	}
	start := max(opts.Start, 0)
	if src.Live { // a stream without end plays from its live edge
		start = 0
	}
	l := &load{
		e: e, token: e.token, src: src, local: local, autoRun: opts.Auto,
		seekTo: start, hasSeek: start > 0, control: -1,
	}
	var err error
	objc.WithPool(func() { err = e.open(l) })
	if err != nil {
		return e.token, err
	}
	e.cur = l

	return e.token, nil
}

// Play plays, once loaded; at the end it plays from the start.
func (e *engine) Play() error {
	l := e.cur
	switch {
	case e.closed:
		return errClosed
	case l == nil:
		return errNotLoaded
	case l.failed:
		return errFailed
	default:
	}
	if err := e.sess.Activate(); err != nil {
		// As the media element's refused play(): it plays on anyway.
		log.Printf("dancefitvn: avf audio session: %v", err)
	}
	if !l.ready {
		l.autoRun = true

		return nil
	}
	if l.ended {
		l.ended = false
		e.seekTo(l, 0)
	}
	objc.Call(e.player, av.sel.play)

	return nil
}

// Pause pauses, keeping the position.
func (e *engine) Pause() error {
	l := e.cur
	switch {
	case e.closed:
		return errClosed
	case l == nil:
		return errNotLoaded
	case !l.ready:
		l.autoRun = false

		return nil
	default:
	}
	objc.Call(e.player, av.sel.pause)

	return nil
}

// Stop stops and unloads: the events of the load are stale.
func (e *engine) Stop() error {
	if e.closed {
		return nil
	}
	e.unload()
	objc.Call(e.player, av.sel.replaceItem, 0)
	e.token++
	e.video.SetActive(false)
	e.settling, e.force = false, false
	e.video.Free()

	return nil
}

// Seek moves to seconds, exactly (no tolerance, as the media element's
// currentTime), and tells the time at once. A live stream does not move:
// AVPlayer would seek within the playlist's window, behind the live edge,
// which the app never shows (local/online/spike.md §5), as amp does.
// Before ready, or in answer to the EventDuration of onReady, it is where
// the load starts, told by onReady's EventTime.
func (e *engine) Seek(seconds float64) error {
	l := e.cur
	switch {
	case e.closed:
		return errClosed
	case l == nil:
		return errNotLoaded
	case math.IsNaN(seconds):
		return fmt.Errorf("avf: seek to NaN: %w", core.ErrInvalid)
	case l.src.Live:
		return nil
	default:
	}
	t := max(seconds, 0)
	if l.dur > 0 {
		t = min(t, l.dur)
	}
	if !l.ready || l.starting { // onReady seeks there (onReady)
		l.seekTo, l.hasSeek = t, true

		return nil
	}
	if l.failed {
		return errFailed
	}
	l.ended = false
	e.seekTo(l, t)
	e.publish(l, media.Event{Kind: media.EventTime, Time: t})
	if l.output != 0 && l.control != controlPlaying {
		e.settle()
	}

	return nil
}

// SetVolume sets the player's volume, 0 to 1.
func (e *engine) SetVolume(v float64) error {
	if e.closed {
		return errClosed
	}
	if math.IsNaN(v) {
		return fmt.Errorf("avf: volume NaN: %w", core.ErrInvalid)
	}
	e.volume = min(max(v, 0), 1)
	if err := e.inv.setVolume.SetArg(0, float32(e.volume)); err != nil {
		return err
	}

	return e.inv.setVolume.Invoke(e.player)
}

// SetMuted mutes or unmutes the player.
func (e *engine) SetMuted(on bool) error {
	if e.closed {
		return errClosed
	}
	e.muted = on
	m := uintptr(0)
	if on {
		m = 1
	}
	objc.Call(e.player, av.sel.setMuted, m)

	return nil
}

// SetRate sets the speed: the player's defaultRate, which play uses, always,
// and its rate only while it plays, since a rate set while paused would
// start it (04 §2.2). The item keeps its pitch (TimeDomain, open).
func (e *engine) SetRate(r float64) error {
	if e.closed {
		return errClosed
	}
	if math.IsNaN(r) || r <= 0 {
		return fmt.Errorf("avf: rate %v: %w", r, core.ErrInvalid)
	}
	e.rate = r
	if err := e.inv.setDefaultRate.SetArg(0, float32(r)); err != nil {
		return err
	}
	if err := e.inv.setDefaultRate.Invoke(e.player); err != nil {
		return err
	}
	if l := e.cur; l != nil && l.ready && l.control != controlPaused && l.control != -1 {
		if err := e.inv.setRate.SetArg(0, float32(r)); err != nil {
			return err
		}

		return e.inv.setRate.Invoke(e.player)
	}

	return nil
}

// SetBackground follows the app's lifecycle (media.Backgrounder): in the
// background the tick stops, and so do the pulls: the surface is no longer
// Active, so the views stop repainting for frames (they would go on copying
// and drawing at the display's rate, as the window keeps painting in the
// background), and nothing activates it again until the foreground. The
// video tracks of the item are disabled too, so that the player decodes
// the sound only (D4 of M4.5; a load made in the background starts so).
// They are enabled again as the scene leaves the background
// (leavingBackground), which gives the decoder a head start on the frame
// to show: from the last key frame, up to a few hundred milliseconds of
// decoding. Back in the foreground the engine tells the time and takes the
// frame shown. The background told again (the app left it, then went back
// without becoming active) disables the tracks again. The video's images
// are freed in the background (F3 of M11.P): nothing pulls there, and the
// first pull back makes them again. A source with a BackgroundPeak is
// capped at it in the background (capPeak), and lifted back
// (liftPeak).
func (e *engine) SetBackground(on bool) {
	if e.closed {
		return
	}
	l := e.cur
	ready := l != nil && l.ready && !l.failed
	if e.back == on {
		if on && ready {
			e.setVideoTracks(l, false)
		}
		if on {
			e.capPeak(l)
			e.video.Free()
		}

		return
	}
	e.back = on
	e.ticker.SetBackground(on)
	e.outputWait.SetBackground(on)
	if on {
		e.settling = false
		e.video.SetActive(false)
		if ready {
			e.setVideoTracks(l, false)
		}
		e.capPeak(l)
		e.video.Free()

		return
	}
	flush := e.liftPeak(l)
	if !ready {
		return
	}
	e.wakeVideo(l, flush)
	if flush {
		e.flushPeak(l)
	}
	e.ticker.Tick()
	if l.output != 0 {
		e.force = true
		if l.control == controlPlaying {
			e.activate()
		} else {
			e.settle()
		}
	}
}

// ReadTime reads the position of the load token from the player now
// (media.TimeReader): in the background no tick tells it (F1 of M9). The
// player's currentTime may be the position a seek left until the seek
// completes (seekWatch): meanwhile it tells the position the seek goes
// to. Kept within the length. It allocates nothing (an invocation made
// once, no object).
func (e *engine) ReadTime(token uint64) (float64, bool) {
	l := e.cur
	if e.closed || l == nil || l.token != token || !l.ready || l.failed {
		return 0, false
	}
	t := e.goal
	if !e.seeks.Pending() || e.goalToken != l.token {
		var ok bool
		if t, ok = e.position(); !ok {
			return 0, false
		}
	}
	t = max(t, 0)
	if l.dur > 0 {
		t = min(t, l.dur)
	}

	return t, true
}

// Events returns the topic the engine publishes to, on the UI thread.
func (e *engine) Events() *event.Topic[media.Event] { return &e.events }

// Video returns the surface the frames of a video are copied to.
func (e *engine) Video() *media.Surface { return &e.video }

// Close stops and releases the player and the invocations.
func (e *engine) Close() error {
	if e.closed {
		return nil
	}
	_ = e.Stop()
	e.closed = true
	e.video.SetPull(nil)
	objc.WithPool(func() { objc.Call(center(), av.sel.removeObserver, uintptr(e.scene)) })
	watchEngine(e.scene, nil)
	objc.Release(e.scene)
	e.scene = 0
	e.inv.release()
	objc.Release(e.player)
	e.player = 0
	e.seeks.release()

	return nil
}

// StatsOf returns the counters of e, an engine of this package (false for
// another).
func StatsOf(e media.Engine) (Stats, bool) {
	en, ok := e.(*engine)
	if !ok {
		return Stats{}, false
	}

	st := Stats{
		LiveItems: en.liveItems, Observers: observers(en),
		Frames: en.frames, Pulls: en.pulls, Outputs: en.outputs, NextOutputs: en.nextMade, NextGiveUps: en.nextGaveUp,
		Players: en.players, FootprintBytes: footprint(),
	}
	if l := en.cur; l != nil {
		st.VideoTracksOff = l.videoOff
	}

	return st, true
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// publish tags ev with the token of l and publishes it.
func (e *engine) publish(l *load, ev media.Event) {
	ev.Token = l.token
	e.events.Publish(ev)
}

// setState publishes a change of state of l, once per change.
func (e *engine) setState(l *load, s media.PlayState) {
	if l.state == s {
		return
	}
	l.state = s
	e.publish(l, media.Event{Kind: media.EventState, State: s})
}

// seekTo seeks the player to t seconds exactly, for l, and counts the
// seek until its completion handler tells it is over (seekWatch). UI
// thread.
func (e *engine) seekTo(l *load, t float64) {
	zero := enginekit.CMTime{Timescale: 1, Flags: enginekit.CMTimeFlagValid}
	in := e.inv.seek
	if in.SetArg(0, enginekit.CMTimeOf(t)) != nil || in.SetArg(1, zero) != nil || in.SetArg(2, zero) != nil ||
		in.SetArg(3, e.seeks.block) != nil {
		return
	}
	if in.Invoke(e.player) != nil {
		return
	}
	e.seeks.Ask()
	e.goal, e.goalToken = t, l.token
}

// newSeekWatch returns the seekWatch of a new player: its completion
// handler, one block for every seek, counts the seek done on the UI
// thread. Its C callback is made once (newBlock).
func (e *engine) newSeekWatch() *seekWatch {
	w := &seekWatch{}
	// A seek landing while the UI pulls for its frame gives the frame
	// settleFor more to come (pull keeps pulling while a seek is under way).
	done := func() {
		w.Done()
		if e.settling && e.seeks == w {
			e.settleUntil = e.cfg.Clock.Now().Add(settleFor)
		}
	}
	w.block = newBlock(seekArgs, func([]reflect.Value) {
		if objc.IsMainThread() {
			done()

			return
		}
		e.cfg.Dispatch(done)
	})

	return w
}

// release releases the completion handler; the player keeps its copies
// for the seeks under way.
func (w *seekWatch) release() {
	if w != nil && w.block != 0 {
		objc.Release(w.block)
		w.block = 0
	}
}

// seekArgs are the arguments of a seek's completion handler after the
// block: BOOL finished, which it does not read (a register's width).
var seekArgs = []reflect.Type{reflect.TypeFor[uintptr]()}

// position returns the player's time in seconds.
func (e *engine) position() (float64, bool) {
	if e.inv.currentTime.Invoke(e.player) != nil {
		return 0, false
	}
	t, err := e.inv.currentTime.Result[enginekit.CMTime]()
	if err != nil {
		return 0, false
	}

	return t.Seconds()
}

// itemDuration returns the length of l's item, false while unknown or for a
// stream without end.
func (e *engine) itemDuration(l *load) (float64, bool) {
	if e.inv.duration.Invoke(l.item) != nil {
		return 0, false
	}
	t, err := e.inv.duration.Result[enginekit.CMTime]()
	if err != nil {
		return 0, false
	}
	d, ok := t.Seconds()

	return d, ok && d > 0
}

// settle keeps the UI pulling for the frame of a new position, until one
// comes or settleFor passes.
func (e *engine) settle() {
	e.settling = true
	e.settleUntil = e.cfg.Clock.Now().Add(settleFor)
	e.activate()
}

// activate tells that frames are coming (Surface.SetActive) and asks the
// views to paint (Surface.Notify): a view pulls only from its paint, so
// without a paint to start them the pulls would wait for something else on
// screen to move, and a mini player or a screen of the video alone would
// stay on its old frame. In the background it does nothing: SetBackground
// activates again, if the media plays or a frame is due, on the way back.
// UI thread.
func (e *engine) activate() {
	if e.back {
		return
	}
	e.video.SetActive(true)
	e.video.Notify()
}

// endSettle stops the pulls a settle asked for, unless the media plays.
func (e *engine) endSettle() {
	e.settling = false
	if l := e.cur; l == nil || l.control != controlPlaying {
		e.video.SetActive(false)
	}
}

// renew makes a new AVPlayer when the media services were reset since the
// one the engine has was made (D5): the old one, and every object of the
// old services, is dead. The new player gets the speed, the volume and the
// mute asked so far. It runs between the unload and the load, nothing
// loaded. UI thread.
func (e *engine) renew() error {
	n := resetsOf(e.sess)
	if n == e.resets {
		return nil
	}
	p := newPlayer()
	if p == 0 {
		return fmt.Errorf("avf: no player after a reset of the media services: %w", core.ErrNotFound)
	}
	objc.Call(e.player, av.sel.replaceItem, 0)
	objc.Release(e.player)
	e.player, e.resets = p, n
	e.players++
	// The old player's seeks may never be told done: the new one starts
	// its own count.
	e.seeks.release()
	e.seeks = e.newSeekWatch()
	log.Printf("dancefitvn: avf a new player after the reset of the media services")
	var err error
	if e.inv.setDefaultRate.SetArg(0, float32(e.rate)) == nil {
		err = e.inv.setDefaultRate.Invoke(e.player)
	}
	if verr := e.SetVolume(e.volume); verr != nil && err == nil {
		err = verr
	}
	if merr := e.SetMuted(e.muted); merr != nil && err == nil {
		err = merr
	}
	if err != nil {
		log.Printf("dancefitvn: avf new player: %v", err)
	}

	return nil
}

// leavingBackground enables the video tracks again as the app's scene
// leaves the background, before SetBackground(false) comes with the
// lifecycle's Active. UI thread.
func (e *engine) leavingBackground() {
	if !e.back || e.closed {
		return
	}
	l := e.cur
	flush := e.liftPeak(l)
	if l != nil && l.ready && !l.failed {
		e.wakeVideo(l, flush)
		if flush {
			e.flushPeak(l)
		}
	}
}

// wakeVideo enables the video tracks of l disabled in the background; a
// paused player decodes nothing of a track enabled again, so a seek where
// it is makes it decode the frame shown, unless flushPeak follows (flush),
// which seeks there. UI thread.
func (e *engine) wakeVideo(l *load, flush bool) {
	if l.videoOff == 0 {
		return
	}
	e.setVideoTracks(l, true)
	if !flush && l.output != 0 && l.control != controlPlaying && !l.ended {
		if t, ok := e.position(); ok {
			e.seekTo(l, t)
		}
	}
}

// capPeak caps the bit rate of l's item at its source's BackgroundPeak
// (AVPlayerItem.preferredPeakBitRate), in the background: AVPlayer
// disables no fetch of the video track of an HLS stream it does not
// decode, and would go on fetching its 1080p variant (60 MB in five
// minutes, local/youtube/spikeS1.md (c)); capped, it fetches its smallest
// variant. A source without one (a live channel: its smaller variants have
// a poorer sound) or a live one is left alone. It does nothing for a load
// already capped. UI thread; allocates nothing.
func (e *engine) capPeak(l *load) {
	if l == nil || l.item == 0 || l.capped || l.src.BackgroundPeak <= 0 || l.src.Live {
		return
	}
	if e.setPeak(l, float64(l.src.BackgroundPeak)) {
		l.capped = true
		log.Printf("dancefitvn: avf background: capped at %d bit/s", l.src.BackgroundPeak)
	}
}

// liftPeak lifts the cap of capPeak from l's item, and reports whether
// what was fetched under it is to be thrown away (flushPeak): the item is
// ready and not over. UI thread; allocates nothing.
func (e *engine) liftPeak(l *load) bool {
	if l == nil || !l.capped || l.item == 0 {
		return false
	}
	l.capped = false

	return e.setPeak(l, 0) && l.ready && !l.failed && !l.ended
}

// flushPeak throws away what AVPlayer fetched of l's item while it was
// capped, once its video tracks are enabled again: the cap, lowered in the
// background, made AVPlayer fetch the rest of the video at 240p (the
// 1080p fetched before dropped), and lifted it fetches nothing more until
// that is played out, at 240p, up to the end of the video. A seek does not
// throw it away, where it is, 0.01 s or 2 s on, nor one far in the video,
// as soon as the cap is lifted or once active (the simulator, iOS 26.5,
// M19 YH.E; local/youtube/spikeS1.md P5 saw a seek do it with the app in
// front). The item taken out of the player and put back does: the player
// fetches again from where it is, at the variant its measures choose
// (1080p over Wi-Fi). It plays on if it played or waited to, from the
// position (a seek with zero tolerance); a pause shows the frame there.
// The state goes loading for about 0.4 s. UI thread, once a return to the
// foreground; it logs.
func (e *engine) flushPeak(l *load) {
	t := e.goal
	if !e.seeks.Pending() || e.goalToken != l.token {
		var ok bool
		if t, ok = e.position(); !ok {
			return
		}
	}
	t = max(t, 0)
	if l.dur > 0 {
		t = min(t, l.dur)
	}
	playing := l.control == controlPlaying || l.control == controlWaiting // as SetRate tells it
	objc.Call(e.player, av.sel.replaceItem, 0)
	objc.Call(e.player, av.sel.replaceItem, uintptr(l.item))
	e.seekTo(l, t)
	if playing {
		objc.Call(e.player, av.sel.play)
	}
	log.Printf("dancefitvn: avf foreground: cap lifted, the item put back at %.2f s (playing %t)", t, playing)
}

// setPeak sets the preferredPeakBitRate of l's item, bps bits per second
// (0 lifts it), and reports whether it could.
func (e *engine) setPeak(l *load, bps float64) bool {
	in := e.inv.setPeak

	return in.SetArg(0, bps) == nil && in.Invoke(l.item) == nil
}

// applyMaxResolution sets the preferredMaximumResolution of item to the
// engine's limit (0×0 lifts it) and reports whether it could.
func (e *engine) applyMaxResolution(item objc.ID) bool {
	in := e.inv.setMaxResolution

	return in.SetArg(0, e.maxRes) == nil && in.Invoke(item) == nil
}

// newPlayer returns an owned AVPlayer, 0 when none could be made.
func newPlayer() objc.ID {
	return objc.Call(objc.Call(av.cls.player, av.sel.alloc), av.sel.init)
}

// setVideoTracks enables or disables the video tracks of l's item
// (AVPlayerItemTrack.enabled, AVPlayerItemTrack.h:52): disabled, the
// player decodes only the sound (D4 of M4.5). It keeps how many it
// disabled, and does nothing when that is already so; an item without a
// video track, or a load that asked for audio only, has none to disable,
// and with keepTracks the tracks stay enabled. UI thread.
func (e *engine) setVideoTracks(l *load, on bool) {
	if on == (l.videoOff == 0) || (!on && (l.src.AudioOnly || e.keepTracks)) {
		return
	}
	n := 0
	objc.WithPool(func() {
		tracks := objc.Call(l.item, av.sel.tracks)
		for i := range int(objc.Call(tracks, av.sel.count)) {
			track := objc.Call(tracks, av.sel.objectAtIndex, uintptr(i))
			if videoAsset(track) == 0 {
				continue
			}
			v := uintptr(0)
			if on {
				v = 1
			}
			objc.Call(track, av.sel.setEnabled, v)
			n++
		}
	})
	if on {
		l.videoOff = 0
	} else {
		l.videoOff = n
	}
}

// make makes the invocations, in a pool of their own.
func (in *invocations) make() error {
	for _, x := range []struct {
		dst  **objc.Invocation
		cls  string
		name string
	}{
		{&in.currentTime, "AVPlayer", "currentTime"},
		{&in.duration, "AVPlayerItem", "duration"},
		{&in.rangeValue, "NSValue", "CMTimeRangeValue"},
		{&in.seek, "AVPlayer", "seekToTime:toleranceBefore:toleranceAfter:completionHandler:"},
		{&in.setRate, "AVPlayer", "setRate:"},
		{&in.setDefaultRate, "AVPlayer", "setDefaultRate:"},
		{&in.setVolume, "AVPlayer", "setVolume:"},
		{&in.presentationSize, "AVPlayerItem", "presentationSize"},
		{&in.naturalSize, "AVAssetTrack", "naturalSize"},
		{&in.preferredTransform, "AVAssetTrack", "preferredTransform"},
		{&in.variantSize, "AVAssetVariantVideoAttributes", "presentationSize"},
		{&in.setPeak, "AVPlayerItem", "setPreferredPeakBitRate:"},
		{&in.setMaxResolution, "AVPlayerItem", "setPreferredMaximumResolution:"},
		{&in.maxResolution, "AVPlayerItem", "preferredMaximumResolution"},
	} {
		made, err := objc.NewInvocation(objc.GetClass(x.cls), x.name)
		if err != nil {
			return fmt.Errorf("avf: -[%s %s]: %w", x.cls, x.name, err)
		}
		*x.dst = made
	}

	return nil
}

// release releases the invocations made.
func (in *invocations) release() {
	for _, x := range []*objc.Invocation{
		in.currentTime, in.duration, in.rangeValue, in.seek, in.setRate, in.setDefaultRate,
		in.setVolume, in.presentationSize, in.naturalSize, in.preferredTransform, in.variantSize, in.setPeak,
		in.setMaxResolution, in.maxResolution,
	} {
		if x != nil {
			x.Release()
		}
	}
}
