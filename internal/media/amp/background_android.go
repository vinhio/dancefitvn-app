// Adapted from jplayer-mobile/internal/media/amp/background_android.go at 49de3c2.
// Changed: hasAudioTrack takes a media that lists no track (HLS) to have
// sound.

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

// SetBackground follows the app's lifecycle (media.Backgrounder): in the
// background the tick stops, the frames are not copied, the video's
// images are freed (F3 of M11.P), and a video with sound gives its
// Surface up (setSurface(null)), so that the player decodes the sound
// only: NuPlayer shuts its video decoder down, and the sound plays on
// (F2 of M11.P: the decoder of a 720p60 video cost 8.8 % of a core in the
// codec service and 3.8 % in the media server with the screen off). A
// video without sound keeps it: without an audio track NuPlayer's clock
// stops with the video decoder (M4.2a), and so would the media. Back in
// the foreground the Surface is given again and the player seeks where
// it is, exactly (the decoder starts again from the key frame before,
// where NuPlayer itself would go back to), and the engine tells the
// time at once; the frames come as the seek ends, into images made again.
// Samsung's NuPlayer seeks on its own to a key frame then, at times after
// that seek: the engine seeks again (attach, reseek).
//
// A live stream (Source.Live, a channel of VTV Online) gives its Surface
// up too, whatever getTrackInfo tells (an HLS master without
// EXT-X-MEDIA lists no audio track; ON.E2): its sound keeps the clock.
// It does not take it back: given its Surface again, MediaPlayer shows
// no picture on API 36 (local/online/spike.md §5.1) and cannot seek a
// stream without a length. It plays on without its video, and LiveStale
// tells the app to load it again (media.Rejoiner).
func (e *engine) SetBackground(on bool) {
	if e.back == on {
		return
	}
	e.back = on
	e.prod.back.Store(on)
	e.ticker.SetBackground(on)
	l := e.cur
	if on {
		e.detach(l)
		e.dropImages(l)

		return
	}
	if e.attach(l) {
		return
	}
	e.onTick()
}

// LiveStale reports whether the live stream loaded gave its Surface up in
// the background (media.Rejoiner): it plays its sound only until it is
// loaded again. UI thread.
func (e *engine) LiveStale() bool {
	l := e.cur

	return l != nil && l.src.Live && l.detached
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// detachable tells whether l's player may play on without its Surface: a
// video handed over and prepared, in a state that takes setSurface, with
// a sound to keep the clock going and a seek to take the Surface back
// with; or a live stream, which has its sound (getTrackInfo may not list
// it: HLS) and is loaded again rather than given its Surface back.
func detachable(l *load) bool {
	if l == nil || l.reader == nil || l.opening || !l.prepared || l.mp == 0 || !l.state.Allows(enginekit.CallSetSurface) {
		return false
	}

	return l.src.Live || (l.hasAudio && l.state.Allows(enginekit.CallSeekTo))
}

// detach takes the Surface away from l's player, if it may play on
// without it. UI thread.
func (e *engine) detach(l *load) {
	if !detachable(l) || l.detached {
		return
	}
	err := jni.Do(func(env jni.Env) error {
		return e.call(env, l, enginekit.CallSetSurface, ids.setSurface, jni.Ref(0))
	})
	if err != nil {
		logf("amp: setSurface(null): %v", err)

		return
	}
	l.detached = true
	l.reattach.Disarm()
	e.detaches++
}

// attach gives l's player its Surface again and seeks to where it plays,
// exactly, and tells that time; it tells whether it did. A paused player
// shows the frame there; a playing one goes on from there. A live stream
// is left without it (SetBackground). Samsung's NuPlayer (Galaxy A17)
// seeks on its own to a key frame as it makes its video decoder again,
// told to nobody and at times after this seek: the seek's completion
// finds it elsewhere and seeks again (reseek). UI thread.
func (e *engine) attach(l *load) bool {
	if l == nil || !l.detached || e.cur != l || l.mp == 0 || l.reader == nil || l.src.Live ||
		!l.state.Allows(enginekit.CallSetSurface) {
		return false
	}
	l.detached = false
	var at float64
	err := jni.Do(func(env jni.Env) error {
		// The position before the Surface: NuPlayer's own seek, as it
		// makes the video decoder again, goes back to a key frame.
		ms, err := env.CallInt(l.mp, ids.getCurrentPosition)
		if err != nil {
			return err
		}
		if err := e.call(env, l, enginekit.CallSetSurface, ids.setSurface, jni.Ref(l.reader.surface)); err != nil {
			return err
		}
		at = max(float64(ms)/1000, 0)
		if l.dur > 0 {
			at = min(at, l.dur)
		}
		if err := e.call(env, l, enginekit.CallSeekTo, ids.seekTo, jni.Long(int64(at*1000)), jni.Int(seekClosest)); err != nil {
			return err
		}
		l.seekTo = at
		l.seeks++
		rate := 0.0
		if l.state == enginekit.MPStarted {
			rate = l.rate
		}
		l.reattach.Arm(l.seeks, at, rate, e.cfg.Clock.Now())

		return nil
	})
	e.attaches++
	if err != nil {
		logf("amp: Surface again: %v", err)

		return false
	}
	e.emit(l, media.Event{Kind: media.EventTime, Time: at})

	return true
}

// reseek seeks l's player again to at, exactly, if the completion of the
// seek attach gave it finds it at pos, away from at: its own seek to a key
// frame came after (enginekit.ReattachSeek). It tells whether it seeks:
// the time then stays at's until it is done. UI thread.
func (e *engine) reseek(l *load, pos float64) bool {
	at, again := l.reattach.Check(l.seeksDone, l.seeks, pos, e.cfg.Clock.Now())
	if !again {
		return false
	}
	logf("amp: Surface again: the player went to %.3f s; seeking %.3f s again", pos, at)
	l.seekTo = at
	e.opMs = int64(at * 1000)
	if err := e.do(e.seekDo, l); err != nil {
		logf("amp: seek again: %v", err)

		return false
	}
	l.seeks++
	e.reseeks++

	return true
}

// dropImages frees the video's images, once the reader of l, if any,
// has handed over the frame it may be converting: the producer, told the
// background before, converts none after. They are made again with the
// first frame back. UI thread.
func (e *engine) dropImages(l *load) {
	if l != nil {
		l.reader.sync()
	}
	e.video.Free()
}

// hasAudioTrack tells whether the media of mp, prepared, has a sound
// track (MediaPlayer.getTrackInfo). A media that lists no track at all
// is taken to have one: NuPlayer's HLS source lists none for a master
// without EXT-X-MEDIA, its sound muxed in the segments (the lessons' HLS:
// 0 tracks on a Galaxy A17, API 36), and the lessons have music; kept,
// their Surface would have the video decoder run on in the background
// (48 frames decoded and dropped in 2 s, 5.3 % of a core).
func hasAudioTrack(env jni.Env, mp jni.Object) bool {
	arr, err := env.CallObject(mp, ids.getTrackInfo)
	if err != nil || arr == 0 {
		return false
	}
	defer env.DeleteLocalRef(arr)
	n := env.ArrayLength(arr)
	if n == 0 {
		return true
	}
	for i := range n {
		t, err := env.Element(arr, i)
		if err != nil || t == 0 {
			continue
		}
		kind, err := env.CallInt(t, ids.trackType)
		env.DeleteLocalRef(t)
		if err == nil && kind == trackAudio {
			return true
		}
	}

	return false
}
