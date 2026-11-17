// Adapted from jplayer-mobile/internal/media/amp/load_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// load is one Load: its MediaPlayer, the proxy of its listeners, its
// reader, and where the player is in its state machine (D6: a new player
// and proxy for each load, so a late event of an older one carries its
// own load and is dropped). The UI thread owns it, but for what the opener
// makes the player with: the opener reads src and the funcs made at the
// load, and hands its result over in res, under mu (open_android.go). The
// proxy's function, on Java's main thread, only reads the funcs made at
// the load and writes the atomics.
type load struct {
	e     *engine
	token uint64
	src   media.Source
	opts  media.LoadOptions
	local bool

	mp     jni.Object // global
	proxy  *jni.Proxy
	cancel context.CancelFunc // ends the preflight
	reader *reader
	state  enginekit.MPState
	// pending is what waits for onPrepared: Play, the seek.
	pending  enginekit.Pending
	prepared bool
	dur      float64 // seconds, 0 when unknown (live)
	rate     float64 // the speed the player plays at, 1 at first
	seekTo   float64 // the last seek asked, seconds
	vw, vh   int     // the picture size last reported
	// hasAudio tells that the video has a sound track (onPrepared), so
	// that its player plays on without its Surface; detached that the
	// background took the Surface away (background_android.go).
	hasAudio, detached bool

	// seeks counts the seeks asked of the player, seeksDone those it
	// reported finished (it reports each one, the ones it skipped too):
	// while one is under way ReadTime tells seekTo, as the player may
	// tell the position it left.
	seeks, seeksDone int
	// reattach guards the seek the Surface given back from the
	// background was given with: a player that lands behind it (its own
	// seek to the key frame before) seeks there again (attach).
	reattach enginekit.ReattachSeek

	buffered atomic.Int32 // percent, from onBufferingUpdate
	size     atomic.Int64 // width<<32 | height, from onVideoSizeChanged

	// opening is a load the opener has not handed its player to yet:
	// Play, Pause and Seek wait in pending, as they do while preparing.
	opening bool
	// The events of the player that came before its opener's result did
	// (the player prepares as the opener hands it over): applyOpen
	// replays them.
	earlyPrepared, earlyErr bool
	earlyWhat, earlyExtra   int32

	mu      sync.Mutex // guards dropped and res, between the UI and the opener
	dropped bool       // the load went: the opener frees what it made
	res     *opened    // the opener's result, until applyOpen or release takes it

	// The work the listeners and the opener hand the UI thread, made at
	// the load so a frequent event allocates nothing.
	preparedFn, completedFn, seekedFn, bufferedFn, sizeFn, openedFn func()
}

// errCleartext is a link of plain http the app's network security policy
// forbids (Q1: android.cleartext off).
var errCleartext = errors.New("amp: cleartext http not permitted")

// LinkVerdict is what the app's check of a linked document tells
// (SetLinkCheck): the engine's own errors cannot (C5).
type LinkVerdict uint8

// The verdicts of a check of a linked document.
const (
	// LinkVerdictUnknown is no verdict: as LinkFine, the failure stays the
	// engine's.
	LinkVerdictUnknown LinkVerdict = iota
	// LinkFine is a document there, or one the check could not tell: the
	// failure stays the engine's.
	LinkFine
	// LinkGone is a document gone for good: media.ErrMissingFile.
	LinkGone
	// LinkAway is a document out of reach now: media.ErrUnreachable.
	LinkAway
)

// linkCheck is the check SetLinkCheck set, nil for none.
var linkCheck atomic.Pointer[func(url string) LinkVerdict]

// localFrame is the room of the local frame the releaser pushes for each
// load it frees.
const localFrame = 8

// liveProxies counts the listeners' proxies made and not closed (Stats).
var liveProxies atomic.Int64

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newLoad returns the load of src, with its listeners' work made.
func (e *engine) newLoad(src media.Source, opts media.LoadOptions) *load {
	l := &load{e: e, token: e.token, src: src, opts: opts, local: isLocal(src.URL), rate: 1}
	l.buffered.Store(-1)
	l.pending.Play = opts.Auto
	if opts.Start > 0 && !src.Live {
		l.pending.Seek, l.pending.HasSeek = opts.Start, true
	}
	l.preparedFn = func() { e.onPrepared(l) }
	l.completedFn = func() { e.onCompletion(l) }
	l.seekedFn = func() { e.onSeekComplete(l) }
	l.bufferedFn = func() { e.onBuffered(l) }
	l.sizeFn = func() { e.onVideoSize(l) }
	l.openedFn = func() { e.applyOpen(l) }

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// open makes the player of l and starts preparing it: a MediaPlayer with
// the listeners, music or movie attributes (required, 04 §3.1), the wake
// lock when the app may hold one, the source, the reader's Surface for a
// video. It runs on the opener's thread (R3: the first MediaPlayer of the
// process takes 300 ms, a file's setDataSource up to 16): it only reads
// what never changes after New and l's source, and keeps the player's
// state in its result, which holds what it made even when it fails. The
// volume waits for onPrepared (Preparing takes none).
func (e *engine) open(env jni.Env, l *load) *opened {
	r := &opened{}
	if blocked, err := cleartextBlocked(env, l.src.URL); err != nil {
		r.err = err

		return r
	} else if blocked {
		r.err = errCleartext

		return r
	}
	mp, err := env.NewObject(ids.player, ids.playerNew)
	if err != nil {
		r.err = err

		return r
	}
	r.mp = newGlobal(env, mp)
	env.DeleteLocalRef(mp)
	r.state = enginekit.MPIdle
	if r.proxy, r.err = jni.NewProxy(env, listenerInterfaces, listenerMethods, l.listen); r.err != nil {
		return r
	}
	liveProxies.Add(1)
	for _, set := range ids.listenerSetters {
		if r.err = env.CallVoid(r.mp, set, jni.Ref(r.proxy.Object())); r.err != nil {
			return r
		}
	}
	attrs := e.attrsMovie
	if l.src.AudioOnly {
		attrs = e.attrsMusic
	}
	if r.err = env.CallVoid(r.mp, ids.setAudioAttributes, jni.Ref(attrs)); r.err != nil {
		return r
	}
	if e.wake {
		// MediaPlayer takes the wake lock as it starts: without the
		// permission, start would throw.
		if err := env.CallVoid(r.mp, ids.setWakeMode, jni.Ref(jni.Context()), jni.Int(partialWakeLock)); err != nil {
			logf("amp: setWakeMode: %v", err)
		}
	}
	if r.err = e.setSource(env, r, l.src.URL); r.err != nil {
		return r
	}
	if !l.src.AudioOnly {
		// Before prepareAsync: Preparing takes no setSurface, and a
		// Surface given late costs a reconfiguration of the decoder.
		if r.reader, err = newReader(env, e.handle); err != nil {
			logf("amp: no frames: %v", err)
		} else if r.err = r.call(env, enginekit.CallSetSurface, ids.setSurface, jni.Ref(r.reader.surface)); r.err != nil {
			return r
		}
	}
	r.err = r.call(env, enginekit.CallPrepareAsync, ids.prepareAsync)

	return r
}

// setSource sets the player of r to rawURL: a path or a link as a
// String, a content:// document (a linked file, LINKED §3.4) as a Uri
// with the app's Context, which opens it through its provider. A
// document that fails is checked (contentError, C5). Opener's thread.
func (e *engine) setSource(env jni.Env, r *opened, rawURL string) error {
	if !enginekit.IsContent(rawURL) {
		s, err := env.NewString(rawURL)
		if err != nil {
			return err
		}
		defer env.DeleteLocalRef(s)

		return r.call(env, enginekit.CallSetDataSource, ids.setDataSource, jni.Ref(s))
	}
	uri, err := parseURI(env, rawURL)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(uri)
	if err := r.call(env, enginekit.CallSetDataSource, ids.setDataSourceURI, jni.Ref(jni.Context()), jni.Ref(uri)); err != nil {
		return contentError(rawURL, err)
	}

	return nil
}

// SetLinkCheck sets how the engines and the probers tell why a content://
// document failed to open (contentError), for the process: check, which
// blocks, tells whether the document of url is gone or out of reach. The
// app sets it once, before its engine plays a linked file, to its link
// layer's check (sys.CheckLink); nil sets none, and a failure stays the
// engine's (media.ErrSource). Any goroutine.
func SetLinkCheck(check func(url string) LinkVerdict) {
	if check == nil {
		linkCheck.Store(nil)

		return
	}
	linkCheck.Store(&check)
}

// unload drops the current load (release).
func (e *engine) unload() {
	l := e.cur
	if l == nil {
		return
	}
	e.cur = nil
	e.release(l)
}

// release frees what l holds, in the order M4.2a found safe: the reader's
// callback closed here, at once, so no frame reaches the Surface after
// it; then, on the releaser's thread, the player released (it writes no
// more frames, and its listeners are unset), the proxy closed, the
// Surface released and the reader deleted. MediaPlayer.release waits for
// the media server, 21 ms at the median and 61 ms at worst on the
// emulator: too long for the UI thread. A load the opener has not handed
// over yet is marked dropped: the opener frees what it makes, or release
// takes the result waiting for applyOpen. It ends the preflight. UI
// thread.
func (e *engine) release(l *load) {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	if e.cur == l || e.cur == nil {
		e.ticker.Stop()
		e.video.SetActive(false)
	}
	if l.opening {
		l.opening = false
		l.mu.Lock()
		l.dropped = true
		r := l.res
		l.res = nil
		l.mu.Unlock()
		if r != nil {
			l.mp, l.proxy, l.reader, l.state = r.mp, r.proxy, r.reader, r.state
		}
	}
	r := held{mp: l.mp, proxy: l.proxy, reader: l.reader, call: l.state.Allows(enginekit.CallRelease)}
	l.mp, l.proxy, l.reader = 0, nil, nil
	if l.state != enginekit.MPError {
		l.state = enginekit.MPEnd
	}
	e.hand(r)
}

// hand closes the reader of what a load held, at once, and hands the rest
// to the releaser; nothing held is nothing to do. UI thread or the
// opener's.
func (e *engine) hand(r held) {
	r.reader.close()
	if r.mp == 0 && r.proxy == nil && r.reader == nil {
		return
	}
	e.releasing.Add(1)
	e.releases <- r
}

// held is what a load held, on its way to the releaser.
type held struct {
	mp     jni.Object // global
	proxy  *jni.Proxy
	reader *reader
	call   bool // the player's state allows release()
}

// releaser frees what loads held, in turn, on a thread of its own
// attached to the VM for its life, until Close closes the queue.
func (e *engine) releaser() {
	defer close(e.released)
	err := jni.Do(func(env jni.Env) error {
		for r := range e.releases {
			framed := env.PushLocalFrame(localFrame) == nil
			r.free(env)
			if framed {
				env.PopLocalFrame(0)
			}
			e.releasing.Add(-1)
		}

		return nil
	})
	if err != nil { // no VM: nothing to release with
		for range e.releases {
			e.releasing.Add(-1)
		}
	}
}

// free releases the player, closes the proxy, and frees the reader.
func (r held) free(env jni.Env) {
	if r.mp != 0 {
		if r.call {
			if err := env.CallVoid(r.mp, ids.release); err != nil {
				logf("amp: release: %v", err)
			}
		}
		deleteGlobal(env, r.mp)
	}
	if r.proxy != nil {
		if err := r.proxy.Close(); err != nil {
			logf("amp: close the listeners: %v", err)
		}
		liveProxies.Add(-1)
	}
	r.reader.free(env)
}

// fail ends l, the current load, with err: its player released (so it
// retries no more), the error told. Play then reports errFailed until the
// next Load.
func (e *engine) fail(l *load, err error) {
	if e.cur != l {
		return
	}
	e.release(l)
	e.wifi.set(false)
	l.state = enginekit.MPError
	e.emit(l, media.Event{Kind: media.EventError, Err: err})
}

// startPreflight checks a link alongside the player (preflight), until
// the player is prepared or the load goes.
func (e *engine) startPreflight(l *load) {
	if l.local || !isHTTP(l.src.URL) {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	rawURL := l.src.URL
	go func() {
		err := preflight(ctx, rawURL)
		if err == nil || ctx.Err() != nil {
			return
		}
		e.cfg.Dispatch(func() {
			if e.cur == l && !l.prepared && l.state != enginekit.MPError {
				e.fail(l, enginekit.OpenError(enginekit.PhaseOpening, false, rawURL, err))
			}
		})
	}()
}

// waitState is l's state, as the calls that wait for the player see it: a
// load the opener has not handed over is one preparing.
func (l *load) waitState() enginekit.MPState {
	if l.opening {
		return enginekit.MPPreparing
	}

	return l.state
}

// call makes the call c of l's player, method m, when its state allows it,
// and follows the state. A call the state does not allow is not made: it
// is counted, and reported as core.ErrInvalid.
func (e *engine) call(env jni.Env, l *load, c enginekit.MPCall, m jni.MethodID, args ...jni.Value) error {
	if l.mp == 0 || !l.state.Allows(c) {
		e.refused++

		return fmt.Errorf("amp: call %d in state %d: %w", c, l.state, core.ErrInvalid)
	}
	if err := env.CallVoid(l.mp, m, args...); err != nil {
		return err
	}
	l.state = l.state.After(c)

	return nil
}

// applyVolume sets the player's volume (0 when muted) when its state takes
// it; onPrepared sets it otherwise.
func (e *engine) applyVolume(env jni.Env, l *load) {
	v := float32(e.vol)
	if e.muted {
		v = 0
	}
	if !l.state.Allows(enginekit.CallSetVolume) {
		l.pending.Volume, l.pending.VolumeSet = v, true

		return
	}
	if err := e.call(env, l, enginekit.CallSetVolume, ids.setVolume, jni.Float(v), jni.Float(v)); err != nil {
		logf("amp: setVolume: %v", err)
	}
	l.pending.VolumeSet = false
}

// applyRate sets the speed of a playing player. setPlaybackParams with a
// speed starts a prepared or paused player, so only a started one takes
// it; the others get it as they start.
func (e *engine) applyRate(env jni.Env, l *load) error {
	if l.state != enginekit.MPStarted || e.rate <= 0 || e.rate == l.rate {
		return nil
	}
	p, err := env.CallObject(e.params, ids.setSpeed, jni.Float(float32(e.rate)))
	if err != nil {
		return err
	}
	env.DeleteLocalRef(p)
	if err := e.call(env, l, enginekit.CallSetPlaybackParams, ids.setPlaybackParams, jni.Ref(e.params)); err != nil {
		return err
	}
	l.rate = e.rate

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cleartextBlocked tells whether rawURL is plain http to a host the app's
// network security policy keeps from cleartext, as MediaPlayer's own
// connection would find out later, worded less clearly.
func cleartextBlocked(env jni.Env, rawURL string) (bool, error) {
	if len(rawURL) < 7 || !strings.EqualFold(rawURL[:7], "http://") {
		return false, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return true, nil // a link that does not parse cannot be opened either
	}
	policy, err := env.CallStaticObject(ids.policy, ids.policyInstance)
	if err != nil || policy == 0 {
		return false, err
	}
	defer env.DeleteLocalRef(policy)
	host, err := env.NewString(u.Hostname())
	if err != nil {
		return false, err
	}
	defer env.DeleteLocalRef(host)
	ok, err := env.CallBool(policy, ids.cleartextPermitted, jni.Ref(host))

	return !ok, err
}

// isLocal tells whether rawURL is on the phone: a path, or a content://
// document (a linked file): no Wi-Fi lock, no preflight, read through
// the retriever as a file.
func isLocal(rawURL string) bool {
	return !strings.Contains(rawURL, "://") || enginekit.IsContent(rawURL)
}

// parseURI returns the android.net.Uri of s, a local reference.
func parseURI(env jni.Env, s string) (jni.Object, error) {
	js, err := env.NewString(s)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(js)
	uri, err := env.CallStaticObject(ids.uri, ids.uriParse, jni.Ref(js))
	if err == nil && uri == 0 {
		err = fmt.Errorf("amp: no Uri of %q: %w", s, core.ErrInvalid)
	}

	return uri, err
}

// contentError tells why the content:// document rawURL failed to open
// with cause. MediaPlayer and MediaMetadataRetriever swallow the
// provider's FileNotFoundException and SecurityException and throw the
// same IOException or IllegalArgumentException for every cause (C5 of
// local/linked/spike.md), so the link is checked as the library's sweep
// checks it, through the check SetLinkCheck set (sys.CheckLink, which the
// core may not import): gone is media.ErrMissingFile, out of reach
// media.ErrUnreachable, and a document that is there, or with no check
// set, fails with cause. It blocks for a check (4-40 ms): the opener's
// thread or a probe's goroutine only.
func contentError(rawURL string, cause error) error {
	check := linkCheck.Load()
	if check == nil {
		return cause
	}
	switch (*check)(rawURL) {
	case LinkGone:
		return fmt.Errorf("amp: %s: %v: %w", rawURL, cause, media.ErrMissingFile)
	case LinkAway:
		return fmt.Errorf("amp: %s: %v: %w", rawURL, cause, media.ErrUnreachable)
	case LinkFine, LinkVerdictUnknown:
		return cause
	default:
		return cause
	}
}

// packSize and unpackSize keep a picture size in one atomic.
func packSize(w, h int32) int64 { return int64(w)<<32 | int64(uint32(h)) }

func unpackSize(v int64) (w, h int) { return int(int32(v >> 32)), int(int32(uint32(v))) }
