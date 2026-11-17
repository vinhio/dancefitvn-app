//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/hold.go at 781bee5.

package focus

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SelectDown is Select going down (tvKey, index.html:762): Heal,
// then, unless Select is already down, the current item shows .pressing and the
// hold timer starts. A second down before the up is ignored. When the Pre hook
// takes the press (a screen that wants Select for itself), the press counts as
// already held: no .pressing, no timer, and the up selects nothing.
func (m *Manager) SelectDown() {
	m.Heal()
	if m.down {
		return
	}
	if m.pre != nil && m.pre(ButtonSelect) {
		m.down, m.held = true, true

		return
	}
	m.down, m.held = true, false
	if p, ok := m.Current().(Presser); ok {
		p.SetPressing(true)
		m.pressing = p
	}
	if m.clock != nil {
		m.timer = m.clock.AfterFunc(HoldDuration, m.holdFn)
	}
}

// SelectUp is Select going up (Q-P7-4, K27: on release): it cancels the
// timer, clears .pressing and, when the hold did not fire, selects. An up
// without a down is ignored.
func (m *Manager) SelectUp() {
	if !m.down {
		return
	}
	m.timer.Stop()
	m.down = false
	m.clearPressing()
	if !m.held {
		m.Select(false)
	}
}

// Select activates the current item (tvActivate, index.html:733): the item
// pulses, then runs Activate(long). A hold the item has no handler for runs as
// a short press. Nothing is focused: nothing happens.
func (m *Manager) Select(long bool) {
	cur := m.Current()
	if cur == nil {
		return
	}
	if p, ok := cur.(Presser); ok {
		p.Pulse()
	}
	a, ok := cur.(Activator)
	if !ok {
		return
	}
	if long && !a.Activate(true) {
		a.Activate(false)

		return
	}
	if !long {
		a.Activate(false)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// holdFired is the hold timer's call: the press became a hold.
func (m *Manager) holdFired() {
	if !m.down || m.held {
		return
	}
	m.held = true
	m.clearPressing()
	m.Select(true)
}

// clearPressing takes .pressing off the item that was given it, even when the
// focus has moved since.
func (m *Manager) clearPressing() {
	if m.pressing != nil {
		m.pressing.SetPressing(false)
		m.pressing = nil
	}
}
