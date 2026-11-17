//go:build !tvos

package shell_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atHome signs in and boots to Home over the Fake.
func atHome(t *testing.T, ph kittest.Phone) *shelltest.Rig {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	r := shelltest.MountRig(t, ph, false, f, shell.Options{})
	r.Boot()
	require.Equal(t, "home", r.Route())

	return r
}

// stubOf returns the placeholder screen showing and the route it stands for.
func stubOf(t *testing.T, r *shelltest.Rig) *screens.Stub {
	t.Helper()
	s, ok := r.Shell.Nav().Screen().(*screens.Stub)
	require.True(t, ok, "a placeholder is showing")

	return s
}

// sheetID returns the id of the placeholder sheet that is open.
func sheetID(t *testing.T, r *shelltest.Rig) string {
	t.Helper()
	require.True(t, r.Shell.Nav().Sheet().IsOpen(), "a sheet is open")
	if _, ok := r.Shell.Nav().Sheet().Content().(*screens.AvatarSheet); ok { // SH.avatar is real (P6.3b)
		return env.SheetAvatar
	}
	if _, ok := r.Shell.Nav().Sheet().Content().(*screens.DevicesSheet); ok { // the sheet of devices is real (P6.4)
		return env.SheetDevices
	}
	s, ok := r.Shell.Nav().Sheet().Content().(*screens.StubSheet)
	require.True(t, ok, "a placeholder sheet")

	return s.ID
}

// TestAccountRoutes checks the routes of P6: the tab "Tài khoản" opens screen 15
// with the tab bar on its tab, and the screens 15B, 15C and 16 open from it without
// one, each on its own route, Back leaving for 15. All four are real screens
// (P6.1, P6.2, P6.4).
func TestAccountRoutes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph)
		require.NoError(t, r.Tester.Click("Tài khoản"))
		r.Settle()
		require.Equal(t, env.RouteAccount, r.Route())
		acct, ok := r.Shell.Nav().Screen().(*screens.Account)
		require.True(t, ok, "screen 15")
		assert.Equal(t, 3, acct.Tab(), "the tab bar's fourth tab")
		assert.Equal(t, 0, r.Shell.Nav().Depth(), "a tab is a root")

		for _, route := range []string{env.RouteInfo, env.RouteCoach, env.RouteConnect} {
			r.Shell.Go(nav.Route{Screen: route}, false)
			r.Settle()
			require.Equal(t, route, r.Route())
			switch scr := r.Shell.Nav().Screen().(type) {
			case *screens.Info:
				assert.Equal(t, env.RouteInfo, route)
				assert.Equal(t, -1, scr.Tab(), route+" has no tab bar")
			case *screens.Coach:
				assert.Equal(t, env.RouteCoach, route)
				assert.Equal(t, -1, scr.Tab(), route+" has no tab bar")
			case *screens.Connect:
				assert.Equal(t, env.RouteConnect, route)
				assert.Equal(t, -1, scr.Tab(), route+" has no tab bar")
			default:
				s := stubOf(t, r)
				assert.Equal(t, route, s.Route)
				assert.Equal(t, -1, s.Tab(), route+" has no tab bar")
			}
			r.Shell.Back()
			r.Settle()
			assert.Equal(t, env.RouteAccount, r.Route(), "Back from "+route)
		}
	})
}

// TestAccountSheets checks the actions that open the sheets of the account, and
// that "Huỷ" closes each of them.
func TestAccountSheets(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		tests := []struct {
			name string
			open func(s *shell.Shell)
			id   string
		}{
			{"avatar", func(s *shell.Shell) { s.AskAvatar() }, env.SheetAvatar},
			{"devices", func(s *shell.Shell) {
				s.AskDevices(env.DevicesSheetParams{Code: "7K4M2P", Devices: []model.Device{{ID: "d_201"}}})
			}, env.SheetDevices},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r := atHome(t, ph)
				tc.open(r.Shell)
				r.Settle()
				r.Tester.Elapse(400 * ms) // the sheet slides in
				assert.Equal(t, tc.id, sheetID(t, r))
				require.NoError(t, r.Tester.Click("Huỷ"))
				r.Tester.Elapse(400 * ms)
				assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			})
		}
		t.Run("password", func(t *testing.T) {
			r := atHome(t, ph)
			r.Shell.AskPassword()
			r.Settle()
			r.Tester.Elapse(400 * ms) // the sheet slides in
			require.True(t, r.Shell.Nav().Sheet().IsOpen())
			_, ok := r.Shell.Nav().Sheet().Content().(*screens.PasswordSheet)
			assert.True(t, ok, "SH.password")
			assert.True(t, r.Tester.Shows("Mật khẩu hiện tại"))
			require.NoError(t, r.Tester.Click("Đóng"))
			r.Tester.Elapse(400 * ms)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "the cross closes it")
		})
		t.Run("logout", func(t *testing.T) {
			r := atHome(t, ph)
			r.Shell.AskLogout()
			r.Settle()
			r.Tester.Elapse(400 * ms) // the sheet slides in
			require.True(t, r.Shell.Nav().Sheet().IsOpen())
			_, ok := r.Shell.Nav().Sheet().Content().(*screens.LogoutSheet)
			assert.True(t, ok, "SH.logout")
			assert.True(t, r.Tester.Shows("Đăng xuất?"))
			assert.True(t, r.Tester.Shows("File âm thanh đã tải vẫn được giữ trên máy."))
			require.NoError(t, r.Tester.Click("Huỷ"))
			r.Tester.Elapse(400 * ms)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, "home", r.Route(), "Huỷ signs nothing out")
			assert.False(t, r.Fake.Account().LogoutBegan())
			assert.True(t, r.Fake.Signed())
		})
	})
}

// tapKey taps the middle of the element layout.json calls key.
func tapKey(t *testing.T, r *shelltest.Rig, key string) {
	t.Helper()
	e, ok := r.Dump("tap").Find(key)
	require.Truef(t, ok, "%s is on the screen", key)
	r.Tester.ClickAt(geom.Pt(float32(e.X+e.W/2), float32(e.Y+e.H/2)))
	r.Settle()
}

// loginBanner returns the banner the login route was opened with.
func loginBanner(t *testing.T, r *shelltest.Rig) env.LoginBanner {
	t.Helper()
	require.Equal(t, env.RouteLogin, r.Route())
	p, _ := r.Shell.Nav().Current().Params.(env.LoginParams)

	return p.Banner
}

// playingAudio puts a sound playing on the Fake's player, as a lesson left running.
func playingAudio(r *shelltest.Rig) *shelltest.FakePlayback {
	pb := r.Fake.FakePlayback()
	pb.Set(player.Lesson{CourseID: "set-35", ID: "l_3503", No: 3}, player.KindAudio, player.StatePlaying, 42, 600)

	return pb
}

// TestLogoutFlow checks `logout` of SH.logout (PROGRESS Q-P6-13, P4 "For P6"): the
// sheet closes, the sound stops and the player Leaves BEFORE the sign-out begins,
// the login shows at once with no banner, the stack empty, the sign-out runs
// without holding the UI thread, and the account view is forgotten.
func TestLogoutFlow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("order", func(t *testing.T) {
			r := atHome(t, ph)
			pb := playingAudio(r)
			_, err := r.Shell.Env().Account().Me(context.Background())
			require.NoError(t, err)
			require.True(t, r.Shell.Env().AccountView().HasMe)
			r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
			r.Shell.AskLogout()
			r.Settle()
			r.Tester.Elapse(400 * ms)

			// screen 15 is behind the sheet and has a "Đăng xuất" of its own: tap the sheet's
			tapKey(t, r, "logout")

			assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet closed")
			assert.Equal(t, env.LoginBannerNone, loginBanner(t, r), "no banner: the user asked")
			assert.Equal(t, 0, r.Shell.Nav().Depth())
			assert.Equal(t, []string{"Pause", "Leave"}, pb.Calls(), "the sound stops, then the player Leaves")
			acct := r.Fake.Account()
			require.True(t, acct.LogoutBegan())
			assert.Equal(t, []string{"Pause", "Leave"}, acct.LogoutSaw(), "Leave came before Logout began")
			assert.Equal(t, 1, r.Fake.Calls("auth.logout"), "the server is told")
			assert.False(t, r.Fake.Signed(), "the vault is clear")
			assert.Equal(t, env.AccountState{}, r.Shell.Env().AccountView(), "nothing of the user is kept")
			assert.Equal(t, 5, r.Fake.Dl().DoneCount(), "the downloaded files stay (Q-P5-8)")
		})
		t.Run("the UI thread is not held and a Login waits", func(t *testing.T) {
			r := atHome(t, ph)
			release := r.Fake.Account().HoldLogout()
			t.Cleanup(release) // a Logout still held would stop the tester from closing
			r.Shell.AskLogout()
			r.Settle()
			r.Tester.Elapse(400 * ms)
			require.NoError(t, r.Tester.Click("Đăng xuất"))
			r.WaitFor(r.Fake.Account().LogoutBegan, "the sign-out beginning")

			assert.Equal(t, env.LoginBannerNone, loginBanner(t, r), "the login shows while the sign-out is held")
			assert.True(t, r.Fake.Signed(), "the session is still there: the sign-out is under way")

			loginErr := make(chan error, 1)
			go func() { loginErr <- r.Fake.Login(context.Background(), "next@example.com", "x") }()
			for range 2000 {
				runtime.Gosched()
			}
			select {
			case err := <-loginErr:
				t.Fatalf("a Login ended while the Logout was held: %v", err)
			default:
			}

			release()
			require.NoError(t, <-loginErr)
			r.Settle()
			assert.True(t, r.Fake.Signed(), "the Login came after the Logout: its session is not the one cleared")
			assert.Equal(t, 1, r.Fake.Calls("auth.logout"))
		})
		t.Run("the sign-out is marked on the UI thread", func(t *testing.T) {
			r := atHome(t, ph)
			release := r.Fake.Account().HoldLogout()
			t.Cleanup(release)
			r.Shell.Logout()
			assert.True(t, r.Fake.Account().LogoutMarked(), "marked before Logout returns, not by the goroutine: a Login waits")
			release()
			r.WaitFor(func() bool { return !r.Fake.Account().LogoutMarked() }, "the sign-out ending")
		})
		t.Run("a failed sign-out is not shown", func(t *testing.T) {
			r := atHome(t, ph)
			r.Fake.Account().SetLogoutError(errors.New("vault: cannot clear"))
			r.Shell.AskLogout()
			r.Settle()
			r.Tester.Elapse(400 * ms)
			require.NoError(t, r.Tester.Click("Đăng xuất"))
			r.Settle()
			r.Tester.Elapse(300 * ms)
			assert.Equal(t, env.RouteLogin, r.Route())
			assert.False(t, r.Tester.Shows("vault: cannot clear"))
			assert.False(t, r.Tester.Shows(toastFailed))
		})
		t.Run("without a player", func(t *testing.T) {
			r := atHome(t, ph)
			r.Fake.SetPlayback(nil)
			r.Shell.Logout()
			r.Settle()
			assert.Equal(t, env.LoginBannerNone, loginBanner(t, r))
			assert.True(t, r.Fake.Account().LogoutBegan())
		})
	})
}

// TestSessionEndedByTheServerForgetsTheAccount checks that a session the server
// refuses also forgets what the account screens kept, and shows the login with
// its banner.
func TestSessionEndedByTheServerForgetsTheAccount(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph)
		_, err := r.Shell.Env().Account().Devices(context.Background())
		require.NoError(t, err)
		require.True(t, r.Shell.Env().AccountView().HasDevices)
		r.Fake.SignOut(auth.ErrSignedOut)
		r.Settle()
		assert.Equal(t, env.LoginBannerExpired, loginBanner(t, r))
		assert.Equal(t, env.AccountState{}, r.Shell.Env().AccountView())
	})
}
