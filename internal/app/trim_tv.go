//go:build tvos

// Adapted from jplayer-tv/internal/app/app.go at 781bee5 (dialogsGone:
// the memory given back to the system, by one goroutine at a time): on the
// TV it runs when a screen, the panel or a toast goes (local/plans/P7.md
// P7.5c, the memory gate R1). jPlayer also trims the view there, for the
// frosted backdrops its dialogs draw on the CPU; the TV draws none (F-11),
// the gate found nothing a panel or a toast keeps (20 of each: -3.4 and
// -8.3 MB of footprint at 4K, local/p7/p75c/part2/memgate-trips30.md), and
// a trimmed view uploads every poster again on its next frame, so the view
// is not trimmed. The screens' trips were what grew: 30 TV05 → TV06 → TV05
// trips +25.5 MB without this (the Go runtime's mapped memory, the live
// heap flat), +1.1 MB with it (memgate-free-trips30.md).

package app

import (
	"runtime/debug"
	"sync/atomic"

	"dancefitvn-app/internal/ui/tv/shell"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvMemory gives the Go heap's free memory back to the system when
// something on screen goes: a screen left (each TV05 → TV06 → TV05 trip
// builds a screen anew, megabytes of garbage that the runtime keeps mapped
// up to its next collection), the panel closed,
// a toast gone. after is the view's AfterFrame: it looks at the shell,
// allocating nothing; the freeing runs off the UI thread, one at a time:
// what goes in quick succession leaves one in flight, not one each (the
// next one to go finds the memory already free or starts it again).
type tvMemory struct {
	sh *shell.Shell
	// What the last frame showed.
	route        string
	panel, toast bool
	// free gives the memory back (debug.FreeOSMemory); freeing tells one
	// runs; frees counts those started, for the tests.
	free    func()
	freeing atomic.Bool
	frees   atomic.Int64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVMemory returns the watcher of sh, giving memory back with
// debug.FreeOSMemory.
func newTVMemory(sh *shell.Shell) *tvMemory {
	return &tvMemory{sh: sh, free: debug.FreeOSMemory}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// after runs after each frame: when the route is another than the last
// frame's, or the panel or the toast the last frame showed is gone, the
// memory goes back. UI thread.
func (m *tvMemory) after() {
	st := m.sh.Stack()
	route, panel, toast := st.Screen(), st.Panel(), m.sh.ToastHost().Visible()
	gone := (m.route != "" && route != m.route) || (m.panel && !panel) || (m.toast && !toast)
	m.route, m.panel, m.toast = route, panel, toast
	if gone {
		m.giveBack()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// giveBack runs free on a goroutine unless one runs already.
func (m *tvMemory) giveBack() {
	if !m.freeing.CompareAndSwap(false, true) {
		return
	}
	m.frees.Add(1)
	go func() {
		defer m.freeing.Store(false)
		m.free()
	}()
}
