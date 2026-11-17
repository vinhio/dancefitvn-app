//go:build tvos

package shell

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Keys the focus is asked for by name: the first `.f` of the settings screen is
// a nav item (renderTV, index.html:723), and closing the panel focuses `p2`
// (tvBack, :731).
const (
	keySettingsFirst = "nav0"
	keyPanelClose    = "p2"
)

// Shell is the root widget of the TV: mount it as the root of the view, made
// WithDirectionalFocus and WithTransparentBackground. It takes the Siri
// Remote's keys for the focus layer and carries out env.Actions for the
// screens.
type Shell struct {
	ui.Base

	e     *env.Env
	eng   *focus.ViewEngine
	m     *focus.Manager
	root  *focus.Root
	stage *stage
	host  *host
	toast *tvkit.Toast

	screen ui.Widget // the screen of the route showing

	// pending is the focus owed after a change the next layout settles
	// (renderTV: `cur` found by key or by the default rule).
	pending bool
	prefer  string // the key the focus is asked for, "" for the default rule

	stops []func()
}

var (
	_ ui.Mounter        = (*Shell)(nil)
	_ ui.Unmounter      = (*Shell)(nil)
	_ ui.KeyInterceptor = (*Shell)(nil)
	_ env.Actions       = (*Shell)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns the Shell over the services and the texts of locales/vi.json. It
// shows nothing until Go is called with the first route (the welcome screen or
// the home, once the session is read).
func New(svc env.Services, texts map[string]string) *Shell {
	s := &Shell{e: env.New(svc, texts)}
	s.e.Act = s
	s.eng = focus.NewViewEngine(nil)
	s.m = focus.NewManager(s.eng, s.eng)
	s.stage, s.host, s.toast = &stage{}, &host{}, tvkit.NewToast()
	s.stage.Add(s.host, s.toast)
	s.root = focus.NewRoot(s.m, s.stage)
	s.Add(s.root)
	s.stops = append(s.stops, s.m.OnMenu(s.menu))

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Env returns the environment the screens are built with.
func (s *Shell) Env() *env.Env { return s.e }

// Manager returns the focus manager.
func (s *Shell) Manager() *focus.Manager { return s.m }

// Stack returns the navigation state.
func (s *Shell) Stack() *nav.Stack { return s.e.Stack }

// Screen returns the widget of the route showing, nil before the first.
func (s *Shell) Screen() ui.Widget { return s.screen }

// ToastHost returns the toast widget.
func (s *Shell) ToastHost() *tvkit.Toast { return s.toast }

// TextShift1x is the DIPs the text origin moves left of layout at the TV's 1x
// (1080p sim and Tester). The phones' -0.5 was measured at 3x; at 1x it leaves the
// app's ink 0.4 to 0.6 px left of Chrome's, and -0.1 brings the median to 0.0 (the
// mean to 0.02) over the 31 text boxes of TV04, TV06 and TV12 that tools/inkshift.py
// matches (0 gave -0.1, -0.25 gave 0.3, -0.5 gave 0.6). 2x has no capture of the
// prototype to measure against and keeps the default.
const TextShift1x float32 = -0.1

// Mounted binds the focus layer to the view and sets the text shift for the
// view's scale, once.
func (s *Shell) Mounted(c *ui.Context) {
	if c != nil && c.Scale() <= 1 {
		kit.SetTextShift(TextShift1x)
	}
	if v := s.View(); v != nil {
		s.eng.Bind(v)
	}
}

// Unmounted takes the Manager off the view and ends the chains.
func (s *Shell) Unmounted(*ui.Context) {
	for _, stop := range s.stops {
		stop()
	}
	s.stops = nil
	s.m.Close()
}

// InterceptKey hands the keys to the focus Root: with nothing focused only the
// view's root sees them.
func (s *Shell) InterceptKey(c *ui.Context, e *ui.KeyEvent) bool {
	return s.root.InterceptKey(c, e)
}

// Layout fills the room with the Root and, once the screens are laid out,
// puts the focus where a change of screen asked for it.
func (s *Shell) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(s.root, ui.Tight(size))
	c.Place(s.root, geom.Point{})
	s.settle()

	return size
}

// Go is tvGo (index.html:730): the route becomes the screen, the one showing
// is pushed unless root is set, the focus goes to the default of the new
// screen and the panel closes. Leaving for a screen that is not the player or
// the music pauses the playback.
func (s *Shell) Go(r nav.Route, root bool) {
	s.e.Stack.Go(r, root)
	if r.Screen != env.RoutePlayer && r.Screen != env.RouteMusic {
		s.pause()
	}
	s.show(r, "")
}

// Replace puts r in the place of the screen showing, as the player and the
// up-next screen take each other's place.
func (s *Shell) Replace(r nav.Route) {
	s.e.Stack.Replace(r)
	s.show(r, "")
}

// Back is tvBack (index.html:731): the panel closes and the focus goes to `p2`,
// else the stack is popped and the playback pauses. It tells whether it did
// anything; false leaves Menu to the system.
func (s *Shell) Back() bool {
	if s.e.Stack.Panel() {
		if h, ok := s.screen.(env.PanelHost); ok {
			h.PanelClosed()
		}
		s.ClosePanel(keyPanelClose)

		return true
	}
	r, ok := s.e.Stack.Pop()
	if !ok {
		return false
	}
	s.pause()
	s.show(r, "")

	return true
}

// SwitchTab is Select on a nav item (tvActivate, index.html:735-736): the screen
// of item i takes the place of the one showing, the stack empties, the focus
// stays on the item, and the playback goes on only for the music.
func (s *Shell) SwitchTab(i int) {
	if i < 0 || i >= len(env.Tabs) {
		return
	}
	r := nav.Route{Screen: env.Tabs[i]}
	s.e.Stack.Go(r, true)
	if r.Screen != env.RouteMusic {
		s.pause()
	}
	s.show(r, tvkit.NavKey(i))
}

// Toast is tvToast (index.html:732).
func (s *Shell) Toast(msg string) { s.toast.Show(msg) }

// OpenPanel is `topts` (index.html:742): the panel opens, the focus is confined
// to it and goes to its first item.
func (s *Shell) OpenPanel(panel ui.Widget) {
	s.e.Stack.SetPanel(true)
	s.m.SetScope(panel)
	s.m.Clear()
	s.pending, s.prefer = true, ""
	s.screenChanged()
	s.Relayout()
}

// ClosePanel is `tclose` (index.html:742): the panel closes and the focus goes
// to the item with focusKey.
func (s *Shell) ClosePanel(focusKey string) {
	s.e.Stack.SetPanel(false)
	s.m.SetScope(nil)
	s.pending, s.prefer = true, focusKey
	s.screenChanged()
	s.Relayout()
}

// OnPlayPause adds a Play/Pause handler to the chain.
func (s *Shell) OnPlayPause(fn func()) (remove func()) { return s.m.OnPlayPause(fn) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// menu is the Menu chain's one handler: the panel closes, else the stack goes
// back, else nothing and the press is left to the system.
func (s *Shell) menu() bool { return s.Back() }

// pause pauses the playback, `S.pt.playing=false`.
func (s *Shell) pause() {
	if pb := s.e.Playback(); pb != nil && pb.Playing() {
		pb.Pause()
	}
}

// show builds the screen of r in the place of the one showing; the focus is
// owed to the next layout, on the item with the key prefer or, with none, by
// the default rule.
func (s *Shell) show(r nav.Route, prefer string) {
	if s.screen != nil {
		s.host.Remove(s.screen)
		s.screen = nil
	}
	s.m.SetScope(nil)
	s.m.Clear()
	w := s.build(r)
	s.screen = w
	s.host.Add(w)
	if prefer == "" && r.Screen == env.RouteSettings {
		prefer = keySettingsFirst
	}
	s.pending, s.prefer = true, prefer
	s.screenChanged()
	s.Relayout()
}

// screenChanged tells a screen reader that another screen shows, so that it reads
// from the first element again: a route was entered, left or replaced, a tab
// switched, the panel of the player opened or closed. VoiceOver on tvOS follows
// the focus, which settle gives to the new screen next. It does nothing before
// the shell is in a view, and costs nothing until a screen reader has read the
// tree. Adapted from jplayer-tv/internal/ui/shell/describe.go at 781bee5
// (Shell.screenChanged).
func (s *Shell) screenChanged() {
	if v := s.View(); v != nil {
		v.Context().SemanticsChanged(platform.ChangeScreen)
	}
}

// settle gives the focus its place when a change owes it: renderTV's rule
// (index.html:722-724). The candidates exist once the screens are laid out.
func (s *Shell) settle() {
	if !s.pending {
		return
	}
	s.pending = false
	s.m.Initial(s.m.Scope(), s.prefer)
}

// build makes the screen of a route. A route nobody knows is the welcome screen.
func (s *Shell) build(r nav.Route) ui.Widget {
	switch r.Screen {
	case env.RouteWelcome:
		return screens.NewWelcome(s.e)
	case env.RouteCode:
		return screens.NewCode(s.e)
	case env.RouteHome:
		return screens.NewHome(s.e)
	case env.RouteCourses:
		return screens.NewCourses(s.e)
	case env.RouteDetail:
		id, _ := r.Params.(string) // the course id (`S.tv.set`)

		return screens.NewDetail(s.e, id)
	case env.RoutePlayer:
		return screens.NewPlayer(s.e)
	case env.RouteMusic:
		return screens.NewMusic(s.e)
	case env.RouteUpNext:
		return screens.NewUpNext(s.e)
	case env.RouteError:
		return screens.NewError(s.e)
	case env.RouteSettings:
		return screens.NewSettings(s.e)
	default:
		return screens.NewWelcome(s.e)
	}
}
