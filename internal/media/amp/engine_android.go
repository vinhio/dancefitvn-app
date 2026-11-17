// Adapted from jplayer-mobile/internal/media/amp/engine_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"runtime/cgo"
	"sync/atomic"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of calls that have nothing to work on.
var (
	errNotLoaded = fmt.Errorf("amp: nothing loaded: %w", core.ErrInvalid)
	errClosed    = fmt.Errorf("amp: engine closed: %w", core.ErrClosed)
	errFailed    = fmt.Errorf("amp: the load failed: %w", core.ErrInvalid)
)

// loadRing is how many load and unload times the engine keeps for Stats.
const loadRing = 128

// releaseQueue is how many loads may wait for the releaser before a load
// waits for it.
const releaseQueue = 16

// freeDelay is how long an audio load keeps the video's images before
// they are freed (R8): a video of the same size that comes meanwhile, as
// the next of a queue mixing audio and video, draws into them again
// instead of making three new ones (11 MB at 1280 by 720). It is a reuse
// only: the renderer no longer pins the images of the last frames (GMF16),
// so freeing them at once would leak nothing.
const freeDelay = 10 * time.Second

// engine is the media.Engine on android.media.MediaPlayer, and a
// media.Backgrounder. Everything runs on the UI thread but the frames,
// the making of a load's player (the opener) and its release (the
// releaser).
type engine struct {
	cfg    media.Config
	sess   media.Session
	events event.Topic[media.Event]
	video  media.Surface
	ticker *enginekit.Ticker
	prod   *producer
	handle cgo.Handle // the producer, as the readers' C reaches it

	token  uint64
	cur    *load
	vol    float64
	muted  bool
	rate   float64
	back   bool
	closed bool
	wake   bool // the app holds WAKE_LOCK: the player keeps the CPU awake
	// wifi keeps the Wi-Fi up while a link plays or prepares to (D7 of
	// M9); it does nothing without WAKE_LOCK.
	wifi *wifiLock
	// handoff keeps the CPU awake from a media's end to the next's start.
	handoff *handoff

	attrsMusic, attrsMovie jni.Object // AudioAttributes, global
	params                 jni.Object // the PlaybackParams setSpeed reuses, global

	tickDo func(jni.Env) error // made once
	pos    int32               // the tick's position, ms

	// pauseDo, seekDo, volumeDo, seekPosDo, startDo and rateDo are the
	// calls into Java of Pause, Seek, the volume (the sleep timer's fade
	// sets it four times a second), a finished seek, Play and the speed,
	// made once: a closure handed to jni.Do
	// would be allocated at every call. They work on op, the load of the
	// call, set just before and cleared after; seekDo seeks to opMs, and
	// seekPosDo reads the position into opPos. UI thread.
	pauseDo, seekDo, volumeDo, seekPosDo, startDo, rateDo func(jni.Env) error
	op                                                    *load
	opMs                                                  int64
	opPos                                                 int32

	refused, wrongState int
	// detaches and attaches count the Surfaces taken from the players in
	// the background and given back (background_android.go).
	detaches, attaches int
	// reseeks counts the seeks again after a Surface given back, as the
	// player went to a key frame (background_android.go reseek).
	reseeks        int
	loads, unloads durations
	opens          timeRing // the opener's times

	freeT  clock.Timer // the Free of an audio load's images, 0 when none
	freeFn func()      // made once

	next      atomic.Pointer[load] // the load the opener opens next
	openWake  chan struct{}        // a load waits in next; closed by Close
	openDone  chan struct{}        // closed as the opener ends
	releases  chan held            // to the releaser
	released  chan struct{}        // closed as the releaser ends
	releasing atomic.Int32         // loads the opener or the releaser have not done with
}

// durations keeps the last loadRing times of something. UI thread.
type durations struct {
	v [loadRing]time.Duration
	n int
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns the Android engine. cfg.Dispatch and cfg.Clock are required;
// sess, the app's audio session, is activated as the media starts playing
// (D13), and may be nil. Call it on the UI thread of a running app.
func New(cfg media.Config, sess media.Session) (media.Engine, error) {
	if cfg.Dispatch == nil || cfg.Clock == nil {
		return nil, fmt.Errorf("amp: new: no dispatch or clock: %w", core.ErrInvalid)
	}
	e := &engine{cfg: cfg, sess: sess, vol: 1, rate: 1}
	e.ticker = enginekit.NewTicker(cfg.Clock, enginekit.TickEvery, e.onTick)
	e.tickDo = e.doTick
	e.pauseDo, e.seekDo, e.volumeDo, e.seekPosDo = e.doPause, e.doSeek, e.doVolume, e.doSeekPos
	e.startDo, e.rateDo = e.doStart, e.doRate
	e.freeFn = e.freeImages
	err := jni.Do(func(env jni.Env) error {
		if err := lookup(env); err != nil {
			return err
		}
		var err error
		if e.attrsMusic, err = audioAttributes(env, contentMusic); err != nil {
			return err
		}
		if e.attrsMovie, err = audioAttributes(env, contentMovie); err != nil {
			return err
		}
		p, err := env.NewObject(ids.params, ids.paramsNew)
		if err != nil {
			return err
		}
		e.params = newGlobal(env, p)
		env.DeleteLocalRef(p)
		ok, err := jni.HasPermission(env, "android.permission.WAKE_LOCK")
		e.wake = err == nil && ok
		if e.wifi, err = newWifiLock(env, e.wake); err != nil {
			logf("amp: wifi lock: %v", err) // links play on without one
			e.wifi, _ = newWifiLock(env, false)
		}
		if e.handoff, err = newHandoff(env, e.wake); err != nil {
			logf("amp: hand-off lock: %v", err) // a queue may stall with the screen off
			e.handoff, _ = newHandoff(env, false)
		}

		return nil
	})
	if err != nil {
		e.freeRefs()

		return nil, fmt.Errorf("amp: new: %w", err)
	}
	e.prod, e.handle = newProducer(&e.video, cfg.Dispatch)
	e.releases, e.released = make(chan held, releaseQueue), make(chan struct{})
	e.openWake, e.openDone = make(chan struct{}, 1), make(chan struct{})
	go e.releaser()
	go e.opener()

	return e, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load drops the last load and starts preparing src, and returns at
// once: the opener makes the player (R3), and Play, Pause, Seek and the
// volume wait for it as they wait for onPrepared. A local file that is not
// there is media.ErrMissingFile, at once; a source that fails to open is
// an EventError, as are links of plain http while the app keeps from
// cleartext (Q1). A content:// document (a linked file) is local, with no
// path to look at first: one that fails to open is an EventError of
// media.ErrMissingFile or media.ErrUnreachable when its check says so
// (contentError).
func (e *engine) Load(_ context.Context, src media.Source, opts media.LoadOptions) (uint64, error) {
	if e.closed {
		return 0, errClosed
	}
	t0 := time.Now()
	e.unload()
	e.unloads.add(time.Since(t0))
	t0 = time.Now()
	e.token++
	e.video.SetRotation(0)
	e.video.Reset()
	e.keepImages(src.AudioOnly)
	l := e.newLoad(src, opts)
	e.cur = l
	e.wifi.set(!l.local && opts.Auto)
	if l.local {
		if err := enginekit.OpenError(enginekit.PhaseOpening, true, src.URL, nil); err != nil {
			l.state = enginekit.MPError
			e.loads.add(time.Since(t0))

			return e.token, err
		}
	}
	l.opening = true
	e.releasing.Add(1)
	if old := e.next.Swap(l); old != nil {
		e.releasing.Add(-1) // gone before the opener took it
	}
	select {
	case e.openWake <- struct{}{}:
	default: // the opener is told already
	}
	e.loads.add(time.Since(t0))

	return e.token, nil
}

// Play starts or resumes playing; before the player is prepared it plays
// once it is, and at the end it plays from the start (MediaPlayer does).
func (e *engine) Play() error {
	l := e.cur
	if l == nil {
		return errNotLoaded
	}
	switch l.waitState() {
	case enginekit.MPInitialized, enginekit.MPPreparing:
		l.pending.Play = true
		e.wifi.set(!l.local)
		if e.sess != nil {
			if err := e.sess.Activate(); err != nil {
				logf("amp: %v", err)
			}
		}
	case enginekit.MPPrepared, enginekit.MPPaused, enginekit.MPCompleted:
		e.begin(l)
	case enginekit.MPStarted:
	case enginekit.MPError, enginekit.MPEnd, enginekit.MPIdle, enginekit.MPStopped, enginekit.MPUnknown:
		return errFailed
	default:
		return errFailed
	}

	return nil
}

// Pause pauses, keeping the position; before the player is prepared it
// stays paused once it is.
func (e *engine) Pause() error {
	l := e.cur
	if l == nil {
		return errNotLoaded
	}
	switch l.waitState() {
	case enginekit.MPInitialized, enginekit.MPPreparing:
		l.pending.Play = false
		e.wifi.set(false)
	case enginekit.MPStarted:
		if err := e.do(e.pauseDo, l); err != nil {
			return fmt.Errorf("amp: pause: %w", err)
		}
		e.wifi.set(false)
		e.ticker.Stop()
		e.video.SetActive(false)
		e.emit(l, media.Event{Kind: media.EventState, State: media.StatePaused})
	default:
	}

	return nil
}

// Stop unloads: nothing plays, the frames are freed, and the events of the
// load are stale.
func (e *engine) Stop() error {
	e.unload()
	e.wifi.set(false)
	e.handoff.release()
	e.token++
	e.keepImages(false)
	e.video.Free()

	return nil
}

// Seek moves to seconds, exactly (SEEK_CLOSEST), and tells the time at
// once; before the player is prepared it seeks once it is.
func (e *engine) Seek(seconds float64) error {
	l := e.cur
	if l == nil {
		return errNotLoaded
	}
	if l.src.Live {
		return nil
	}
	t := max(seconds, 0)
	if l.dur > 0 {
		t = min(t, l.dur)
	}
	switch l.waitState() {
	case enginekit.MPInitialized, enginekit.MPPreparing:
		l.pending.Seek, l.pending.HasSeek = t, true
	case enginekit.MPPrepared, enginekit.MPStarted, enginekit.MPPaused, enginekit.MPCompleted:
		l.seekTo = t
		e.opMs = int64(t * 1000)
		if err := e.do(e.seekDo, l); err != nil {
			return fmt.Errorf("amp: seek: %w", err)
		}
		l.seeks++
		e.emit(l, media.Event{Kind: media.EventTime, Time: t})
	default:
		return errFailed
	}

	return nil
}

// SetVolume sets the volume, 0 to 1.
func (e *engine) SetVolume(v float64) error {
	e.vol = min(max(v, 0), 1)
	e.volumeChanged()

	return nil
}

// SetMuted mutes or unmutes: the player's volume is 0 while muted.
func (e *engine) SetMuted(on bool) error {
	e.muted = on
	e.volumeChanged()

	return nil
}

// SetRate sets the speed. A player that is not playing gets it as it
// starts: setPlaybackParams would start a paused one.
func (e *engine) SetRate(r float64) error {
	if r <= 0 {
		return fmt.Errorf("amp: rate %v: %w", r, core.ErrInvalid)
	}
	e.rate = r
	l := e.cur
	if l == nil || l.state != enginekit.MPStarted {
		return nil
	}
	if err := e.do(e.rateDo, l); err != nil {
		return fmt.Errorf("amp: rate: %w", err)
	}

	return nil
}

// ReadTime reads the position of the load token from its player now
// (media.TimeReader): in the background no tick tells it (F1 of M9). A
// seek under way tells the position it goes to; the end, the length.
// Kept within the length. It allocates nothing.
func (e *engine) ReadTime(token uint64) (float64, bool) {
	l := e.cur
	if e.closed || l == nil || l.token != token || l.opening || !l.prepared || l.mp == 0 ||
		!l.state.Allows(enginekit.CallGetCurrentPosition) {
		return 0, false
	}
	switch {
	case l.seeks > l.seeksDone:
		return l.seekTo, true
	case l.state == enginekit.MPCompleted && l.dur > 0:
		return l.dur, true // what onCompletion told
	default:
	}
	if err := jni.Do(e.tickDo); err != nil {
		return 0, false
	}
	t := max(float64(e.pos)/1000, 0)
	if l.dur > 0 {
		t = min(t, l.dur)
	}

	return t, true
}

// Events returns the topic the engine publishes to, on the UI thread.
func (e *engine) Events() *event.Topic[media.Event] { return &e.events }

// Video returns the surface the frames of a video go to.
func (e *engine) Video() *media.Surface { return &e.video }

// Close stops and releases the engine, and waits for the opener to end
// and the releaser to free the last load. The audio session lives on.
func (e *engine) Close() error {
	if e.closed {
		return nil
	}
	_ = e.Stop()
	e.closed = true
	close(e.openWake)
	<-e.openDone // what it opened last is the releaser's now
	close(e.releases)
	<-e.released
	e.freeRefs()
	e.handle.Delete()

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// begin starts a prepared, paused or finished player, at the speed asked,
// with the audio session active, and starts the tick.
func (e *engine) begin(l *load) {
	if e.sess != nil {
		if err := e.sess.Activate(); err != nil {
			logf("amp: %v", err) // the prototype plays on when play() is refused
		}
	}
	if e.cur != l {
		return
	}
	if err := e.do(e.startDo, l); err != nil {
		logf("amp: start: %v", err)

		return
	}
	e.wifi.set(!l.local)
	e.handoff.release()
	e.ticker.Start()
	e.video.SetActive(!l.src.AudioOnly)
	e.emit(l, media.Event{Kind: media.EventState, State: media.StatePlaying})
}

// keepImages arranges what becomes of the video's images: an audio load
// frees them after freeDelay, unless a video comes first; any other
// load, or Stop, cancels that.
func (e *engine) keepImages(audio bool) {
	if e.freeT != 0 {
		e.cfg.Clock.Stop(e.freeT)
		e.freeT = 0
	}
	if audio {
		e.freeT = e.cfg.Clock.After(freeDelay, e.freeFn)
	}
}

// freeImages frees the video's images while audio, or nothing, is loaded:
// no reader draws into them then.
func (e *engine) freeImages() {
	e.freeT = 0
	if l := e.cur; l == nil || l.src.AudioOnly {
		e.video.Free()
	}
}

// volumeChanged hands the volume to the current player.
func (e *engine) volumeChanged() {
	l := e.cur
	if l == nil || l.mp == 0 {
		return
	}
	if err := e.do(e.volumeDo, l); err != nil {
		logf("amp: volume: %v", err)
	}
}

// do runs fn, one of the calls made once, on l: it sets op for the call
// and clears it after, so that the engine keeps no load gone.
func (e *engine) do(fn func(jni.Env) error, l *load) error {
	e.op = l
	err := jni.Do(fn)
	e.op = nil

	return err
}

// doPause pauses op's player: Pause's call into Java.
func (e *engine) doPause(env jni.Env) error {
	return e.call(env, e.op, enginekit.CallPause, ids.pause)
}

// doSeek seeks op's player to opMs, exactly: Seek's call into Java.
func (e *engine) doSeek(env jni.Env) error {
	return e.call(env, e.op, enginekit.CallSeekTo, ids.seekTo, jni.Long(e.opMs), jni.Int(seekClosest))
}

// doVolume hands the volume to op's player: volumeChanged's call into Java.
func (e *engine) doVolume(env jni.Env) error {
	e.applyVolume(env, e.op)

	return nil
}

// doStart starts op's player at the speed asked: begin's call into Java.
func (e *engine) doStart(env jni.Env) error {
	if err := e.call(env, e.op, enginekit.CallStart, ids.start); err != nil {
		return err
	}
	if err := e.applyRate(env, e.op); err != nil {
		logf("amp: rate: %v", err)
	}

	return nil
}

// doRate sets the speed of op's player: SetRate's call into Java.
func (e *engine) doRate(env jni.Env) error { return e.applyRate(env, e.op) }

// doSeekPos reads the position of op's player into opPos:
// onSeekComplete's call into Java.
func (e *engine) doSeekPos(env jni.Env) error {
	var err error
	e.opPos, err = env.CallInt(e.op.mp, ids.getCurrentPosition)

	return err
}

// onTick tells the position of a playing player; while a seek is under
// way, the position it goes to, as ReadTime does (the player may tell
// the one it left, or, as it makes its video decoder again back from the
// background, the key frame it decodes from). It allocates nothing.
func (e *engine) onTick() {
	l := e.cur
	if l == nil || !l.prepared || l.mp == 0 || !l.state.Allows(enginekit.CallGetCurrentPosition) {
		return
	}
	if l.seeks > l.seeksDone {
		e.emit(l, media.Event{Kind: media.EventTime, Time: l.seekTo})

		return
	}
	if err := jni.Do(e.tickDo); err != nil {
		return
	}
	e.emit(l, media.Event{Kind: media.EventTime, Time: float64(e.pos) / 1000})
}

// doTick reads the position into e.pos: the tick's call into Java.
func (e *engine) doTick(env jni.Env) error {
	var err error
	e.pos, err = env.CallInt(e.cur.mp, ids.getCurrentPosition)

	return err
}

// emit publishes ev, with l's token, while l is the current load, and
// tells whether it still is after (a listener may have loaded another).
func (e *engine) emit(l *load, ev media.Event) bool {
	if e.cur != l {
		return false
	}
	ev.Token = l.token
	e.events.Publish(ev)

	return e.cur == l
}

// freeRefs deletes the engine's own global references.
func (e *engine) freeRefs() {
	_ = jni.Do(func(env jni.Env) error {
		for _, o := range []*jni.Object{&e.attrsMusic, &e.attrsMovie, &e.params} {
			deleteGlobal(env, *o)
			*o = 0
		}
		if e.wifi != nil {
			e.wifi.close(env)
		}
		if e.handoff != nil {
			e.handoff.close(env)
		}

		return nil
	})
}

// audioAttributes returns, as a global reference, the AudioAttributes of
// media (USAGE_MEDIA) of a content type.
func audioAttributes(env jni.Env, content int32) (jni.Object, error) {
	b, err := env.NewObject(ids.attrsBuilder, ids.attrsBuilderNew)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(b)
	for _, s := range []struct {
		m jni.MethodID
		v int32
	}{{ids.setUsage, usageMedia}, {ids.setContentType, content}} {
		r, err := env.CallObject(b, s.m, jni.Int(s.v))
		if err != nil {
			return 0, err
		}
		env.DeleteLocalRef(r) // the builder itself, again
	}
	a, err := env.CallObject(b, ids.attrsBuild)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(a)

	return newGlobal(env, a), nil
}

// add keeps one more time.
func (d *durations) add(t time.Duration) {
	d.v[d.n%loadRing] = t
	d.n++
}

// list returns the times kept.
func (d *durations) list() []time.Duration {
	n := min(d.n, loadRing)

	return append([]time.Duration(nil), d.v[:n]...)
}
