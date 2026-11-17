// Adapted from jplayer-mobile/internal/media/avf/testhooks_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tick runs the time tick of e, an engine of this package, now, as its
// ticker does every 250 ms while it plays; false for another engine. For
// device tests only (the tick's allocations, the time after a seek); UI
// thread.
func Tick(e media.Engine) bool {
	en, ok := e.(*engine)
	if ok {
		en.tick()
	}

	return ok
}

// SetLateOutput makes the next loads of e, an engine of this package, give
// a video its output a tick after it is ready rather than then, as for a
// stream whose size is not known then; false for another engine. For
// device tests only (a late output, playing or paused); UI thread.
func SetLateOutput(e media.Engine, on bool) bool {
	en, ok := e.(*engine)
	if ok {
		en.lateOutput = on
	}

	return ok
}

// SetKeepVideoInBackground keeps the video tracks of e, an engine of this
// package, enabled in the background, as before D4 of M4.5; false for
// another engine. For device measures only (the CPU of a video in the
// background, with and without its tracks); UI thread.
func SetKeepVideoInBackground(e media.Engine, on bool) bool {
	en, ok := e.(*engine)
	if ok {
		en.keepTracks = on
	}

	return ok
}

// VideoTracks reads, from AVFoundation, how many video tracks the current
// item of e has and how many of them are enabled; false for another
// engine or nothing loaded. For device tests only; UI thread.
func VideoTracks(e media.Engine) (total, enabled int, ok bool) {
	en, isAVF := e.(*engine)
	if !isAVF || en.cur == nil || en.cur.item == 0 {
		return 0, 0, false
	}
	objc.WithPool(func() {
		tracks := objc.Call(en.cur.item, av.sel.tracks)
		for i := range int(objc.Call(tracks, av.sel.count)) {
			track := objc.Call(tracks, av.sel.objectAtIndex, uintptr(i))
			if videoAsset(track) == 0 {
				continue
			}
			total++
			if objc.CallBool(track, av.sel.isEnabled) {
				enabled++
			}
		}
	})

	return total, enabled, true
}

// SetVariantSizing makes the next loads of e, an engine of this package,
// size the output of an HLS stream for its largest variant (on, as by
// default) or for the frames it presents first (off), so that the output
// is made again as the stream switches up (followSize); false for another
// engine. For device tests only (the fallback of followSize); UI thread.
func SetVariantSizing(e media.Engine, on bool) bool {
	en, ok := e.(*engine)
	if ok {
		en.noVariants = !on
	}

	return ok
}

// SetNextStarved keeps the pull of e, an engine of this package, from
// taking the frames of a next output of another size (on), as if it got
// none, so that followSize makes it again and gives up after nextRetries;
// false for another engine. For device tests only (the limit of
// followSize); UI thread.
func SetNextStarved(e media.Engine, on bool) bool {
	en, ok := e.(*engine)
	if ok {
		en.starveNext = on
	}

	return ok
}

// SetPeakBitRate sets the preferredPeakBitRate of the current item of e, an
// engine of this package (AVPlayerItem.h:539; 0 lifts the cap), so that an
// HLS stream starts on its smallest variant and switches up when lifted,
// whatever the player measured of the network before; false for another
// engine, nothing loaded, or an error. For device tests only; UI thread.
func SetPeakBitRate(e media.Engine, bps float64) bool {
	en, ok := e.(*engine)
	if !ok || en.cur == nil || en.cur.item == 0 {
		return false
	}
	in, err := objc.NewInvocation(objc.GetClass("AVPlayerItem"), "setPreferredPeakBitRate:")
	if err != nil {
		return false
	}
	defer in.Release()

	return in.SetArg(0, bps) == nil && in.Invoke(en.cur.item) == nil
}

// PeakBitRate returns the preferredPeakBitRate of the current item of e,
// an engine of this package (AVPlayerItem.h:539; 0 is no cap); false for
// another engine, nothing loaded, or an error. For device tests only; UI
// thread.
func PeakBitRate(e media.Engine) (float64, bool) {
	en, ok := e.(*engine)
	if !ok || en.cur == nil || en.cur.item == 0 {
		return 0, false
	}
	in, err := objc.NewInvocation(objc.GetClass("AVPlayerItem"), "preferredPeakBitRate")
	if err != nil {
		return 0, false
	}
	defer in.Release()
	if in.Invoke(en.cur.item) != nil {
		return 0, false
	}
	v, err := in.Result[float64]()

	return v, err == nil
}

// PlayerRate reads, from AVFoundation, the rate and the defaultRate of the
// player of e, an engine of this package (AVPlayer.h:152, :163): the
// speed it plays at now (0 paused) and the one play uses; false for
// another engine or an error. For device measures only (04 §2.2: play
// keeps the speed chosen); UI thread.
func PlayerRate(e media.Engine) (rate, defaultRate float64, ok bool) {
	en, isAVF := e.(*engine)
	if !isAVF || en.closed {
		return 0, 0, false
	}
	for _, x := range []struct {
		sel string
		dst *float64
	}{{"rate", &rate}, {"defaultRate", &defaultRate}} {
		in, err := objc.NewInvocation(objc.GetClass("AVPlayer"), x.sel)
		if err != nil {
			return 0, 0, false
		}
		if in.Invoke(en.player) != nil {
			in.Release()

			return 0, 0, false
		}
		v, err := in.Result[float32]()
		in.Release()
		if err != nil {
			return 0, 0, false
		}
		*x.dst = float64(v)
	}

	return rate, defaultRate, true
}

// PitchAlgorithm reads the audioTimePitchAlgorithm of the current item of
// e, an engine of this package (04 §2.2: TimeDomain); false for another
// engine or nothing loaded. For device measures only; UI thread.
func PitchAlgorithm(e media.Engine) (string, bool) {
	en, ok := e.(*engine)
	if !ok || en.cur == nil || en.cur.item == 0 {
		return "", false
	}
	var s string
	objc.WithPool(func() { s = objc.GoString(objc.Call(en.cur.item, av.sel.pitchAlgorithm)) })

	return s, true
}

// ErrorLog reads the error log of the item of e, an engine of this package
// (AVPlayerItem.errorLog, AVPlayerItem.h:1035): how many entries it has,
// and the domain, status code and comment of the last; false for another
// engine or nothing loaded. For device tests only; UI thread.
func ErrorLog(e media.Engine) (n int, domain string, code int, comment string, ok bool) {
	en, isAVF := e.(*engine)
	if !isAVF || en.closed || en.cur == nil || en.cur.item == 0 {
		return 0, "", 0, "", false
	}
	objc.WithPool(func() {
		log := objc.Call(en.cur.item, objc.Sel("errorLog"))
		if log == 0 {
			return
		}
		events := objc.Call(log, objc.Sel("events"))
		n = int(objc.Call(events, av.sel.count))
		last := objc.Call(events, objc.Sel("lastObject"))
		if last == 0 {
			return
		}
		domain = objc.GoString(objc.Call(last, objc.Sel("errorDomain")))
		code = int(objc.Call(last, objc.Sel("errorStatusCode")))
		comment = objc.GoString(objc.Call(last, objc.Sel("errorComment")))
	})

	return n, domain, code, comment, true
}
