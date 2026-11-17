// Adapted from jplayer-mobile/internal/media/amp/listen_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// listen is the function of the proxy of l's listeners. It runs on Java's
// main thread (the player is made on the opener's thread, which has no
// Looper: countThread counts a call anywhere else), and may run after the
// load is gone, or before the opener's result reached the UI thread
// (applyOpen replays those): it only reads its
// arguments, writes atomics, takes the hand-off lock at a completion,
// and hands l's work to the UI thread, which
// drops the work of a load that is not the current one. The frequent
// events (buffering, sizes, seeks) hand over a func made at the load.
func (l *load) listen(env jni.Env, method int, args jni.Args) jni.Value {
	countThread()
	dispatch := l.e.cfg.Dispatch
	switch method {
	case onPrepared:
		dispatch(l.preparedFn)
	case onCompletion:
		// MediaPlayer let go of its wake lock: the CPU stays awake for
		// the next media (handoff).
		l.e.handoff.take(env)
		dispatch(l.completedFn)
	case onError: // onError(MediaPlayer, what, extra)
		what, extra := args.Int(1), args.Int(2)
		dispatch(func() { l.e.onError(l, what, extra) })

		return jni.Bool(true) // handled: no onCompletion after it
	case onInfo: // onInfo(MediaPlayer, what, extra)
		if what := args.Int(1); what == infoBufferStart || what == infoBufferEnd {
			dispatch(func() { l.e.onInfo(l, what) })
		}

		return jni.Bool(true)
	case onBufferingUpdate: // onBufferingUpdate(MediaPlayer, percent)
		if p := args.Int(1); l.buffered.Swap(p) != p {
			dispatch(l.bufferedFn)
		}
	case onSeekComplete:
		dispatch(l.seekedFn)
	case onVideoSizeChanged: // onVideoSizeChanged(MediaPlayer, width, height)
		l.size.Store(packSize(args.Int(1), args.Int(2)))
		dispatch(l.sizeFn)
	default:
	}

	return 0
}

// onPrepared applies what waited for the player (the volume, the start
// position, Play) and tells the length and the time. A video that does not
// play at once seeks to its position anyway, which has the player decode
// and show the frame there, as a browser shows the first frame.
func (e *engine) onPrepared(l *load) {
	if e.cur != l {
		return
	}
	if l.opening {
		l.earlyPrepared = true

		return
	}
	if l.state != enginekit.MPPreparing {
		return
	}
	l.state = l.state.On(enginekit.EvPrepared)
	l.prepared = true
	if l.cancel != nil {
		l.cancel() // the player answered: the preflight has nothing to tell
		l.cancel = nil
	}
	if !l.local && !l.src.AudioOnly {
		e.readRotation(l)
	}
	at := 0.0
	err := jni.Do(func(env jni.Env) error {
		if d, err := env.CallInt(l.mp, ids.getDuration); err == nil && d > 0 && !l.src.Live {
			l.dur = float64(d) / 1000
		}
		e.applyVolume(env, l)
		if l.reader != nil && l.state.Allows(enginekit.CallGetTrackInfo) {
			l.hasAudio = hasAudioTrack(env, l.mp)
			if l.src.Live {
				// What getTrackInfo lists for HLS (ON.E2): the background
				// does not depend on it for a live stream.
				logf("amp: live stream prepared, audio track listed %t", l.hasAudio)
			}
		}
		seek := l.pending.HasSeek
		if seek {
			at = l.pending.Seek
			if l.dur > 0 {
				at = min(at, l.dur)
			}
		}
		l.pending.HasSeek = false
		if !seek && !l.pending.Play && l.reader != nil && !l.src.Live {
			seek = true // the preview frame
		}
		if !seek {
			return nil
		}
		l.seekTo = at
		if err := e.call(env, l, enginekit.CallSeekTo, ids.seekTo, jni.Long(int64(at*1000)), jni.Int(seekClosest)); err != nil {
			return err
		}
		l.seeks++

		return nil
	})
	if err != nil {
		logf("amp: prepared: %v", err)
	}
	if e.back {
		e.detach(l) // a load made in the background decodes its sound only
	}
	seeks := l.seeks
	if l.dur > 0 && !e.emit(l, media.Event{Kind: media.EventDuration, Duration: l.dur}) {
		return
	}
	if l.seeks != seeks {
		// A Seek made in answer to EventDuration (the player drops a resume
		// position past the media's end) went after the start's: the time
		// is where it went, not the start (P4.9 D2).
		at = l.seekTo
	}
	if !e.emit(l, media.Event{Kind: media.EventTime, Time: at}) {
		return
	}
	if l.pending.Play {
		l.pending.Play = false
		e.begin(l)

		return
	}
	e.emit(l, media.Event{Kind: media.EventState, State: media.StatePaused})
}

// onCompletion ends a media that played to its end: the last time, then
// Ended, and no Paused (the prototype ignores pause once ended,
// player.js:52).
func (e *engine) onCompletion(l *load) {
	if e.cur != l || l.state != enginekit.MPStarted {
		return
	}
	l.state = l.state.On(enginekit.EvCompleted)
	e.wifi.set(false)
	e.ticker.Stop()
	e.video.SetActive(false)
	if !e.emit(l, media.Event{Kind: media.EventTime, Time: l.dur}) {
		return
	}
	e.emit(l, media.Event{Kind: media.EventEnded})
}

// onError reports a failure as the phase of the load tells it (D12): a
// source that cannot be opened before onPrepared, a network or decoding
// failure after.
func (e *engine) onError(l *load, what, extra int32) {
	if what == errInvalidOp {
		e.wrongState++
	}
	if e.cur != l {
		return
	}
	if l.opening {
		if !l.earlyErr {
			l.earlyErr, l.earlyWhat, l.earlyExtra = true, what, extra
		}

		return
	}
	phase := enginekit.PhasePlaying
	if !l.prepared {
		phase = enginekit.PhaseOpening
	}
	l.state = l.state.On(enginekit.EvError)
	e.wifi.set(false)
	e.ticker.Stop()
	e.video.SetActive(false)
	e.emit(l, media.Event{Kind: media.EventError, Err: enginekit.AndroidError(phase, what, extra)})
}

// onInfo turns buffering into Loading and back into Playing.
func (e *engine) onInfo(l *load, what int32) {
	if e.cur != l || l.state != enginekit.MPStarted {
		return
	}
	s := media.StatePlaying
	if what == infoBufferStart {
		s = media.StateLoading
	}
	e.emit(l, media.Event{Kind: media.EventState, State: s})
}

// onBuffered tells how far the media is loaded: its percent of the length.
func (e *engine) onBuffered(l *load) {
	if e.cur != l || l.dur <= 0 {
		return
	}
	p := l.buffered.Load()
	if p < 0 {
		return
	}
	e.emit(l, media.Event{Kind: media.EventBuffered, Buffered: float64(min(p, 100)) / 100 * l.dur})
}

// onVideoSize tells the size of the picture, as shown (a rotated video's
// size is turned already).
func (e *engine) onVideoSize(l *load) {
	if e.cur != l {
		return
	}
	w, h := unpackSize(l.size.Load())
	if w <= 0 || h <= 0 || (w == l.vw && h == l.vh) {
		return
	}
	l.vw, l.vh = w, h
	e.emit(l, media.Event{Kind: media.EventVideoSize, Width: w, Height: h})
}

// onSeekComplete tells the time a seek reached, unless another seek
// followed it (the player reports every seek it finishes, the ones it
// skipped too).
func (e *engine) onSeekComplete(l *load) {
	l.seeksDone++
	if e.cur != l || !l.state.Allows(enginekit.CallGetCurrentPosition) || l.state == enginekit.MPIdle {
		return
	}
	if err := e.do(e.seekPosDo, l); err != nil {
		return
	}
	t := float64(e.opPos) / 1000
	if e.reseek(l, t) {
		return
	}
	if d := t - l.seekTo; d > seekSlack || d < -seekSlack {
		return
	}
	e.emit(l, media.Event{Kind: media.EventTime, Time: t})
}

// seekSlack is how far from the last seek asked a finished seek may land
// and still be that one, in seconds.
const seekSlack = 1.0
