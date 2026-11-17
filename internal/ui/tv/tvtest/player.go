//go:build tvos

package tvtest

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Prefs is a fake env.Prefs with the defaults of a fresh app: auto next on, the
// quality automatic. The cycle is the app's: Tự động, 1080p, 720p, Tự động.
type Prefs struct {
	Auto bool
	Q    store.Quality

	// Cycles counts the calls of CycleQualityTV.
	Cycles int
}

var _ env.Prefs = (*Prefs)(nil)

// planeLayer is the handle the fake plane is added to the canvas with.
const planeLayer uintptr = 0xD00D

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPrefs returns the preferences of a fresh app.
func NewPrefs() *Prefs { return &Prefs{Auto: true, Q: store.QualityAuto} }

// NewPlayerServices returns signed-in fake services in the state of the prototype's
// player screens: the mock's course set-35, its user and support, a Playback
// holding lesson 3 (l_3503, "Stink and Dutty") at 72 of 220 seconds, paused, with
// a lesson before and after it, and fresh preferences.
func NewPlayerServices(t testing.TB) *Services {
	t.Helper()
	s := NewSignedServices(t)
	Mock(t, "me/me.json", &s.MeData)
	Mock(t, "app/config.json", &s.ConfigData)
	s.User = s.MeData.User
	s.PB = NewLessonPlayback(s.CourseData, 3)
	s.PrefsV = NewPrefs()

	return s
}

// NewLessonPlayback returns a Playback holding lesson no of c, paused at 72 of 220
// seconds, in the default options (speed 1, no mirror, loop or beat; the loop's
// ends at 33 % of the length and 28 s on).
func NewLessonPlayback(c model.Course, no int) *Playback {
	list := env.PlayerLessons(c)
	pb := &Playback{LessonV: list[no-1], KindV: player.KindVideo, StateV: player.StatePaused}
	pb.PosV, pb.DurV, pb.RateV = 72, 220, 1
	pb.AV, pb.BV = 73, 101
	pb.Prev_, pb.Next_ = no > 1, no < len(list)

	return pb
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AutoNext returns the preference.
func (p *Prefs) AutoNext() bool { return p.Auto }

// SetAutoNext sets the preference.
func (p *Prefs) SetAutoNext(on bool) { p.Auto = on }

// Quality returns the quality.
func (p *Prefs) Quality() store.Quality { return p.Q }

// CycleQualityTV counts the call and goes Tự động, 1080p, 720p, Tự động; a stored
// 480p goes to Tự động.
func (p *Prefs) CycleQualityTV() {
	p.Cycles++
	switch p.Q {
	case store.QualityAuto:
		p.Q = store.QualityP1080
	case store.QualityP1080:
		p.Q = store.QualityP720
	case store.QualityUnknown, store.QualityP720, store.QualityP480:
		p.Q = store.QualityAuto
	default:
		p.Q = store.QualityAuto
	}
}

// AttachPlane puts a fake plane under the tester's canvas, as the app's engine does
// through its Underlay, and has pb's Place move it: a rectangle shows it there, the
// empty one hides it. Tester.Underlays then tells where the plane is.
func AttachPlane(t testing.TB, tt *ui.Tester, pb *Playback) {
	t.Helper()
	cv, ok := tt.View().Canvas().(platform.UnderlayCanvas)
	if !ok {
		t.Fatalf("the tester's canvas takes no underlays")
	}
	u, err := cv.AddUnderlay(planeLayer)
	if err != nil {
		t.Fatalf("add underlay: %v", err)
	}
	pb.OnPlace = func(r geom.Rect) {
		if r.Empty() {
			u.SetHidden(true)

			return
		}
		u.SetFrame(r)
		u.SetHidden(false)
	}
	if !pb.Placed.Empty() { // the screen placed the plane before it was there
		pb.OnPlace(pb.Placed)
	}
}
