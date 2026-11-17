//go:build !tvos

package shell

import (
	"context"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The boot's times (docs/plan/03 section 1, Q-P3-22): the splash shows for at
// least bootMin and, while the app config is not back, at most bootMax; the
// session read is always waited for.
const (
	bootMin = 600 * time.Millisecond
	bootMax = 1300 * time.Millisecond
)

// Options are what New asks besides the services.
type Options struct {
	// HoldSplash keeps the splash on the screen, its bar at the end: the state
	// M01 of the shot script. The boot still runs but does not leave it.
	HoldSplash bool
}

// Shell is the root widget of the phone: mount it as the root of the view. It
// takes the system's Back key for the navigator.
type Shell struct {
	ui.Base

	e    *env.Env
	nav  *nav.Nav
	opts Options

	started  bool
	stops    []func()
	restored bool
	signed   bool
	cfgDone  bool
	upgrade  *env.UpgradeParams // set by a 426 or force_update
	minDone  bool
	maxDone  bool
	routed   bool
	// waitToasted is "Đang chờ Wi‑Fi để tải" having been shown for the downloads
	// held back by the rule Wi-Fi only; it is set again once none is.
	waitToasted bool
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
// opens the splash at once; the boot goes on when the Shell is mounted.
func New(svc env.Services, texts map[string]string, o Options) *Shell {
	s := &Shell{opts: o}
	s.e = env.New(svc, texts)
	s.e.Act = s
	s.nav = nav.New(nav.Options{
		Home:    nav.Route{Screen: env.RouteHome},
		Tabs:    s.tabs(),
		Factory: s.build,
	})
	s.e.Nav = s.nav
	s.Add(s.nav)
	s.nav.Go(nav.Route{Screen: env.RouteSplash}, true)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Nav returns the navigator.
func (s *Shell) Nav() *nav.Nav { return s.nav }

// Env returns the environment the screens are built with.
func (s *Shell) Env() *env.Env { return s.e }

// BootState is how far the boot has come, for the shot script and the tests.
type BootState struct {
	// Restored is the session read being back, ConfigDone the app config's answer
	// (or its failure), Routed the splash gone.
	Restored, ConfigDone, Routed bool
}

// BootState returns how far the boot has come.
func (s *Shell) BootState() BootState {
	return BootState{Restored: s.restored, ConfigDone: s.cfgDone, Routed: s.routed}
}

// Mounted starts the boot: the session and the app config are read at once, in
// parallel; the splash leaves once the session is read and 600 ms have passed,
// at 1300 ms at the latest unless the session is still being read.
func (s *Shell) Mounted(c *ui.Context) {
	if s.started {
		return
	}
	s.started = true
	s.stops = append(s.stops, s.e.OnSignedOut(s.signedOut), s.e.WatchOnline(s.onlineChanged), s.e.WatchForeground(s.foregrounded))
	if stop := s.watchPlayback(); stop != nil {
		s.stops = append(s.stops, stop)
	}
	s.stops = append(s.stops, s.watchDownloads())
	// A view that is being mounted has no host yet, and ui.Go would run the work
	// on the UI thread then: the boot starts with the first frame.
	c.AfterFunc(0, func() { s.boot(c) })
}

// boot reads the session and the app config, off the UI thread, and sets the
// splash's two times.
func (s *Shell) boot(c *ui.Context) {
	ui.Go(s, func(ctx context.Context) (bool, error) {
		return s.e.Restore(ctx)
	}, func(_ *ui.Context, signed bool, err error) {
		s.restored, s.signed = true, signed && err == nil
		s.tryRoute()
	})
	ui.Go(s, func(ctx context.Context) (model.AppConfig, error) {
		return s.e.Config(ctx)
	}, func(_ *ui.Context, cfg model.AppConfig, err error) { s.configDone(cfg, err) })
	c.AfterFunc(bootMin, func() { s.minDone = true; s.tryRoute() })
	c.AfterFunc(bootMax, func() { s.maxDone = true; s.tryRoute() })
}

// Unmounted ends the watches of the services.
func (s *Shell) Unmounted(*ui.Context) {
	for _, stop := range s.stops {
		stop()
	}
	s.stops, s.started = nil, false
}

// InterceptKey gives the Back key to the navigator (nav.Nav.InterceptKey).
func (s *Shell) InterceptKey(c *ui.Context, e *ui.KeyEvent) bool { return s.nav.InterceptKey(c, e) }

// Layout gives the navigator the whole window.
func (s *Shell) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(s.nav, ui.Tight(cs.Max))
	c.Place(s.nav, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tabs returns the four tabs of the tab bar (TABS, index.html:323).
func (s *Shell) tabs() [kit.TabCount]nav.TabRoute {
	t := func(ic icon.Name, label, name, route string) nav.TabRoute {
		return nav.TabRoute{
			Item:  kit.TabItem{Icon: ic, Label: s.e.T(label), Name: name},
			Route: nav.Route{Screen: route},
		}
	}

	return [kit.TabCount]nav.TabRoute{
		t(icon.Home, "tabs.home", "go.home", env.RouteHome),
		t(icon.Grid, "tabs.courses", "go.courses", env.RouteCourses),
		t(icon.Dl, "tabs.downloads", "go.downloads", env.RouteDownloads),
		t(icon.User, "tabs.account", "go.account", env.RouteAccount),
	}
}

// tryRoute leaves the splash when the boot is ready: the session is read and
// 600 ms have passed, with the app config back or 1300 ms gone. A 426 or
// force_update goes to the upgrade screen, a session to Home, none to the login.
func (s *Shell) tryRoute() {
	if s.routed || s.opts.HoldSplash || !s.restored || !s.minDone || !(s.cfgDone || s.maxDone) {
		return
	}
	s.routed = true
	switch {
	case s.upgrade != nil:
		s.nav.Go(nav.Route{Screen: env.RouteUpgrade, Params: *s.upgrade}, true)
	case s.signed:
		s.nav.Go(nav.Route{Screen: env.RouteHome}, true)
	default:
		s.nav.Go(nav.Route{Screen: env.RouteLogin}, true)
	}
}

// configDone takes the answer of GET /app/config: the config is kept; a 426 or
// force_update makes the upgrade screen the way on, at once when the splash is
// already gone. Any other failure (no network at launch) leaves the app running
// without the config.
func (s *Shell) configDone(cfg model.AppConfig, err error) {
	s.cfgDone = true
	if p, ok := env.UpgradeOf(err); ok {
		s.upgrade = &p
	} else if err == nil {
		s.e.SetConfig(cfg)
		if cfg.ForceUpdate {
			s.upgrade = &env.UpgradeParams{}
		}
	}
	if s.routed && s.upgrade != nil && s.nav.Current().Screen != env.RouteUpgrade {
		s.nav.Go(nav.Route{Screen: env.RouteUpgrade, Params: *s.upgrade}, true)

		return
	}
	s.tryRoute()
}

// signedOut is the session ended by the server: the login, with the banner that
// says so (03 section 9), the stack empty.
func (s *Shell) signedOut(error) {
	// Already at the login (the user's own sign-out, or a late refusal of a
	// request that left before it): no expiry to tell of.
	if s.nav.Current().Screen == env.RouteLogin {
		return
	}
	s.e.ResetAccount()
	s.nav.Go(nav.Route{Screen: env.RouteLogin, Params: env.LoginParams{Banner: env.LoginBannerExpired}}, true)
}

// onlineChanged is the network going or coming back (03 section 2): the detail
// of a course gives way to its offline screen and back, a video stops and gives
// way to the offline screen of its course (`$P:810-813`, F-13), and the screen
// showing is told, for its offline bar. Audio (08) goes on: its file may be on
// the device.
func (s *Shell) onlineChanged(online bool) {
	cur := s.nav.Current()
	switch {
	case !online && cur.Screen == env.RouteDetail:
		s.nav.Replace(nav.Route{Screen: env.RouteOffline, Params: cur.Params})
	case !online && cur.Screen == env.RouteVideo:
		s.offlineFromVideo()
	case online && cur.Screen == env.RouteOffline:
		s.nav.Replace(nav.Route{Screen: env.RouteDetail, Params: cur.Params})
	default: // no change of screen for this change of network
	}
	if oa, ok := s.nav.Screen().(env.OnlineAware); ok {
		oa.SetOnline(online)
	}
}

// foregrounded is the app coming back to the front: the screen showing is told, to
// read again what the system cut while the app was away.
func (s *Shell) foregrounded() {
	if fa, ok := s.nav.Screen().(env.ForegroundAware); ok {
		fa.Foregrounded()
	}
}

// offlineFromVideo is the network going while 06 shows: the player pauses and the
// offline screen of the lesson's course takes its place, the stack as it was, so
// that Back leaves for where the video was opened from. With no player the screen
// is a stub and stays.
func (s *Shell) offlineFromVideo() {
	pb := s.e.Playback()
	if pb == nil {
		return
	}
	pb.Pause()
	s.nav.Replace(nav.Route{Screen: env.RouteOffline, Params: pb.Lesson().CourseID})
}
