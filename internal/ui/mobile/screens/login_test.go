//go:build !tvos

package screens_test

import (
	"fmt"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bioLabel is the text of the biometric button.
const bioLabel = "Đăng nhập bằng Face ID / Vân tay"

// membersOnly is the notice of the footer (K30), verbatim from the user.
const membersOnly = "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN"

// TestLoginBoxesMatchPrototype lays the login out with the iPhone's real text
// engine and compares every element with M02 and M02B within 1 DIP; the sub line
// of the banner carries another text than the prototype's K2 hint, so its width
// is not compared.
func TestLoginBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	ios := kittest.Phones[0]
	t.Run("M02", func(t *testing.T) {
		t.Parallel()

		r, _ := atLogin(t, ios, true, bio(sys.BiometryFace, nil), lastEmail(meEmail))
		checkAgainstProto(t, "M02", r.Dump("M02"), 1, nil)
	})
	t.Run("M02B", func(t *testing.T) {
		t.Parallel()

		r, lg := atLogin(t, ios, true, bio(sys.BiometryFace, nil), lastEmail(meEmail))
		require.NoError(t, r.Fake.SetScenario("auth.login", "invalid_credentials"))
		lg.Submit("x")
		r.Settle()
		r.Tester.Elapse(100 * time.Millisecond)
		lay := r.Dump("M02B")
		want := shelltest.ProtoLayout(t, "M02B")
		for _, d := range shelltest.Compare(want, lay, map[string]bool{"div.muted": true}) {
			require.False(t, d.Missing, d.Key)
			assert.LessOrEqual(t, d.Max(), 1.0, "%s: dx %+.2f dy %+.2f dw %+.2f dh %+.2f", d.Key, d.DX, d.DY, d.DW, d.DH)
		}
		e, ok := lay.Find("div.muted")
		require.True(t, ok)
		w, _ := want.Find("div.muted")
		assert.InDelta(t, w.X, e.X, 1, "the sub line starts where the prototype's does")
		assert.InDelta(t, w.Y, e.Y, 1)
		assert.InDelta(t, w.H, e.H, 1)
	})
}

// TestLoginOnTheWiderPhone checks the 412 wide phone's login against the iPhone's:
// the same column from the top (y of every element within 1.5 DIPs, the engines'
// line boxes differing by a fraction), the full-width elements 40 narrower than
// the screen, the right-hand ones moved by the 19 the screen is wider.
func TestLoginOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	ios, android := kittest.Phones[0], kittest.Phones[1]
	rI, _ := atLogin(t, ios, true, bio(sys.BiometryFace, nil), lastEmail(meEmail))
	rA, _ := atLogin(t, android, true, bio(sys.BiometryFace, nil), lastEmail(meEmail))
	a, b := rI.Dump("M02"), rA.Dump("M02")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	for _, key := range []string{"div.inp", "div.inp#2", "button.btn.p", "bioLogin", "label"} {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 412-40, e.W, 0.01, key+": width")
	}
	for key, dx := range map[string]float64{"pwEye": 19, "toast": 19} {
		ea, _ := a.Find(key)
		eb, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, ea.X+dx, eb.X, 0.5, key+": moves with the right edge")
	}
}

// TestLoginErrorScenarios runs every auth.login error through the screen on both
// phones and checks the banner (Q-P3-3: the prototype's title for a wrong
// password, the new texts for the rest), the password field and that nothing
// moves on.
func TestLoginErrorScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prepare   func(t *testing.T, f *shelltest.Fake)
		title     string
		sub       string
		fieldErr  bool
		route     string
		wantShown bool
		keepPass  bool // K31: the password stays for the sign-in sent again
	}{
		{
			name: "invalid_credentials with attempts_left",
			prepare: func(t *testing.T, f *shelltest.Fake) {
				require.NoError(t, f.SetScenario("auth.login", "invalid_credentials"))
			},
			title: "Email hoặc mật khẩu chưa đúng", sub: "Bạn còn 3 lần thử", fieldErr: true, route: "login", wantShown: true,
		},
		{
			name: "account_locked, retry_after 900 s is 15 minutes",
			prepare: func(t *testing.T, f *shelltest.Fake) {
				require.NoError(t, f.SetScenario("auth.login", "account_locked"))
			},
			title: "Tài khoản đang tạm khoá", sub: "Thử lại sau 15 phút", route: "login", wantShown: true,
		},
		{
			name:    "device_limit",
			prepare: func(t *testing.T, f *shelltest.Fake) { require.NoError(t, f.SetScenario("auth.login", "device_limit")) },
			title:   "Đã đạt giới hạn thiết bị", sub: "Gỡ bớt một thiết bị để tiếp tục", route: "login", wantShown: true,
			keepPass: true,
		},
		{
			name:    "offline",
			prepare: func(_ *testing.T, f *shelltest.Fake) { f.SetOnline(false) },
			title:   "Không kết nối được", sub: "Kiểm tra mạng rồi thử lại", route: "login", wantShown: true,
		},
		{
			name:    "server error",
			prepare: func(_ *testing.T, f *shelltest.Fake) { f.Fail("auth.login", 500, "errors/server_error.json") },
			title:   "Hệ thống đang bận", sub: "Vui lòng thử lại sau ít phút", route: "login", wantShown: true,
		},
		{
			name:    "rate limited",
			prepare: func(_ *testing.T, f *shelltest.Fake) { f.Fail("auth.login", 429, "errors/rate_limited.json") },
			title:   "Hệ thống đang bận", sub: "Vui lòng thử lại sau ít phút", route: "login", wantShown: true,
		},
		{
			name:    "426 goes to the upgrade screen",
			prepare: func(_ *testing.T, f *shelltest.Fake) { f.Fail("auth.login", 426, "errors/upgrade_required.json") },
			route:   "upgrade",
		},
		{
			name:    "default signs in",
			prepare: func(*testing.T, *shelltest.Fake) {},
			route:   "home",
		},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, lg := atLogin(t, ph, false, lastEmail(meEmail))
				tc.prepare(t, r.Fake)
				if tc.name == "offline" {
					r.Fake.SetOnline(false)
				}
				lg.Submit("secret1")
				r.Settle()
				require.Equal(t, tc.route, r.Route())
				if !tc.wantShown {
					return
				}
				title, sub, shown := lg.Banner()
				assert.True(t, shown)
				assert.Equal(t, tc.title, title)
				assert.Equal(t, tc.sub, sub)
				assert.True(t, r.Tester.Shows(tc.title), "the title is on the screen")
				assert.True(t, r.Tester.Shows(tc.sub), "the sub line is on the screen")
				if tc.keepPass {
					assert.Equal(t, "secret1", lg.Password().Text(), "the password is kept for the retry (K31)")
				} else {
					assert.Equal(t, "", lg.Password().Text(), "the password is cleared")
				}
				assert.Equal(t, tc.fieldErr, lg.Password().Error, "only a wrong password marks the field")
				assert.Equal(t, tc.fieldErr, r.Tester.Shows("Mật khẩu chưa đúng. Kiểm tra lại Caps Lock."))
				assert.False(t, lg.Busy())
				assert.False(t, lg.SubmitButton().Disabled, "the button is back")
				assert.Equal(t, meEmail, lg.Email().Text(), "the email stays")
			})
		}
	})
}

// TestLoginAfterAnErrorSignsIn checks that a sign-in goes through after an error:
// the scenario back to the default, the same screen.
func TestLoginAfterAnErrorSignsIn(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg := atLogin(t, ph, false, lastEmail(meEmail))
		require.NoError(t, r.Fake.SetScenario("auth.login", "invalid_credentials"))
		lg.Submit("wrong")
		r.Settle()
		require.Equal(t, "login", r.Route())
		require.NoError(t, r.Fake.SetScenario("auth.login", "default"))
		lg.Submit("right")
		r.Settle()
		assert.Equal(t, "home", r.Route())
		assert.Equal(t, 0, r.Shell.Nav().Depth(), "03 section 1: the stack is empty")
		assert.True(t, r.Fake.Signed())
	})
}

// TestFlowSplashLoginHomeLoading runs the whole entry on both phones: the splash,
// the login after 600 ms, the form filled by typing with Enter moving on, and the
// Home with its loading state (03B) under the Home tab.
func TestFlowSplashLoginHomeLoading(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shell.Options{})
		r.Settle()
		assert.Equal(t, "splash", r.Route())
		r.Tester.Elapse(600 * time.Millisecond)
		r.Settle()
		require.Equal(t, "login", r.Route())
		lg := r.Shell.Nav().Screen().(*screens.Login)
		t.Cleanup(r.Fake.Gate("me.home")) // Home's data is on its way: 03B stays on show

		r.Tester.View().Focus(lg.Email().Field())
		assert.True(t, r.Tester.Type("coach@example.com"))
		assert.True(t, r.Tester.Key(keys.KeyEnter, 0), "Enter on the email")
		assert.Same(t, lg.Password().Field(), focused(r), "moves on to the password")
		assert.True(t, r.Tester.Type("secret1"))
		assert.Equal(t, 0, r.Fake.Calls("auth.login"))
		assert.True(t, r.Tester.Key(keys.KeyEnter, 0), "Enter on the password goes")
		r.WaitFor(func() bool { return r.Route() == "home" }, "Home")

		assert.Equal(t, 1, r.Fake.Calls("auth.login"))
		assert.Equal(t, "home", r.Route())
		assert.False(t, r.Shell.Nav().TabBar().Hidden, "the Home tab's bar shows")
		assert.True(t, r.Tester.Shows("Đang đồng bộ khoá học…"), "03B: the loading state")
		assert.Equal(t, "coach@example.com", r.Fake.LastEmail())
	})
}

// TestLoginEmptyFieldsTakeTheFocusAndAskNothing checks K6: an empty email takes
// the focus, then an empty password does, and auth.login is not called.
func TestLoginEmptyFieldsTakeTheFocusAndAskNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg := atLogin(t, ph, false, lastEmail(""))
		tap(r, lg.SubmitButton())
		assert.Same(t, lg.Email().Field(), focused(r), "the empty email")
		lg.Email().SetText("   ")
		lg.SubmitButton().Press()
		assert.Same(t, lg.Email().Field(), focused(r), "blanks are empty")
		lg.Email().SetText("a@b.vn")
		lg.SubmitButton().Press()
		assert.Same(t, lg.Password().Field(), focused(r), "the empty password")
		r.Settle()
		assert.Equal(t, 0, r.Fake.Calls("auth.login"))
		assert.False(t, lg.Busy())
	})
}

// TestLoginBusyIgnoresASecondTap checks that while auth.login is under way another
// tap asks nothing, the button is dimmed, and the answer takes the app Home.
func TestLoginBusyIgnoresASecondTap(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg := atLogin(t, ph, false, lastEmail(meEmail))
		release := r.Fake.Gate("auth.login")
		t.Cleanup(release)
		lg.Submit("secret1")
		r.WaitFor(func() bool { return r.Fake.Calls("auth.login") == 1 }, "the request")
		assert.True(t, lg.Busy())
		assert.True(t, lg.SubmitButton().Disabled)
		lg.SubmitButton().Press()
		lg.Submit("secret1")
		r.Tester.Elapse(50 * time.Millisecond)
		assert.Equal(t, 1, r.Fake.Calls("auth.login"), "the second tap is ignored")
		assert.Nil(t, focused(r), "the keyboard went away")
		release()
		r.WaitFor(func() bool { return r.Route() == "home" }, "Home")
		assert.Equal(t, 1, r.Fake.Calls("auth.login"))
	})
}

// TestLoginPwEyeTogglesTheMask checks `pwEye`: the dots, the text, the dots.
func TestLoginPwEyeTogglesTheMask(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg := atLogin(t, ph, false)
		assert.Equal(t, '•', lg.Password().Mask)
		for _, want := range []rune{0, '•'} {
			tap(r, lg.Eye())
			assert.Equal(t, want, lg.Password().Mask)
		}
	})
}

// TestLoginLinksOpenTheConfigsPages checks K15: "Quên mật khẩu?" opens the app
// config's page; with no config yet (no network at launch) it is asked for
// once. The footer opens nothing (K30, TestLoginFooterIsTheMembersOnlyNotice).
func TestLoginLinksOpenTheConfigsPages(t *testing.T) {
	t.Parallel()

	const forgot = "https://dancefitvn.com/quen-mat-khau"
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("config at launch", func(t *testing.T) {
			r, _ := atLogin(t, ph, false)
			require.NoError(t, r.Tester.Click("Quên mật khẩu?"))
			r.Settle()
			assert.Equal(t, []string{forgot}, r.Fake.OpenedURLs())
			assert.Equal(t, 1, r.Fake.Calls("app.config"), "the boot's config is kept")
		})
		t.Run("no config at launch", func(t *testing.T) {
			r := newRig(t, ph, false, shell.Options{}, func(_ *testing.T, f *shelltest.Fake) { f.SetOnline(false) })
			r.Boot()
			require.Equal(t, "login", r.Route())
			require.NoError(t, r.Tester.Click("Quên mật khẩu?"))
			r.Settle()
			assert.Empty(t, r.Fake.OpenedURLs(), "offline: nothing opens")
			assert.True(t, r.Tester.Shows("Kiểm tra mạng rồi thử lại"), "a toast says why")
			r.Fake.SetOnline(true)
			r.Tester.Elapse(2300 * time.Millisecond)
			require.NoError(t, r.Tester.Click("Quên mật khẩu?"))
			r.Settle()
			assert.Equal(t, []string{forgot}, r.Fake.OpenedURLs())
		})
	})
}

// TestLoginFooterIsTheMembersOnlyNotice checks K30: the footer is the notice,
// plain muted text, centred in the page, not focusable and not a link; the old
// "Chưa có tài khoản?" and "Đăng ký trên website" are gone and a tap on the
// notice, or anywhere below the sign-in button, opens nothing.
func TestLoginFooterIsTheMembersOnlyNotice(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atLogin(t, ph, false, bio(sys.BiometryFace, nil))
		w, rect, ok := r.Tester.Find(membersOnly)
		require.True(t, ok, "the notice shows")
		_, isLabel := w.(*kit.Label)
		assert.True(t, isLabel, "plain text, not a link: %T", w)
		f, ok := w.(ui.Focuser)
		require.True(t, ok)
		assert.False(t, f.AcceptsFocus(), "not focusable")
		a, ok := w.(interface{ AccessibleLabel() string })
		require.True(t, ok)
		assert.Equal(t, membersOnly, a.AccessibleLabel(), "the accessibility label is the text")
		assert.InDelta(t, ph.Size.W/2, rect.Center().X, 1, "centred")
		assert.LessOrEqual(t, rect.Max().X, ph.Size.W, "inside the screen")
		for _, gone := range []string{"Chưa có tài khoản?", "Đăng ký trên website", "Đăng ký"} {
			assert.False(t, r.Tester.Shows(gone), gone)
			_, _, found := r.Tester.Find(gone)
			assert.False(t, found, gone)
		}
		before := focused(r)
		r.Tester.ClickAt(rect.Center())
		r.Settle()
		assert.Empty(t, r.Fake.OpenedURLs(), "the footer opens nothing")
		assert.Equal(t, "login", r.Route())
		assert.Equal(t, before, focused(r), "a tap on the notice moves no focus")
		assert.Zero(t, r.Fake.Calls("app.config")-1, "and asks for no config")
	})
}

// TestLoginBioButtonFollowsAvailability checks K13: the biometric button and the
// "hoặc" over it show only when the device offers a biometry with a slot.
func TestLoginBioButtonFollowsAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind sys.BiometryKind
		want bool
	}{
		{sys.BiometryNone, false}, {sys.BiometryUnknown, false},
		{sys.BiometryFace, true}, {sys.BiometryFingerprint, true}, {sys.BiometryOther, true},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(fmt.Sprint(tc.kind), func(t *testing.T) {
				r, lg := atLogin(t, ph, false, bio(tc.kind, nil))
				assert.Equal(t, tc.want, lg.BioShown())
				assert.Equal(t, tc.want, r.Tester.Shows(bioLabel))
				assert.Equal(t, tc.want, r.Tester.Shows("hoặc"))
				assert.True(t, r.Tester.Shows(membersOnly), "the footer follows either way")
			})
		}
	})
}

// TestLoginBioButtonFollowsFaceIDFeature checks K13 with the app config: the
// biometric button shows only when the device offers a biometry AND the config's
// features.face_id_login is on; a config that never came leaves it to the device.
func TestLoginBioButtonFollowsFaceIDFeature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  func(*testing.T, *shelltest.Fake)
		want bool
	}{
		{name: "on", cfg: func(*testing.T, *shelltest.Fake) {}, want: true},
		{name: "off", cfg: func(_ *testing.T, f *shelltest.Fake) {
			f.SetConfigEdit(func(c *model.AppConfig) { c.Features.FaceIDLogin = false })
		}, want: false},
		{name: "no config", cfg: func(_ *testing.T, f *shelltest.Fake) { f.Fail("app.config", 500, "errors/server_error.json") }, want: true},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, lg := atLogin(t, ph, false, bio(sys.BiometryFace, nil), setup(tc.cfg))
				assert.Equal(t, tc.want, lg.BioShown())
				assert.Equal(t, tc.want, r.Tester.Shows(bioLabel))
			})
		}
	})
}

// TestLoginBioLogin checks bioLogin (03 section 3): the sheet's reason and cancel
// texts, success takes the app Home, a cancel stays with no toast, a failed or
// locked biometry focuses the password with the toast "Không xác thực được ..." for
// 2200 ms, an offline refresh says so, and a refused slot is an expired session
// that takes the button away.
func TestLoginBioLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  error
		route   string
		toast   string
		banner  string
		bioGone bool
		focusPw bool
	}{
		{name: "recognised", route: "home"},
		{name: "cancelled", result: sys.ErrBioCanceled, route: "login"},
		{name: "fallback", result: sys.ErrBioFallback, route: "login"},
		{
			name: "not recognised", result: sys.ErrBioFailed, route: "login",
			toast: "Không xác thực được. Hãy đăng nhập bằng mật khẩu", focusPw: true,
		},
		{
			name: "locked out", result: sys.ErrBioLockout, route: "login",
			toast: "Không xác thực được. Hãy đăng nhập bằng mật khẩu", focusPw: true,
		},
		{
			name: "refresh refused", result: fmt.Errorf("refresh: %w", api.ErrSignedOut), route: "login",
			banner: "Phiên đăng nhập đã hết hạn", bioGone: true,
		},
		{name: "refresh offline", result: fmt.Errorf("refresh: %w", api.ErrOffline), route: "login", banner: "Không kết nối được"},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, lg := atLogin(t, ph, false, bio(sys.BiometryFace, tc.result))
				tap(r, lg.BioButton())
				r.Settle()
				assert.Equal(t, tc.route, r.Route())
				prompts := r.Fake.BioPrompts()
				require.Len(t, prompts, 1)
				assert.Equal(t, "Đăng nhập DanceFITVN", prompts[0].Reason)
				assert.Equal(t, "Dùng mật khẩu", prompts[0].Cancel)
				if tc.route == "home" {
					return
				}
				assert.False(t, lg.Busy())
				if tc.toast == "" {
					assert.False(t, r.Tester.Shows("Không xác thực được. Hãy đăng nhập bằng mật khẩu"), "no toast")
				} else {
					r.Tester.Elapse(100 * time.Millisecond)
					assert.True(t, r.Tester.Shows(tc.toast))
					r.Tester.Elapse(2200 * time.Millisecond)
					assert.False(t, r.Tester.Shows(tc.toast), "the toast is gone after 2200 ms")
				}
				if tc.focusPw {
					assert.Same(t, lg.Password().Field(), focused(r))
				}
				if tc.banner != "" {
					title, _, shown := lg.Banner()
					assert.True(t, shown)
					assert.Equal(t, tc.banner, title)
				}
				assert.Equal(t, !tc.bioGone, lg.BioShown())
				assert.Equal(t, !tc.bioGone, r.Tester.Shows(bioLabel), "on the screen too")
			})
		}
	})
}

// TestLoginDoesNotAllocateWhenSteady checks that a steady frame of the login, a
// caret blinking in its field included, allocates nothing.
func TestLoginDoesNotAllocateWhenSteady(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, lg := atLogin(t, ph, false, bio(sys.BiometryFace, nil), lastEmail(meEmail))
		r.Tester.View().Focus(lg.Password().Field())
		for range 3 {
			r.Tester.Elapse(kittest.Frame)
		}
		r.Tester.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}

// TestLoginBannerParam checks that the zero banner and the explicit none show no
// banner and the expired one shows it.
func TestLoginBannerParam(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		banner env.LoginBanner
		shown  bool
	}{
		{"zero", env.LoginBannerUnknown, false},
		{"none", env.LoginBannerNone, false},
		{"expired", env.LoginBannerExpired, true},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, _ := atLogin(t, ph, false)
				lg := screens.NewLogin(r.Shell.Env(), env.LoginParams{Banner: tc.banner})
				_, _, shown := lg.Banner()
				assert.Equal(t, tc.shown, shown)
			})
		}
	})
}
