// Adapted from jplayer-mobile/internal/media/amp/stats_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Stats are an engine's counters, for the device tests and measures
// (StatsOf).
type Stats struct {
	// LiveGlobalRefs, LiveProxies and LiveReaders count the global
	// references, listener proxies and AImageReaders of the package not
	// yet released (every engine and session of the process).
	LiveGlobalRefs, LiveProxies, LiveReaders int
	// Frames counts the frames the readers handed over; CopiedFrames those
	// converted into the Surface, DroppedFrames those let go unconverted
	// (no view watching, or the app in the background), BadFrames those
	// that could not be read.
	Frames, CopiedFrames, DroppedFrames, BadFrames int64
	// ReaderCallbacks and ReaderFailures count the current reader's
	// callbacks and failed acquires.
	ReaderCallbacks, ReaderFailures int64
	// Releasing counts the loads the opener has still to open or the
	// releaser to free.
	Releasing int
	// WrongState counts the players' errors of a call in a wrong state
	// (what -38); Refused the calls the state machine kept from them.
	WrongState, Refused int
	// ConvertP50 and ConvertP95 are the times of the last conversions.
	ConvertP50, ConvertP95 time.Duration
	// Loads and Unloads are the times the last loads and unloads took on
	// the UI thread; Opens those the opener took to make their players.
	Loads, Unloads, Opens []time.Duration
	// OffMainEvents counts the players' events that came on another
	// thread than Java's main one: 0, as no player is made on a thread
	// with a Looper.
	OffMainEvents int64
	// NativeHeap is Debug.getNativeHeapAllocatedSize; RSS the resident
	// set of the process, in bytes.
	NativeHeap, RSS int64
	// WifiLock tells that the engine's Wi-Fi lock is held, as the system
	// answers (WifiLock.isHeld): while a link plays or prepares to.
	WifiLock bool
	// HandoffLock tells that the CPU is kept awake from a media's end to
	// the next's start (PowerManager.WakeLock.isHeld).
	HandoffLock bool
	// Detaches and Attaches count the Surfaces the background took from
	// the players of videos with sound and the foreground gave back;
	// Detached tells that the current player has none now.
	Detaches, Attaches int
	Detached           bool
	// Reseeks counts the seeks the engine made again after a Surface
	// given back, as the player went to a key frame on its own (Samsung's
	// NuPlayer).
	Reseeks int
	// HoldsImages tells that the video's Surface holds images.
	HoldsImages bool
}

// SessionStats are a session's counters, for the device tests
// (SessionStatsOf).
type SessionStats struct {
	// Requests counts the requests of the audio focus.
	Requests int
	// Held tells that the app holds the focus, lost for a while or not.
	Held bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SessionStatsOf returns the counters of a session NewSession made, false
// for another. UI thread.
func SessionStatsOf(m media.Session) (SessionStats, bool) {
	s, ok := m.(*session)
	if !ok {
		return SessionStats{}, false
	}

	return SessionStats{Requests: s.requests, Held: s.audioFocus.Held()}, true
}

// StatsOf returns the counters of an engine New made, false for another
// engine. Call it on the UI thread; it allocates, and asks Java for the
// native heap.
func StatsOf(m media.Engine) (Stats, bool) {
	e, ok := m.(*engine)
	if !ok {
		return Stats{}, false
	}
	s := Stats{
		LiveGlobalRefs: int(liveGlobals.Load()),
		LiveProxies:    int(liveProxies.Load()),
		LiveReaders:    int(liveReaders.Load()),
		Frames:         e.prod.frames.Load(),
		CopiedFrames:   e.prod.copied.Load(),
		DroppedFrames:  e.prod.dropped.Load(),
		BadFrames:      e.prod.bad.Load(),
		WrongState:     e.wrongState,
		Releasing:      int(e.releasing.Load()),
		Refused:        e.refused,
		Loads:          e.loads.list(),
		Unloads:        e.unloads.list(),
		Opens:          e.opens.list(),
		OffMainEvents:  offMain.Load(),
		RSS:            residentBytes(),
	}
	s.ConvertP50, s.ConvertP95 = e.prod.convertTimes()
	s.WifiLock = e.wifi.isHeld()
	s.HandoffLock = e.handoff.isHeld()
	if l := e.cur; l != nil && l.reader != nil {
		s.ReaderCallbacks, s.ReaderFailures, _ = l.reader.stats()
	}
	s.Detaches, s.Attaches, s.HoldsImages = e.detaches, e.attaches, e.video.Holds()
	s.Reseeks = e.reseeks
	if l := e.cur; l != nil {
		s.Detached = l.detached
	}
	_ = jni.Do(func(env jni.Env) error {
		var err error
		s.NativeHeap, err = env.CallStaticLong(ids.debug, ids.nativeHeap)

		return err
	})

	return s, true
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// residentBytes reads the resident set of the process from
// /proc/self/statm (its second field, in pages), 0 when it cannot.
func residentBytes() int64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(data))
	if len(f) < 2 {
		return 0
	}
	pages, err := strconv.ParseInt(f[1], 10, 64)
	if err != nil {
		return 0
	}

	return pages * int64(os.Getpagesize())
}
