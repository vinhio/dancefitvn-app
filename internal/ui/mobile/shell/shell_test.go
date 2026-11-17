//go:build !tvos

package shell_test

import (
	"fmt"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ms is a millisecond.
const ms = time.Millisecond

// bootCase is one row of the boot table: what the session read and the app
// config do, and where the app is after each time.
type bootCase struct {
	name     string
	signed   bool
	slowCfg  bool          // the app config is held
	slowRest bool          // the session read is held
	cfg426   bool          // the app config answers 426
	steps    []bootStep    // the times, in order, and the route after each
	release  time.Duration // when what is held is let go (0: never)
	after    string        // the route a moment after the release
}

// bootStep is a time since the launch and the route showing then.
type bootStep struct {
	at    time.Duration
	route string
}

// TestBootRoutes runs the boot of docs/plan/03 section 1 on both phones: the splash
// stays at least 600 ms and, while the app config is not back, at most 1300; the
// session read is always waited for; a signed session goes Home, none to the
// login; a 426 goes to the upgrade screen, also when it comes late.
func TestBootRoutes(t *testing.T) {
	t.Parallel()

	tests := []bootCase{
		{name: "unsigned, config fast", steps: []bootStep{{599 * ms, "splash"}, {600 * ms, "login"}}},
		{name: "signed, config fast", signed: true, steps: []bootStep{{599 * ms, "splash"}, {600 * ms, "home"}}},
		{
			name: "config slow: the splash goes at 1300", slowCfg: true,
			steps: []bootStep{{600 * ms, "splash"}, {1299 * ms, "splash"}, {1300 * ms, "login"}},
		},
		{
			name: "signed, config slow", signed: true, slowCfg: true,
			steps: []bootStep{{1299 * ms, "splash"}, {1300 * ms, "home"}},
		},
		{
			name: "session slow: waited for past 1300", slowRest: true,
			steps:   []bootStep{{600 * ms, "splash"}, {1300 * ms, "splash"}, {5000 * ms, "splash"}},
			release: 5000 * ms, after: "login",
		},
		{name: "426 goes to the upgrade screen", cfg426: true, steps: []bootStep{{599 * ms, "splash"}, {600 * ms, "upgrade"}}},
		{
			name: "426 that comes late", cfg426: true, slowCfg: true,
			steps: []bootStep{{1300 * ms, "login"}}, release: 1400 * ms, after: "upgrade",
		},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f, err := shelltest.New(shelltest.MockFS(t))
				require.NoError(t, err)
				if tc.signed {
					require.NoError(t, f.SeedSession())
				}
				if tc.cfg426 {
					require.NoError(t, f.SetScenario("app.config", "upgrade_required"))
				}
				var releases []func()
				if tc.slowCfg {
					releases = append(releases, f.Gate("app.config"))
				}
				if tc.slowRest {
					releases = append(releases, f.HoldRestore())
				}
				r := shelltest.MountRig(t, ph, false, f, shell.Options{})
				start := r.Tester.Now() // the boot starts with the mount's first frame
				// RunUntilIdle would wait for a held read too: wait for the others.
				r.WaitFor(func() bool {
					b := r.Shell.BootState()

					return (b.Restored || tc.slowRest) && (b.ConfigDone || tc.slowCfg)
				}, "the reads that are not held")
				t.Cleanup(func() { // a read still held would stop the tester from closing
					for _, rel := range releases {
						rel()
					}
				})
				for _, st := range tc.steps {
					r.Tester.Elapse(st.at - (r.Tester.Now() - start))
					assert.Equal(t, st.route, r.Route(), "after %v", st.at)
				}
				if tc.release > 0 {
					r.Tester.Elapse(tc.release - (r.Tester.Now() - start))
					for _, rel := range releases {
						rel()
					}
					r.WaitFor(func() bool {
						b := r.Shell.BootState()

						return b.Restored && b.ConfigDone && r.Route() == tc.after
					}, "the route after the release")
					assert.Equal(t, tc.after, r.Route(), "after the release")
				}
				if tc.signed {
					assert.Equal(t, 0, r.Shell.Nav().Depth(), "the stack is empty")
				}
			})
		}
	})
}

// TestBootHoldKeepsTheSplash checks M01's mode: with HoldSplash the boot runs and
// the splash stays.
func TestBootHoldKeepsTheSplash(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := shelltest.NewRig(t, ph, false, shell.Options{HoldSplash: true})
		r.Settle()
		r.Tester.Elapse(3 * time.Second)
		r.Settle()
		assert.Equal(t, "splash", r.Route())
	})
}

// TestSignOutShowsTheLoginWithTheExpiredBanner checks that a session the server
// ended takes the app to the login, the stack empty, with "Phiên đăng nhập đã hết
// hạn" over the form (03 section 9).
func TestSignOutShowsTheLoginWithTheExpiredBanner(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Boot()
		require.Equal(t, "home", r.Route())
		f.SignOut(api.ErrSignedOut)
		r.Settle()
		assert.Equal(t, "login", r.Route())
		assert.Equal(t, 0, r.Shell.Nav().Depth())
		assert.True(t, r.Tester.Shows("Phiên đăng nhập đã hết hạn"))
		assert.True(t, r.Tester.Shows("Đăng nhập lại để tiếp tục"))
		assert.False(t, f.Signed())
	})
}

// TestTabsGoToTheirRoots checks that the four tabs of the tab bar open their
// routes as roots, the stack empty (`data-root`), stubs for now.
func TestTabsGoToTheirRoots(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Boot()
		require.Equal(t, "home", r.Route())
		// the tabs are tapped by place: the screens under the bar have words of their own
		// with the same names ("Khoá học" is Home's stat label and section link)
		for i, tab := range []struct {
			label, route string
			col          float32
		}{
			{"Khoá học", "courses", 1}, {"Tải xuống", "downloads", 2}, {"Tài khoản", "account", 3}, {"Trang chủ", "home", 0},
		} {
			bar := r.Shell.Nav().TabBar().Node().WindowFrame()
			r.Tester.ClickAt(geom.Pt(bar.X+(tab.col+.5)*bar.W/4, bar.Y+bar.H*.4))
			r.Settle()
			assert.Equal(t, tab.route, r.Route(), "%d %s", i, tab.label)
			assert.Equal(t, 0, r.Shell.Nav().Depth(), tab.label)
		}
	})
}

// TestBackLeavesTheAppAtTheLoginAndHome checks Android's Back: at the login, at
// Home and at the splash nothing is left to close, so the key is not taken and the
// system sends the app to the background; a route over Home goes back to it.
func TestBackLeavesTheAppAtTheLoginAndHome(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Settle()
		assert.False(t, r.Tester.Key(keys.KeyBack, 0), "at the splash")
		r.Tester.Elapse(600 * ms)
		r.Settle()
		require.Equal(t, "login", r.Route())
		assert.False(t, r.Tester.Key(keys.KeyBack, 0), "at the login")

		r.Shell.SignedIn()
		r.Settle()
		require.Equal(t, "home", r.Route())
		assert.False(t, r.Tester.Key(keys.KeyBack, 0), "at Home")
		r.Shell.Detail("set-35")
		r.Settle()
		require.Equal(t, "detail", r.Route())
		assert.True(t, r.Tester.Key(keys.KeyBack, 0), "a detail goes back")
		assert.Equal(t, "home", r.Route())
	})
}

// TestActionsOffline checks the offline branches of 03 section 2 and 3: the detail of a
// course opens the offline screen, playing a video and downloading toast their
// texts for 2200 ms.
func TestActionsOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Boot()
		f.SetOnline(false)
		r.Shell.Detail("set-35")
		r.Settle()
		assert.Equal(t, "offline", r.Route(), "03 section 2: the detail while offline")

		r.Shell.PlayVideo("set-35", "l_3503", 3)
		r.Tester.Elapse(100 * ms)
		assert.True(t, r.Tester.Shows("Video bài 03 cần có mạng để xem"))
		r.Tester.Elapse(2200 * ms)
		assert.False(t, r.Tester.Shows("Video bài 03 cần có mạng để xem"), "the toast ends after 2200 ms")

		r.Shell.DlAll("set-35")
		r.Tester.Elapse(100 * ms)
		assert.True(t, r.Tester.Shows("Không có mạng để tải"))
		r.Tester.Elapse(2300 * ms)
		r.Shell.DlTap("set-35", "l_3505", 5)
		r.Tester.Elapse(100 * ms)
		assert.True(t, r.Tester.Shows("Không có mạng để tải"))

		r.Tester.Elapse(2300 * ms)
		r.Shell.OffTap("set-35", "l_3509", 9)
		r.Tester.Elapse(100 * ms)
		assert.True(t, r.Tester.Shows("Video bài 09 cần có mạng để xem"), "a lesson without its audio")
	})
}

// TestOnlineSwapsDetailAndOffline checks 03 section 2: while on the detail of a course the
// network going away puts its offline screen in its place, the stack as it was,
// and its return puts the detail back.
func TestOnlineSwapsDetailAndOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Boot()
		r.Shell.Detail("set-35")
		r.Settle()
		require.Equal(t, "detail", r.Route())
		depth := r.Shell.Nav().Depth()
		f.SetOnline(false)
		r.Settle()
		assert.Equal(t, "offline", r.Route())
		assert.Equal(t, "set-35", r.Shell.Nav().Current().Params)
		assert.Equal(t, depth, r.Shell.Nav().Depth())
		f.SetOnline(true)
		r.Settle()
		assert.Equal(t, "detail", r.Route())
		assert.Equal(t, "set-35", r.Shell.Nav().Current().Params)
	})
}

// TestMidSessionUpgradeRequiredBlocks checks 03 section 9 on both phones: a 426 on
// the read of any screen (Home, Courses, a course) replaces the stack with the
// blocking upgrade screen, whose button opens the store page of the error's
// body, not the website.
func TestMidSessionUpgradeRequiredBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		route string // the API route that answers 426
		boot  bool   // the failure is set before the boot (Home reads then)
		open  func(r *shelltest.Rig)
	}{
		{"me.home", "me.home", true, func(*shelltest.Rig) {}},
		{"me.courses", "me.courses", false, func(r *shelltest.Rig) { r.Shell.Go(nav.Route{Screen: env.RouteCourses}, true) }},
		{"courses.show", "courses.show", false, func(r *shelltest.Rig) { r.Shell.Detail("c_1") }},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f, err := shelltest.New(shelltest.MockFS(t))
				require.NoError(t, err)
				require.NoError(t, f.SeedSession())
				if tc.boot {
					f.Fail(tc.route, 426, "errors/upgrade_required.json")
				}
				r := shelltest.MountRig(t, ph, false, f, shell.Options{})
				r.Boot()
				if !tc.boot {
					require.Equal(t, env.RouteHome, r.Route())
					f.Fail(tc.route, 426, "errors/upgrade_required.json")
					tc.open(r)
					r.Settle() // the screen starts its read with its first frame
					r.Tester.Elapse(time.Millisecond)
					r.Settle()
				}
				require.Equal(t, env.RouteUpgrade, r.Route())
				assert.Equal(t, 0, r.Shell.Nav().Depth())
				assert.True(t, f.Signed(), "a 426 does not sign out")
				up, ok := r.Shell.Nav().Screen().(*screens.Upgrade)
				require.True(t, ok)
				up.Button().OnPress()
				assert.Equal(t, []string{"https://apps.apple.com/"}, f.OpenedURLs())
			})
		}
	})
}

// TestUpgradeOf checks which errors are a refused version.
func TestUpgradeOf(t *testing.T) {
	t.Parallel()

	store := map[string]string{"ios": "u"}
	tests := []struct {
		name string
		err  error
		ok   bool
	}{
		{"by code", &api.Error{Status: 426, Code: "upgrade_required", Body: model.APIError{StoreURL: store}}, true},
		{"by status", &api.Error{Status: 426, Code: "http_426"}, true},
		{"wrapped", fmt.Errorf("read: %w", &api.Error{Status: 426, Code: "upgrade_required"}), true},
		{"another error", &api.Error{Status: 500, Code: "server_error"}, false},
		{"offline", api.ErrOffline, false},
		{"none", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, ok := env.UpgradeOf(tc.err)
			assert.Equal(t, tc.ok, ok)
			if tc.ok && tc.name == "by code" {
				assert.Equal(t, store, p.StoreURL)
			}
		})
	}
}

// TestUpgradeUnused keeps the env import honest: the upgrade params type is the
// route's.
func TestUpgradeUnused(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "upgrade", env.RouteUpgrade)
}
