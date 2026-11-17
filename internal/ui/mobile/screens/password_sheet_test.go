//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the toasts of the password sheet (locales/vi.json).
const (
	toastPwRequired = "Nhập đủ ba ô mật khẩu"
	toastPwShort    = "Mật khẩu mới cần ít nhất 8 ký tự"
	toastPwMismatch = "Mật khẩu nhập lại không khớp"
	toastPwWrong    = "Mật khẩu hiện tại chưa đúng"
	toastPwSaved    = "Đã đổi mật khẩu"
)

// atPassword opens SH.password from screen 15, the way the row does.
func atPassword(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) (*shelltest.Rig, *screens.PasswordSheet) {
	tb.Helper()
	r := atAccount(tb, ph, real, setups...)
	tapKey(tb, r, "sheet:password")
	slideIn(r)
	sh, ok := r.Shell.Nav().Sheet().Content().(*screens.PasswordSheet)
	require.True(tb, ok, "the sheet showing is SH.password")

	return r, sh
}

// fillPassword types the three fields.
func fillPassword(sh *screens.PasswordSheet, current, next, repeat string) {
	sh.Current().SetText(current)
	sh.New().SetText(next)
	sh.Repeat().SetText(repeat)
}

// TestPasswordSheetShows checks the sheet: the title, the three labelled fields that
// show dots, the button.
func TestPasswordSheetShows(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		for _, text := range []string{"Đổi mật khẩu", "Mật khẩu hiện tại", "Mật khẩu mới", "Nhập lại mật khẩu mới", "Lưu mật khẩu"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the sheet", text)
		}
		for _, in := range []*kit.Input{sh.Current(), sh.New(), sh.Repeat()} {
			assert.Equal(t, '•', in.Mask, "the dots of `type=password`")
			assert.Equal(t, "••••••••", in.Placeholder)
		}
		assert.Zero(t, r.Fake.Account().Count("ChangePassword"))
	})
}

// TestPasswordLocalChecks checks Q-P6-5: an empty field, a new password under 8
// characters and a repeat that differs are each a toast, in that order, and nothing
// is sent; the sheet stays open with what was typed.
func TestPasswordLocalChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		current, next, repeat string
		toast                 string
	}{
		{"all empty", "", "", "", toastPwRequired},
		{"no current", "", "abcd1234", "abcd1234", toastPwRequired},
		{"no new", "old-pass1", "", "", toastPwRequired},
		{"no repeat", "old-pass1", "abcd1234", "", toastPwRequired},
		{"seven characters", "old-pass1", "abcd123", "abcd123", toastPwShort},
		{"seven runes of two bytes", "old-pass1", "ĐĐĐĐĐĐĐ", "ĐĐĐĐĐĐĐ", toastPwShort},
		{"short is told before the mismatch", "old-pass1", "abc", "abd", toastPwShort},
		{"repeat differs", "old-pass1", "abcd1234", "abcd1235", toastPwMismatch},
		{"repeat differs by case", "old-pass1", "abcd1234", "ABCD1234", toastPwMismatch},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, sh := atPassword(t, ph, false)
				fillPassword(sh, tc.current, tc.next, tc.repeat)
				press(r, sh.Save())
				assert.True(t, shows(r, tc.toast))
				assert.Zero(t, r.Fake.Account().Count("ChangePassword"), "nothing is sent")
				assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
				assert.Equal(t, tc.next, sh.New().Text(), "what was typed stays")
				assert.False(t, sh.Busy())
			})
		}
	})
}

// TestPasswordAcceptsEightRunes checks the edge of the rule: eight characters are
// enough.
func TestPasswordAcceptsEightRunes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "ĐĐĐĐĐĐĐĐ", "ĐĐĐĐĐĐĐĐ")
		press(r, sh.Save())
		assert.Equal(t, 1, r.Fake.Account().Count("ChangePassword"))
	})
}

// TestPasswordSaves checks success: the passwords go as typed, the sheet closes with
// "Đã đổi mật khẩu", the fields are wiped, and the session is kept (screen 15 stays).
func TestPasswordSaves(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
		press(r, sh.Save())
		assert.Equal(t, [][2]string{{"old-pass1", "new-pass-2"}}, r.Fake.Account().Passwords())
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet closed")
		assert.True(t, shows(r, toastPwSaved))
		assert.Equal(t, env.RouteAccount, r.Route())
		assert.True(t, r.Fake.Signed(), "the session is kept (Q-P6-23)")
		assert.Empty(t, sh.New().Text(), "the passwords are not kept in the fields")
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows(toastPwSaved), "the toast goes after 2200 ms")
	})
}

// TestPasswordSavesOnce checks that a second tap while the request is out is ignored.
func TestPasswordSavesOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
		release := r.Fake.Gate("me.password")
		t.Cleanup(release)
		tap(r, sh.Save())
		r.WaitFor(sh.Busy, "the request under way")
		tap(r, sh.Save())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("me.password"))
	})
}

// TestPasswordServerErrors runs the scenarios of PUT /me/password and the failures of
// the network: the sheet stays open, the field the server names goes red and the
// toast says what is wrong; typing takes the red back.
func TestPasswordServerErrors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("wrong_password", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			require.NoError(t, r.Fake.SetScenario("me.password", "wrong_password"))
			fillPassword(sh, "bad-pass-1", "new-pass-2", "new-pass-2")
			press(r, sh.Save())
			assert.True(t, shows(r, toastPwWrong))
			assert.True(t, sh.Current().Error, "the current password is red")
			assert.False(t, sh.New().Error)
			assert.True(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, "bad-pass-1", sh.Current().Text())
			assert.Same(t, sh.Current().Field(), focused(r), "the focus is on the field to correct")
			assert.False(t, sh.Busy())

			r.Tester.Type("x")
			assert.False(t, sh.Current().Error, "typing takes the red back")

			require.NoError(t, r.Fake.SetScenario("me.password", "default"))
			sh.Current().SetText("old-pass1")
			press(r, sh.Save())
			slideIn(r)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "a corrected password goes through")
		})
		t.Run("offline", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
			r.Fake.SetOnline(false)
			press(r, sh.Save())
			assert.True(t, shows(r, toastOffline))
			assert.True(t, r.Shell.Nav().Sheet().IsOpen())
			assert.False(t, sh.Current().Error)
			assert.Equal(t, "new-pass-2", sh.New().Text(), "what was typed stays")
		})
		t.Run("server error", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
			r.Fake.Fail("me.password", 500, "errors/server_error.json")
			press(r, sh.Save())
			assert.True(t, shows(r, toastBusy))
			assert.True(t, r.Shell.Nav().Sheet().IsOpen())
		})
		t.Run("rate limited", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
			r.Fake.Fail("me.password", 429, "errors/rate_limited.json")
			press(r, sh.Save())
			assert.True(t, shows(r, toastBusy))
			assert.True(t, r.Shell.Nav().Sheet().IsOpen())
		})
		t.Run("upgrade required", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
			r.Fake.Fail("me.password", 426, "errors/upgrade_required.json")
			press(r, sh.Save())
			assert.Equal(t, env.RouteUpgrade, r.Route())
		})
	})
}

// TestPasswordEnterMovesOn checks the Enter of each field: Next to the following
// field, Done on the last one saves.
func TestPasswordEnterMovesOn(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
		r.Tester.View().Focus(sh.Current().Field())
		for _, next := range []*kit.Input{sh.New(), sh.Repeat()} {
			require.True(t, r.Tester.Key(keys.KeyEnter, 0))
			assert.Same(t, next.Field(), focused(r))
		}
		require.True(t, r.Tester.Key(keys.KeyEnter, 0))
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("ChangePassword"))
	})
}

// TestPasswordSheetClosesAndDropsTheRequest checks the cross, and that closing the
// sheet while the request is out leaves no toast behind.
func TestPasswordSheetCloses(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "new-pass-2", "new-pass-2")
		release := r.Fake.Gate("me.password")
		t.Cleanup(release)
		tap(r, sh.Save())
		r.WaitFor(sh.Busy, "the request under way")
		require.NoError(t, r.Tester.Click("Đóng")) // the cross: a Dump would wait for the held request
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		release()
		r.Settle()
		assert.False(t, shows(r, toastPwSaved), "the answer of a closed sheet shows nothing")
	})
}

// TestPasswordKeyboardLiftsTheSheet checks that the sheet rises over the keyboard
// with the focused field in view (kit.Sheet's rule, on both phones).
func TestPasswordKeyboardLiftsTheSheet(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		panel := func() geom.Rect { return element2(r, "div.sheet") }
		before := panel()
		r.Tester.View().Focus(sh.Repeat().Field())
		r.Tester.SetKeyboard(geom.Insets{Bottom: 336})
		r.Tester.Elapse(400 * time.Millisecond)
		assert.LessOrEqual(t, sh.Repeat().Field().WindowFrame().Max().Y, ph.Size.H-336, "the field is above the keyboard")
		assert.LessOrEqual(t, sh.Save().WindowFrame().Max().Y, ph.Size.H-336+0.5, "so is the button")
		assert.Less(t, panel().Y, before.Y, "the sheet rose")
	})
}

// element2 is the rectangle of the element key of the dump.
func element2(r *shelltest.Rig, key string) geom.Rect {
	e, _ := r.Dump("sheet").Find(key)

	return geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
}

// TestPasswordSheetSteadyFrameDoesNotAllocate checks that a frame of the sheet standing
// still allocates nothing, on both phones.
func TestPasswordSheetSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, sh := atPassword(t, ph, false)
		r.Tester.View().Focus(sh.Current().Field())
		sh.Current().SetText("secret")
		for range 40 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
