// Adapted from jplayer-mobile/internal/media/avf/tick_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"math"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// tick is the 250 ms time tick (04 §2.1), on the UI thread: the position,
// how far the media is loaded when that changed, the length when it changed
// (a stream learns it as it goes), and a new output when the frames change
// size (followSize). It allocates nothing in the steady state:
// the messages that answer CMTimes go through invocations made once, the
// rest through objc.Call, inside a pool (the item's ranges come
// autoreleased).
func (e *engine) tick() { objc.WithPool(e.tickInPoolFn) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tickInPool is tick inside its pool.
func (e *engine) tickInPool() {
	l := e.cur
	if l == nil || !l.ready || l.failed {
		return
	}
	if t, ok := e.position(); ok {
		e.publish(l, media.Event{Kind: media.EventTime, Time: t})
	}
	if b := e.bufferedEnd(l); b != l.buffered {
		l.buffered = b
		e.publish(l, media.Event{Kind: media.EventBuffered, Buffered: b})
	}
	if d, ok := e.itemDuration(l); ok && math.Abs(d-l.dur) > durationSlack {
		l.dur = d
		e.publish(l, media.Event{Kind: media.EventDuration, Duration: d})
	}
	e.followSize(l)
}

// waitOutput is outputWait's tick: it looks for the output of the current
// video, which had no size when it became ready. UI thread.
func (e *engine) waitOutput() { objc.WithPool(e.outputInPoolFn) }

// waitOutputInPool gives the current video its output once its size is
// known (it allocates, once) and stops outputWait, which nothing else stops
// but an unload, a failure or a load without a video to wait for. The
// Playing or the paused state came before the output and started no pulls,
// so they start now: playing, they go on; paused, a seek to the position
// makes the output decode the frame shown (as onReady's seek does) and a
// settle pulls it, so the box shows that frame rather than black. Waiting,
// the Playing to come starts them. In a pool.
func (e *engine) waitOutputInPool() {
	l := e.cur
	if l == nil || !l.ready || l.failed || l.src.AudioOnly || l.output != 0 {
		e.outputWait.Stop()

		return
	}
	e.attachOutput(l)
	if l.output == 0 {
		return // not known yet: the next tick
	}
	e.outputWait.Stop()
	switch l.control {
	case controlPlaying:
		e.settling = false
		e.activate()
	case controlPaused:
		if !l.ended {
			if t, ok := e.position(); ok {
				e.seekTo(l, t)
			}
		}
		e.settle()
	default:
	}
}

// bufferedEnd returns where the loaded part of l's item ends: the end of
// the last of its loadedTimeRanges (player.js:56).
func (e *engine) bufferedEnd(l *load) float64 {
	ranges := objc.Call(l.item, av.sel.loadedTimeRanges)
	n := int(objc.Call(ranges, av.sel.count))
	if n == 0 {
		return 0
	}
	v := objc.Call(ranges, av.sel.objectAtIndex, uintptr(n-1))
	if v == 0 || e.inv.rangeValue.Invoke(v) != nil {
		return 0
	}
	r, err := e.inv.rangeValue.Result[enginekit.CMTimeRange]()
	if err != nil {
		return 0
	}
	e.ranges[0] = r

	return enginekit.BufferedEnd(e.ranges[:])
}
