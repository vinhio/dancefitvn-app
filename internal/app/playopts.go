package app

import (
	"sync"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// optionsKeeper remembers the video options the user chose (K41): the
// speed, the mirror and the beat bar. Once the preferences are open it
// puts the stored options on the player, and it saves each later change
// off the UI thread. The A–B loop and the audio repeat are not kept. The
// methods run on the UI thread; the saves run on the wire's background.
type optionsKeeper struct {
	pl         *player.Player
	prefs      func() *store.PrefsStore
	background func(func())
	logf       func(format string, args ...any)

	applied bool
	// saved is what the store holds (or was told to hold) on the UI thread.
	speed        float64
	mirror, beat bool

	mu     sync.Mutex // the latest asked for, whatever order the saves run in
	latest [3]float64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newOptionsKeeper returns the keeper of pl. Until opened it keeps the
// player's own defaults and saves nothing.
func newOptionsKeeper(pl *player.Player, prefs func() *store.PrefsStore, background func(func()),
	logf func(format string, args ...any),
) *optionsKeeper {
	return &optionsKeeper{pl: pl, prefs: prefs, background: background, logf: logf}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// opened puts the stored options on the player, once the store is open.
func (k *optionsKeeper) opened() {
	ps := k.prefs()
	if k.applied || ps == nil {
		return
	}
	k.applied = true
	p := ps.Get()
	k.speed, k.mirror, k.beat = p.Speed, p.Mirror, p.Beat
	k.pl.SetRate(p.Speed)
	k.pl.SetMirror(p.Mirror)
	k.pl.SetBeat(p.Beat)
}

// changed saves the options when the player's differ from the stored ones.
// It is called at each player change and allocates nothing when they do not.
func (k *optionsKeeper) changed() {
	if !k.applied {
		return
	}
	s, m, b := k.pl.Rate(), k.pl.Mirror(), k.pl.Beat()
	if s == k.speed && m == k.mirror && b == k.beat {
		return
	}
	k.speed, k.mirror, k.beat = s, m, b
	k.save(s, m, b)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

func (k *optionsKeeper) save(s float64, m, b bool) {
	ps := k.prefs()
	if ps == nil {
		return
	}
	k.mu.Lock()
	k.latest = [3]float64{s, flag(m), flag(b)}
	k.mu.Unlock()
	k.background(func() {
		k.mu.Lock()
		v := k.latest
		k.mu.Unlock()
		err := ps.Update(func(x *store.Prefs) { x.Speed, x.Mirror, x.Beat = v[0], v[1] == 1, v[2] == 1 })
		if err != nil {
			k.logf("dancefitvn: prefs: %v", err)
		}
	})
}

func flag(on bool) float64 {
	if on {
		return 1
	}

	return 0
}
