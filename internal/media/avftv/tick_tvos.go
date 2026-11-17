// Adapted from jplayer-mobile/internal/media/avf/tick_ios.go at 49de3c2
// (with engine_ios.go's invocations, seekTo, seekWatch, position and
// classes_ios.go's newBlock): the time tick, the seeks and the speed that
// jplayer-tv's engine, which plays live channels, does not have. No cgo:
// a CMTime goes through an objc.Invocation (CLAUDE.md §5, Q-P0-4).

//go:build ios && tvos

package avftv

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"math"
	"reflect"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// durationSlack is how much the length must change for the tick to
// report it again (a stream learns its length as it goes).
const durationSlack = 0.5

// invocations are the engine's messages that pass or answer floats and
// structs: made once by New, released by Close; UI thread.
type invocations struct {
	currentTime    *objc.Invocation // -[AVPlayer currentTime], AVPlayer.h:291
	duration       *objc.Invocation // -[AVPlayerItem duration], AVPlayerItem.h:206
	rangeValue     *objc.Invocation // -[NSValue CMTimeRangeValue], AVTime.h:32
	seek           *objc.Invocation // -[AVPlayer seekToTime:toleranceBefore:toleranceAfter:completionHandler:], AVPlayer.h:358
	setRate        *objc.Invocation // -[AVPlayer setRate:], AVPlayer.h:152
	setDefaultRate *objc.Invocation // -[AVPlayer setDefaultRate:], AVPlayer.h:163
	setVolume      *objc.Invocation // -[AVPlayer setVolume:], AVPlayer.h:490
	rate           *objc.Invocation // -[AVPlayer rate], for the device measures
	defaultRate    *objc.Invocation // -[AVPlayer defaultRate], for the device measures
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

// seekArgs are the arguments of a seek's completion handler after the
// block: BOOL finished, which it does not read (a register's width).
var seekArgs = []reflect.Type{reflect.TypeFor[uintptr]()}

// blockType is the type a block's function takes first (purego's
// objc.Block, which gDesk's objc returns from NewBlock but does not
// name).
var blockType = reflect.TypeOf(objc.NewBlock).Out(0)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// tick is the 250 ms time tick, on the UI thread while the media plays:
// the position, how far the media is loaded when that changed, and the
// length when it changed; until the first frame of a load with a picture
// is told, it looks for it (checkPicture). It allocates nothing in the steady state: the
// messages that answer CMTimes go through invocations made once, the
// rest through objc.Call, inside a pool (the item's ranges come
// autoreleased).
func (e *Engine) tick() { objc.WithPool(e.tickInPoolFn) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tickInPool is tick inside its pool.
func (e *Engine) tickInPool() {
	if e.closed || e.item == 0 || !e.ready || e.failed {
		return
	}
	e.ticks++
	if !e.pictured { // a frame shown before its size was known
		e.checkPicture()
	}
	if t, ok := e.position(); ok {
		e.publish(media.Event{Kind: media.EventTime, Time: t})
	}
	if b := e.bufferedEnd(); b != e.buffered {
		e.buffered = b
		e.publish(media.Event{Kind: media.EventBuffered, Buffered: b})
	}
	if d, ok := e.itemDuration(); ok && math.Abs(d-e.dur) > durationSlack {
		e.dur = d
		e.publish(media.Event{Kind: media.EventDuration, Duration: d})
	}
}

// bufferedEnd returns where the loaded part of the item ends: the end of
// the last of its loadedTimeRanges.
func (e *Engine) bufferedEnd() float64 {
	ranges := objc.Call(e.item, sel.loadedTimeRanges)
	n := int(objc.Call(ranges, sel.count))
	if n == 0 {
		return 0
	}
	v := objc.Call(ranges, sel.objectAtIndex, uintptr(n-1))
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

// seekTo seeks the player to t seconds exactly, and counts the seek until
// its completion handler tells it is over (seekWatch). UI thread.
func (e *Engine) seekTo(t float64) {
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
	e.goal, e.goalToken = t, e.token
}

// newSeekWatch returns the seekWatch of a new player: its completion
// handler, one block for every seek, counts the seek done on the UI
// thread. Its C callback is made once (newBlock).
func (e *Engine) newSeekWatch() *seekWatch {
	w := &seekWatch{}
	done := w.Done
	w.block = newBlock(seekArgs, func([]reflect.Value) {
		if objc.IsMainThread() {
			done()

			return
		}
		e.post(done)
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

// position returns the player's time in seconds.
func (e *Engine) position() (float64, bool) {
	if e.inv.currentTime.Invoke(e.player) != nil {
		return 0, false
	}
	t, err := e.inv.currentTime.Result[enginekit.CMTime]()
	if err != nil {
		return 0, false
	}

	return t.Seconds()
}

// itemDuration returns the length of the item, false while unknown or for
// a stream without end.
func (e *Engine) itemDuration() (float64, bool) {
	if e.inv.duration.Invoke(e.item) != nil {
		return 0, false
	}
	t, err := e.inv.duration.Result[enginekit.CMTime]()
	if err != nil {
		return 0, false
	}
	d, ok := t.Seconds()

	return d, ok && d > 0
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
		{&in.rate, "AVPlayer", "rate"},
		{&in.defaultRate, "AVPlayer", "defaultRate"},
	} {
		made, err := objc.NewInvocation(objc.GetClass(x.cls), x.name)
		if err != nil {
			return fmt.Errorf("avftv: -[%s %s]: %w", x.cls, x.name, err)
		}
		*x.dst = made
	}

	return nil
}

// release releases the invocations made.
func (in *invocations) release() {
	for _, x := range []*objc.Invocation{
		in.currentTime, in.duration, in.rangeValue, in.seek, in.setRate, in.setDefaultRate,
		in.setVolume, in.rate, in.defaultRate,
	} {
		if x != nil {
			x.Release()
		}
	}
	*in = invocations{}
}

// newBlock makes an owned Objective-C block of a function taking args after
// the block itself and answering nothing; fn gets those arguments. Its C
// callback is made once per list of argument types (purego caches it by the
// function's type, which reflect.FuncOf makes the same each time), so blocks
// made for each player cost no new callback. Release it (objc.Release) once
// the API it is given has its copy.
func newBlock(args []reflect.Type, fn func(args []reflect.Value)) objc.ID {
	in := append([]reflect.Type{blockType}, args...)
	typ := reflect.FuncOf(in, nil, false)
	impl := reflect.MakeFunc(typ, func(vals []reflect.Value) []reflect.Value {
		fn(vals[1:])

		return nil
	})

	return objc.ID(objc.NewBlock(impl.Interface()))
}
