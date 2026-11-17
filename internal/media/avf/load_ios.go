// Adapted from jplayer-mobile/internal/media/avf/load_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"log"
	"math"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// load is one Load of an engine: its asset and item, the observer that
// hears about them, and, for a video, the output frames are pulled from.
// UI thread only.
type load struct {
	e     *engine
	token uint64
	src   media.Source
	local bool

	asset, item, output, observer objc.ID // owned

	ready, failed, ended bool
	// starting is onReady publishing the length: a Seek made in answer to
	// EventDuration (the player drops a resume position past the media's
	// end) is where the load starts, as one made before ready is.
	starting bool
	// autoRun plays once ready (Auto, or Play before ready).
	autoRun bool
	// seekTo is where to seek once ready, when hasSeek.
	seekTo  float64
	hasSeek bool
	// control is the timeControlStatus last handled, -1 before any.
	control int
	// state is the state last published.
	state media.PlayState

	dur, buffered float64
	// fw, fh is the size of the output's frames; rot their turn.
	fw, fh, rot int
	// shown is the item's presentationSize when the output was last made:
	// the tick makes another output when it changes, as an HLS stream
	// switches to a variant of another size (ABR, followSize).
	shown cgSize
	// next is the output of the new size (owned), until it has a frame and
	// replaces output (promote); nfw, nfh, nrot its frames' size and turn,
	// nw, nh the video's size (for the log); nextFrames the frames pulled
	// when it was made; retries how many times it was made again for the
	// size shown, as one got no frame (back to 0 when the size changes).
	next           objc.ID
	nfw, nfh, nrot int
	nw, nh         int
	nextFrames     int
	retries        int
	// videoOff is how many video tracks of the item are disabled, in the
	// background (setVideoTracks).
	videoOff int
	// capped says the item's bit rate is capped at the source's
	// BackgroundPeak, in the background (capPeak).
	capped bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// open makes l's asset and item, puts the item in the player, and starts
// observing them. In a pool.
func (e *engine) open(l *load) error {
	str := objc.NSString(l.src.URL)
	defer objc.Release(str)
	sel := av.sel.urlWithString
	if l.local {
		sel = av.sel.fileURLWithPath
	}
	u := objc.Call(av.cls.url, sel, uintptr(str))
	if u == 0 {
		return fmt.Errorf("%w: %q", errNotURL, l.src.URL)
	}
	asset := objc.Call(av.cls.asset, av.sel.assetWithURL, uintptr(u), 0)
	if asset == 0 {
		return fmt.Errorf("avf: no asset for %q: %w", l.src.URL, media.ErrSource)
	}
	item := objc.Call(av.cls.item, av.sel.itemWithAsset, uintptr(asset), uintptr(av.assetKeys))
	if item == 0 {
		return fmt.Errorf("avf: no item for %q: %w", l.src.URL, media.ErrSource)
	}
	l.asset, l.item = objc.Retain(asset), objc.Retain(item)
	// The pitch kept at any speed (04 §2.2): TimeDomain suits speech and
	// music slowed to 0.5 or sped to 1.5.
	objc.Call(item, av.sel.setPitchAlgorithm, uintptr(av.pitchTimeDomain))
	if e.maxRes != (cgSize{}) && !e.applyMaxResolution(item) {
		log.Printf("dancefitvn: avf: the item takes no maximum resolution")
	}
	e.liveItems++
	l.observer = objc.New(av.cls.observer)
	watchLoad(l.observer, l)
	if e.back { // a load made in the background starts capped
		e.capPeak(l)
	}
	objc.Call(e.player, av.sel.replaceItem, uintptr(item))
	objc.Call(item, av.sel.addKVO, uintptr(l.observer), uintptr(av.keyStatus), kvoOptionNew, 0)
	objc.Call(e.player, av.sel.addKVO, uintptr(l.observer), uintptr(av.keyControl), kvoOptionNew, 0)
	c := center()
	objc.Call(c, av.sel.addObserver, uintptr(l.observer), uintptr(av.sel.ended), uintptr(av.didPlayToEnd), uintptr(item))
	objc.Call(c, av.sel.addObserver, uintptr(l.observer), uintptr(av.sel.failed), uintptr(av.failedToPlayToEnd), uintptr(item))
	// The status changes later, never during Load; should it have, it is
	// handled once Load has returned the token the events carry.
	if objc.Call(item, av.sel.status) != 0 {
		e.cfg.Dispatch(func() { e.sync(l) })
	}

	return nil
}

// unload stops observing the current load and releases it: the observer is
// removed before the objects it observes go (R5), and the player paused
// after, so that the old load reports nothing and the next item does not
// start by itself.
func (e *engine) unload() {
	l := e.cur
	e.cur = nil
	e.ticker.Stop()
	e.outputWait.Stop()
	if l == nil {
		return
	}
	objc.WithPool(func() {
		objc.Call(center(), av.sel.removeObserver, uintptr(l.observer))
		objc.Call(l.item, av.sel.removeKVO, uintptr(l.observer), uintptr(av.keyStatus))
		objc.Call(e.player, av.sel.removeKVO, uintptr(l.observer), uintptr(av.keyControl))
		unwatchLoad(l.observer)
		objc.Call(e.player, av.sel.pause)
		e.dropNext(l)
		if l.output != 0 {
			objc.Call(l.item, av.sel.removeOutput, uintptr(l.output))
			objc.Release(l.output)
			l.output = 0
		}
		objc.Call(l.asset, av.sel.cancelLoading)
		objc.Release(l.observer)
		objc.Release(l.item)
		objc.Release(l.asset)
	})
	l.observer, l.item, l.asset = 0, 0, 0
	e.liveItems--
}

// sync reads the item's status and the player's timeControlStatus again,
// on the UI thread, and acts on what changed: key-value observing told that
// one did.
func (e *engine) sync(l *load) {
	if e.cur != l || l.failed {
		return
	}
	if !l.ready {
		switch objc.Call(l.item, av.sel.status) {
		case itemStatusReady:
			e.onReady(l)
		case itemStatusFailed:
			var cause nsError
			objc.WithPool(func() {
				errObj := objc.Call(l.item, av.sel.errorOf)
				if errObj == 0 {
					errObj = objc.Call(e.player, av.sel.errorOf)
				}
				cause = describe(errObj)
			})
			e.fail(l, cause)

			return
		default:
		}
	}
	if !l.ready || e.cur != l || l.failed {
		return
	}
	e.onControl(l, int(objc.Call(e.player, av.sel.timeControlStatus)))
}

// onPlayFailed handles AVPlayerItemFailedToPlayToEndTimeNotification.
func (e *engine) onPlayFailed(l *load, cause nsError) {
	if e.cur != l || l.failed {
		return
	}
	e.fail(l, cause)
}

// onEnded handles AVPlayerItemDidPlayToEndTimeNotification: the last time,
// then Ended (player.js:54), and no pause between (player.js:52).
func (e *engine) onEnded(l *load) {
	if e.cur != l || !l.ready || l.failed || l.ended {
		return
	}
	l.ended = true
	e.ticker.Stop()
	e.settling = false
	e.video.SetActive(false)
	t := l.dur
	if p, ok := e.position(); ok && (t <= 0 || p < t) {
		t = p
	}
	e.publish(l, media.Event{Kind: media.EventTime, Time: t})
	e.publish(l, media.Event{Kind: media.EventEnded})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onReady handles AVPlayerItemStatusReadyToPlay: the length, the video's
// output and size, the start position, then playing or the paused state.
// The start is read after EventDuration: a Seek made in answer to it
// (starting) replaces the load's, so that the player's seek to 0 for a
// resume position past the media's end is not undone by a seek to the end
// (P4.9 D2). A Load or Stop made in answer ends here.
func (e *engine) onReady(l *load) {
	l.ready = true
	if d, ok := e.itemDuration(l); ok {
		l.dur = d
		l.starting = true
		e.publish(l, media.Event{Kind: media.EventDuration, Duration: d})
		l.starting = false
		if e.cur != l || l.failed {
			return
		}
	}
	if !l.src.AudioOnly && !e.lateOutput {
		objc.WithPool(func() { e.attachOutput(l) })
	}
	if !l.src.AudioOnly && l.output == 0 {
		// The size is not known yet (a stream): outputWait attaches the
		// output once it is, whether the media plays or not (M4.5 D3).
		e.outputWait.Start()
	}
	if e.back {
		// Ready in the background (the next of a queue): the sound only,
		// until the foreground.
		e.setVideoTracks(l, false)
	}
	start := 0.0
	if l.hasSeek {
		start = l.seekTo
		if l.dur > 0 {
			start = min(start, l.dur)
		}
	}
	// A seek, even to 0, makes the output decode the frame shown while
	// paused (the output joined after the item decoded its first frame).
	if start > 0 || (l.output != 0 && !l.autoRun) {
		e.seekTo(l, start)
	}
	e.publish(l, media.Event{Kind: media.EventTime, Time: start})
	if l.autoRun {
		if err := e.Play(); err != nil {
			log.Printf("dancefitvn: avf play: %v", err)
		}

		return
	}
	l.control = controlPaused
	e.setState(l, media.StatePaused)
	if l.output != 0 {
		e.settle()
	}
}

// onControl handles a timeControlStatus of the ready load l: Playing starts
// the tick and the pulls, Waiting is loading, Paused stops them, unless it
// is the stop at the end.
func (e *engine) onControl(l *load, c int) {
	if c == l.control {
		return
	}
	l.control = c
	switch c {
	case controlPlaying:
		l.ended = false
		e.setState(l, media.StatePlaying)
		e.ticker.Start()
		if l.output != 0 {
			e.settling = false
			e.activate()
		}
	case controlWaiting:
		e.setState(l, media.StateLoading)
	case controlPaused:
		e.ticker.Stop()
		if l.ended || e.atEnd(l) {
			e.video.SetActive(false)

			return
		}
		e.setState(l, media.StatePaused)
		if l.output != 0 {
			e.settle()
		}
	default:
	}
}

// atEnd tells that l's position is at its end.
func (e *engine) atEnd(l *load) bool {
	if l.dur <= 0 {
		return false
	}
	p, ok := e.position()

	return ok && p >= l.dur-endSlack
}

// fail reports the failure of l: media.ErrSource before it was ready, a
// network or decoding error after (D12), media.ErrMissingFile for a local
// file gone.
func (e *engine) fail(l *load, cause nsError) {
	l.failed = true
	e.ticker.Stop()
	e.outputWait.Stop()
	e.settling = false
	e.video.SetActive(false)
	phase := enginekit.PhasePlaying
	if !l.ready {
		phase = enginekit.PhaseOpening
	}
	domain := cause.domain
	if domain == "" {
		domain = "unknown"
	}
	err := enginekit.AppleError(phase, domain, cause.code, cause.underDomain, cause.underCode)
	if l.local {
		if gone := enginekit.OpenError(phase, true, l.src.URL, nil); gone != nil {
			err = gone
		}
	}
	log.Printf("dancefitvn: %v (%s)", err, cause.text)
	l.state = media.StateError
	e.publish(l, media.Event{Kind: media.EventError, Err: err})
}

// attachOutput gives a video item its output: BGRA frames, IOSurface-backed,
// of the video's size within enginekit.MaxEdge (04 §2.2), turned by the
// track's preferredTransform; then it tells the size shown. A video whose
// size is not known yet gets its output from outputWait. The output of an
// HLS stream has the size of its largest variant of the same shape
// (largestVariant), within enginekit.MaxEdge, from the start: the player
// starts on a small variant (240p, say) and switches up (ABR), and the
// output scales every frame to its size in hardware, so the frames are
// sharp after the switch at no cost. An output added later, in place of
// one of the first size (followSize), gets its frames converted as they
// are copied: 1.5 to 2.1 ms a frame rather than 0.1 to 0.2 at 1280 by
// 720, and 117 to 190 MiB rather than 114 (simulator, M19 YH.A). In a
// pool.
func (e *engine) attachOutput(l *load) {
	l.shown, _ = e.presentation(l)
	w, h, rot := e.videoGeometry(l)
	if w <= 0 || h <= 0 {
		return
	}
	fw, fh := enginekit.Fit(w, h, enginekit.MaxEdge)
	if !l.local && !e.noVariants && rot%180 == 0 {
		if vw, vh := e.largestVariant(l, w, h); vw*vh > w*h {
			fw, fh = enginekit.Fit(vw, vh, enginekit.MaxEdge)
		}
	}
	out, err := newOutput(fw, fh)
	if err != nil {
		log.Printf("dancefitvn: avf output: %v", err)

		return
	}
	objc.Call(l.item, av.sel.addOutput, uintptr(out))
	l.output, l.fw, l.fh, l.rot = out, fw, fh, rot
	e.video.SetRotation(rot)
	dw, dh := enginekit.Turned(w, h, rot)
	e.publish(l, media.Event{Kind: media.EventVideoSize, Width: dw, Height: dh})
}

// followSize keeps l's frames as sharp as the item presents them. An output
// scales every frame to the size it was made with, so an output made for a
// small variant of an HLS stream would keep every frame of a larger one at
// that size. When the item's presentationSize changes and the frames it
// presents need a larger output than l's, or one of another shape,
// another output of their size joins the item (nextOutput), and the pull
// puts it in the old one's place once it has a frame (promote). A smaller
// variant keeps the output: it scales the frames up. attachOutput makes
// the output large enough from the start when the stream tells its
// variants, so this is the fallback (a stream that does not tell them).
// An output that joins as the item switches variants may get no frame at
// all (seen on iOS 26.5 with YouTube's HLS): when the old output gave
// nextRetry frames since and the new one none, a third one replaces it, up
// to nextRetries times for one size: then followSize drops the next one,
// keeps the old one (its frames scaled to its size) and tries no more until
// the presentationSize changes again (another variant), or a new load. It
// tells the new size shown (EventVideoSize). The tick calls it, playing: it
// allocates nothing until the size changes or a next output gets no frame.
// In a pool.
func (e *engine) followSize(l *load) {
	if l.output == 0 {
		return
	}
	if s, ok := e.presentation(l); ok && s.W > 0 && s.H > 0 && s != l.shown {
		l.shown = s
		l.retries = 0
		e.publish(l, media.Event{Kind: media.EventVideoSize, Width: int(math.Round(s.W)), Height: int(math.Round(s.H))})
		e.nextOutput(l)

		return
	}
	if l.next == 0 || e.frames-l.nextFrames < nextRetry {
		return
	}
	if l.retries >= nextRetries {
		log.Printf("dancefitvn: avf output %dx%d got no frame in %d tries, keeping %dx%d until the size changes",
			l.nfw, l.nfh, nextRetries+1, l.fw, l.fh)
		e.dropNext(l)
		e.nextGaveUp++

		return
	}
	l.retries++
	log.Printf("dancefitvn: avf output %dx%d got no frame in %d, another one", l.nfw, l.nfh, nextRetry)
	e.nextOutput(l)
}

// nextOutput makes the output of the size the item presents now l's next
// one, in place of any next one before, when l's output is too small for
// those frames or of another shape (followSize); otherwise it only drops
// the next one. The size is the presentationSize (l.shown), which is
// turned by the track's preferredTransform (360 by 640 for a video of 640
// by 360 turned 90 degrees, iOS 26.5), turned back: the track's natural
// size may already be that of a variant to come. It allocates. In a pool.
func (e *engine) nextOutput(l *load) {
	e.dropNext(l)
	_, _, rot := e.videoGeometry(l)
	w, h := enginekit.Turned(int(math.Round(l.shown.W)), int(math.Round(l.shown.H)), rot)
	if w <= 0 || h <= 0 {
		return
	}
	fw, fh := enginekit.Fit(w, h, enginekit.MaxEdge)
	if rot == l.rot && sameShape(fw, fh, l.fw, l.fh) && fw <= l.fw && fh <= l.fh {
		return
	}
	out, err := newOutput(fw, fh)
	if err != nil {
		log.Printf("dancefitvn: avf output: %v", err)

		return
	}
	objc.Call(l.item, av.sel.addOutput, uintptr(out))
	l.next, l.nfw, l.nfh, l.nrot, l.nw, l.nh = out, fw, fh, rot, w, h
	l.nextFrames = e.frames
	e.nextMade++
}

// promote puts l's next output in the place of its output, which leaves
// the item. The pull calls it once the next output has a frame. In a pool.
func (e *engine) promote(l *load) {
	log.Printf("dancefitvn: avf output %dx%d -> %dx%d (video %dx%d)", l.fw, l.fh, l.nfw, l.nfh, l.nw, l.nh)
	objc.Call(l.item, av.sel.removeOutput, uintptr(l.output))
	objc.Release(l.output)
	l.output, l.fw, l.fh, l.rot = l.next, l.nfw, l.nfh, l.nrot
	l.next = 0
	e.outputs++
	e.video.SetRotation(l.rot)
}

// dropNext takes l's next output, if any, off the item and releases it.
func (e *engine) dropNext(l *load) {
	if l.next == 0 {
		return
	}
	objc.Call(l.item, av.sel.removeOutput, uintptr(l.next))
	objc.Release(l.next)
	l.next = 0
}

// largestVariant returns the largest presentationSize among the variants
// of l's asset (an HLS stream's, from its multivariant playlist; none for
// a file or a media playlist) whose shape is that of a w by h frame; 0, 0
// when none. The playlist is loaded once the item is ready, so reading
// the variants then does not wait (iOS 15, AVAsset.h:749). In a pool.
func (e *engine) largestVariant(l *load, w, h int) (vw, vh int) {
	vs := objc.Call(l.asset, av.sel.variants)
	if vs == 0 {
		return 0, 0
	}
	for i := range int(objc.Call(vs, av.sel.count)) {
		attrs := objc.Call(objc.Call(vs, av.sel.objectAtIndex, uintptr(i)), av.sel.videoAttributes)
		if attrs == 0 || e.inv.variantSize.Invoke(attrs) != nil {
			continue
		}
		s, err := e.inv.variantSize.Result[cgSize]()
		if err != nil {
			continue
		}
		sw, sh := int(math.Round(s.W)), int(math.Round(s.H))
		if sw*sh > vw*vh && sameShape(sw, sh, w, h) {
			vw, vh = sw, sh
		}
	}

	return vw, vh
}

// sameShape tells that a w by h frame and an ow by oh one have the same
// proportions, within the rounding of a codec's even sides (1.5 %, so
// 426 by 240 and 1920 by 1080 are, and 4:3 and 16:9 are not).
func sameShape(w, h, ow, oh int) bool {
	if w <= 0 || h <= 0 || ow <= 0 || oh <= 0 {
		return false
	}

	return math.Abs(float64(w*oh)/float64(h*ow)-1) <= shapeSlack
}

// presentation returns the presentationSize of l's item: the size of the
// frames it presents now, turned by the track's preferredTransform,
// CGSizeZero while unknown or for an item without video. It allocates
// nothing.
func (e *engine) presentation(l *load) (cgSize, bool) {
	if e.inv.presentationSize.Invoke(l.item) != nil {
		return cgSize{}, false
	}
	s, err := e.inv.presentationSize.Result[cgSize]()

	return s, err == nil
}

// videoGeometry returns the natural size of l's video track (as decoded,
// before any turn) and the turn of its preferredTransform; without a track
// that tells, the item's presentationSize as last read (l.shown), unturned.
// 0, 0 when unknown.
func (e *engine) videoGeometry(l *load) (w, h, rot int) {
	tracks := objc.Call(l.item, av.sel.tracks)
	n := int(objc.Call(tracks, av.sel.count))
	for i := range n {
		track := videoAsset(objc.Call(tracks, av.sel.objectAtIndex, uintptr(i)))
		if track == 0 {
			continue
		}
		if e.inv.naturalSize.Invoke(track) != nil {
			break
		}
		size, err := e.inv.naturalSize.Result[cgSize]()
		if err != nil || size.W <= 0 || size.H <= 0 {
			break
		}
		if e.inv.preferredTransform.Invoke(track) == nil {
			if t, err := e.inv.preferredTransform.Result[cgAffineTransform](); err == nil {
				rot = enginekit.RotationOf(t.A, t.B, t.C, t.D)
			}
		}

		return int(math.Round(size.W)), int(math.Round(size.H)), rot
	}
	if l.shown.W <= 0 || l.shown.H <= 0 {
		return 0, 0, 0
	}

	return int(math.Round(l.shown.W)), int(math.Round(l.shown.H)), 0
}

// videoAsset returns the AVAssetTrack of track, an AVPlayerItemTrack, when
// it is a video's (not owned), 0 otherwise.
func videoAsset(track objc.ID) objc.ID {
	asset := objc.Call(track, av.sel.assetTrack)
	if asset == 0 || !objc.CallBool(objc.Call(asset, av.sel.mediaType), av.sel.isEqualTo, uintptr(av.mediaTypeVideo)) {
		return 0
	}

	return asset
}

// newOutput returns an owned AVPlayerItemVideoOutput of BGRA frames w by h
// backed by IOSurfaces (the output scales in hardware).
func newOutput(w, h int) (objc.ID, error) {
	format, wv, hv, empty := cf.Int(pixelFormatBGRA), cf.Int(int64(w)), cf.Int(int64(h)), cf.Dictionary()
	attrs := cf.Dictionary(av.keyFormat, format, av.keyIOSurface, empty, av.keyWidth, wv, av.keyHeight, hv)
	for _, r := range []cf.Ref{format, wv, hv, empty} {
		cf.Release(r)
	}
	defer cf.Release(attrs)
	out := objc.Call(objc.Call(av.cls.output, av.sel.alloc), av.sel.initWithAttributes, uintptr(attrs))
	if out == 0 {
		return 0, fmt.Errorf("avf: no video output %dx%d: %w", w, h, core.ErrNotFound)
	}

	return out, nil
}
