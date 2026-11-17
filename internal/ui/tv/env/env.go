//go:build tvos

package env

import (
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/tv/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The routes of the TV: the prototype's `S.tv.scr` (index.html:645-717).
const (
	// RouteWelcome is TS.welcome (TV01), the first screen when signed out.
	RouteWelcome = "welcome"
	// RouteCode is TS.code (TV02): the pairing code.
	RouteCode = "code"
	// RouteHome is TS.home (TV04), the first screen when signed in; Nav item 0.
	RouteHome = "home"
	// RouteCourses is TS.courses (TV05); Nav item 1.
	RouteCourses = "courses"
	// RouteDetail is TS.detail (TV06); Params is the course id (`S.tv.set`).
	RouteDetail = "detail"
	// RoutePlayer is TS.player (TV07, and TV08 with its panel).
	RoutePlayer = "player"
	// RouteMusic is TS.music (TV09); Nav item 2.
	RouteMusic = "music"
	// RouteUpNext is TS.upnext (TV10).
	RouteUpNext = "upnext"
	// RouteError is TS.error (TV11).
	RouteError = "error"
	// RouteSettings is TS.settings (TV12); Nav item 3.
	RouteSettings = "settings"
)

// Tabs are the routes of the four items of the nav bar, in order: nav0..nav3.
var Tabs = [...]string{RouteHome, RouteCourses, RouteMusic, RouteSettings}

// Actions is what the Shell carries out for the screens (03 section 7.3): the
// screens call it, they never touch the stack or the focus themselves.
type Actions interface {
	// Go is tvGo: r becomes the screen, the one showing is pushed unless root
	// is set (a root empties the stack), the focus goes to the default of the
	// new screen and the panel closes. Leaving for a screen that is not the
	// player or the music pauses the playback.
	Go(r nav.Route, root bool)
	// Replace is the player and the up-next screen taking each other's place:
	// nothing is pushed.
	Replace(r nav.Route)
	// Back is tvBack: it closes the panel and focuses p2, else pops the stack
	// (and pauses); false when there is nothing to do, so that Menu is left to
	// the system.
	Back() bool
	// SwitchTab is a Select on a nav item: the screen of item i takes the place
	// of the one showing, the stack empties and the focus stays on the item.
	SwitchTab(i int)
	// Toast is tvToast: a message for 2400 ms.
	Toast(msg string)
	// OpenPanel opens the player's panel (`topts`): the focus is confined to
	// panel and goes to its first item.
	OpenPanel(panel ui.Widget)
	// ClosePanel closes it (`tclose`), the focus going to the item with focusKey.
	ClosePanel(focusKey string)
	// OnPlayPause adds a Play/Pause handler, for the player and the music
	// screens; remove it when the screen is left. Without one the key is left to
	// the system.
	OnPlayPause(fn func()) (remove func())
}

// PanelHost is a screen with a panel (the player's `.tpanel`): the Shell tells
// it when Menu closes the panel, so that it takes the panel away, as Back
// does (index.html:731).
type PanelHost interface {
	// PanelClosed is the panel closing without the screen asking for it.
	PanelClosed()
}

// Holder is a screen that lets a native plane show through a rectangle of its
// own (the player's video, K5): the Shell's stage leaves that rectangle
// unpainted, since an opaque fill under the view would hide the plane.
type Holder interface {
	// Hole is the rectangle, in window coordinates, that stays transparent;
	// the empty rectangle when there is none.
	Hole() geom.Rect
}

// Env is what a screen is built with: the services of the app, the texts, the
// actions and the navigation state. The Shell makes one and passes it to the
// factory of every route.
type Env struct {
	Services
	// Act is the Shell's actions.
	Act Actions
	// Stack is the navigation state, for the screens that read it.
	Stack *nav.Stack

	texts   map[string]string
	prefs   Prefs                   // the in-memory preferences of a Services that has none
	courses map[string]model.Course // the courses a screen read, for the screens that walk their lessons

	upNext    int  // the seconds the up-next countdown has left (`cdT`)
	upNextSet bool // upNext holds a value; else the countdown is at its start
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns an Env over the services and the texts of locales/vi.json.
func New(s Services, texts map[string]string) *Env {
	return &Env{Services: s, texts: texts, Stack: nav.NewStack()}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// T returns the text of key; a key the locale lacks shows as the key itself,
// where a missing string cannot be overlooked.
func (e *Env) T(key string) string {
	if s, ok := e.texts[key]; ok {
		return s
	}

	return key
}

// Fill returns the template of key with each {name} replaced by the value that
// follows it in kv (name, value, name, value ...).
func (e *Env) Fill(key string, kv ...string) string { return Fill(e.T(key), kv...) }

// CachedCourse returns the course a screen read last (RememberCourse), false
// when none was: the player's list of next lessons and the up-next screen read
// the lessons of the course the player holds from here, as the prototype reads
// them from one table (`lessonsOf`).
func (e *Env) CachedCourse(id string) (model.Course, bool) {
	c, ok := e.courses[id]

	return c, ok
}

// RememberCourse keeps c as the last read of its course.
func (e *Env) RememberCourse(c model.Course) {
	if c.ID == "" {
		return
	}
	if e.courses == nil {
		e.courses = make(map[string]model.Course, 4)
	}
	e.courses[c.ID] = c
}

// UpNextCount is the seconds the up-next countdown starts at, `cdT=5` (index.html:782).
const UpNextCount = 5

// UpNextLeft returns the seconds the up-next countdown has left. It is one number
// for the app, as the prototype's `cdT` is: a screen left and come back to goes on
// from it.
func (e *Env) UpNextLeft() int {
	if !e.upNextSet {
		return UpNextCount
	}

	return e.upNext
}

// SetUpNextLeft keeps the seconds left, for when the up-next screen is shown again.
func (e *Env) SetUpNextLeft(n int) { e.upNext, e.upNextSet = n, true }

// ResetUpNext starts the countdown over at 5: a video ended (`cdT=5`).
func (e *Env) ResetUpNext() { e.upNext, e.upNextSet = UpNextCount, true }

// Prefs returns the preferences: the services' when they have them, else the
// defaults of a fresh app kept in memory (auto next on, quality auto).
func (e *Env) Prefs() Prefs {
	if p := e.Services.Prefs(); p != nil {
		return p
	}
	if e.prefs == nil {
		e.prefs = &memPrefs{auto: true, q: store.QualityAuto}
	}

	return e.prefs
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// memPrefs is the Prefs of a Services that has none.
type memPrefs struct {
	auto bool
	q    store.Quality
}

func (m *memPrefs) AutoNext() bool         { return m.auto }
func (m *memPrefs) SetAutoNext(on bool)    { m.auto = on }
func (m *memPrefs) Quality() store.Quality { return m.q }

// CycleQualityTV is Tự động, 1080p, 720p, Tự động: a stored 480p goes to Tự động.
func (m *memPrefs) CycleQualityTV() {
	switch m.q {
	case store.QualityAuto:
		m.q = store.QualityP1080
	case store.QualityP1080:
		m.q = store.QualityP720
	case store.QualityUnknown, store.QualityP720, store.QualityP480:
		m.q = store.QualityAuto
	default:
		m.q = store.QualityAuto
	}
}

// Fill replaces each "{name}" of tpl by the value after name in kv. A value that is
// empty takes its " · " part of the line with it (format.DropBlank).
func Fill(tpl string, kv ...string) string {
	if len(kv) < 2 || !strings.Contains(tpl, "{") {
		return tpl
	}
	pairs := make([]string, 0, len(kv))
	empty := false
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, "{"+kv[i]+"}", kv[i+1])
		empty = empty || kv[i+1] == ""
	}
	out := strings.NewReplacer(pairs...).Replace(tpl)
	if empty { // a lesson with no tag: no stray " · " where its part would be
		out = format.DropBlank(out)
	}

	return out
}
