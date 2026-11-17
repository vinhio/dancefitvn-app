//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyboardH is the iPhone's docked keyboard (335 points) and nearly the Android one.
const keyboardH = 336

// TestPasswordFirstTapSavesWithTheKeyboardUp checks the first tap on "Lưu mật khẩu"
// with a field being typed in: a press on the button takes the focus from the field,
// the keyboard starts going down and the system tells so while the finger is still on
// the glass (the sheet's bottom follows the keyboard, so the button moves away from the
// finger); the tap still saves, it does not only put the keyboard away.
func TestPasswordFirstTapSavesWithTheKeyboardUp(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atPassword(t, ph, false)
		fillPassword(sh, "old-pass1", "abcd1234", "abcd1234")
		r.Tester.View().Focus(sh.Repeat().Field())
		r.Tester.SetKeyboard(geom.Insets{Bottom: keyboardH})
		r.Tester.Elapse(400 * time.Millisecond)
		require.True(t, r.Tester.TextInput().Enabled, "the keyboard is up")
		at := sh.Save().WindowFrame().Center()
		require.LessOrEqual(t, sh.Save().WindowFrame().Max().Y, ph.Size.H-keyboardH+0.5, "above the keyboard")

		r.Tester.TouchDown(at)
		assert.False(t, r.Tester.TextInput().Enabled, "the press took the focus: the keyboard goes")
		r.Tester.SetKeyboard(geom.Insets{}) // the system reports it going, the finger still down
		r.Tester.Elapse(60 * time.Millisecond)
		r.Tester.TouchUp(at)
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("ChangePassword"), "the first tap saved")
	})
}

// tapAcrossKeyboard plays a tap on b with a field being typed in: the field has the
// focus and the keyboard is up; the finger goes down on b, the keyboard is told to go
// (the press took the focus), the system reports it while the finger is down, and the
// finger lifts where it went down. It reports whether b was above the keyboard.
func tapAcrossKeyboard(t *testing.T, r *shelltest.Rig, scroll *ui.Scroll, field *kit.Input, b *kit.Button) bool {
	t.Helper()
	r.Tester.View().Focus(field.Field())
	r.Tester.SetKeyboard(geom.Insets{Bottom: keyboardH})
	r.Tester.Elapse(400 * time.Millisecond)
	if scroll != nil { // the page is scrolled to its end, where the button is, as a finger would
		scroll.ScrollTo(scroll.MaxOffset(), false)
		r.Tester.Elapse(kittest.Frame)
	}
	if b.WindowFrame().Max().Y > r.Phone.Size.H-keyboardH || b.WindowFrame().Y < 0 {
		return false // the keyboard covers it: nobody can tap it
	}
	at := b.WindowFrame().Center()
	r.Tester.TouchDown(at)
	r.Tester.SetKeyboard(geom.Insets{})
	r.Tester.Elapse(60 * time.Millisecond)
	r.Tester.TouchUp(at)
	r.Settle()

	return true
}

// TestFirstTapOnTheButtonOfAFormWithTheKeyboardUp checks the same of the other forms with
// text fields: the sign-in, the account's data and the coach's profile. A button that
// the keyboard covers cannot be tapped and is left out.
func TestFirstTapOnTheButtonOfAFormWithTheKeyboardUp(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("login", func(t *testing.T) {
			r, l := atLogin(t, ph, false)
			l.Email().SetText("thaovy.hlv@gmail.com")
			l.Password().SetText("abcd1234")
			require.True(t, tapAcrossKeyboard(t, r, l.Scroll(), l.Password(), l.SubmitButton()))
			assert.Equal(t, 1, l.SubmitButton().Presses(), "the first tap signs in")
		})
		t.Run("info", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			if !tapAcrossKeyboard(t, r, in.Scroll(), in.First(), in.Submit()) {
				t.Skip("the keyboard covers the button")
			}
			assert.Equal(t, 1, in.Submit().Presses(), "the first tap saves")
		})
		t.Run("coach", func(t *testing.T) {
			r, co := atCoach(t, ph, false)
			if !tapAcrossKeyboard(t, r, co.Scroll(), co.Places()[0], co.Save()) {
				t.Skip("the keyboard covers the button")
			}
			assert.Equal(t, 1, co.Save().Presses(), "the first tap saves")
		})
	})
}

// TestBackWithTheKeyboardUpOnlyHidesIt checks the system's Back (Android) with a field
// being typed in: the first Back puts the keyboard away and leaves the sheet, or the
// screen, where it is, with what was typed; the next one acts as it always does.
func TestBackWithTheKeyboardUpOnlyHidesIt(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("password sheet", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			sh.New().SetText("abcd1234")
			r.Tester.View().Focus(sh.New().Field())
			r.Tester.SetKeyboard(geom.Insets{Bottom: keyboardH})
			r.Tester.Elapse(400 * time.Millisecond)
			require.True(t, r.Tester.TextInput().Enabled)

			assert.True(t, r.Tester.Key(keys.KeyBack, 0), "the first Back is taken")
			assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
			assert.False(t, r.Tester.TextInput().Enabled, "the keyboard is put away")
			assert.Nil(t, focused(r))
			assert.Equal(t, "abcd1234", sh.New().Text(), "what was typed stays")
			r.Tester.SetKeyboard(geom.Insets{}) // the system's report

			assert.True(t, r.Tester.Key(keys.KeyBack, 0), "the next Back")
			r.Tester.Elapse(400 * time.Millisecond)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "closes the sheet")
		})
		t.Run("screen", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			r.Tester.View().Focus(in.First().Field())
			r.Tester.SetKeyboard(geom.Insets{Bottom: keyboardH})
			r.Tester.Elapse(400 * time.Millisecond)

			assert.True(t, r.Tester.Key(keys.KeyBack, 0))
			assert.Equal(t, env.RouteInfo, r.Route(), "the screen stays")
			assert.False(t, r.Tester.TextInput().Enabled)
			r.Tester.SetKeyboard(geom.Insets{})

			assert.True(t, r.Tester.Key(keys.KeyBack, 0))
			r.Tester.Elapse(400 * time.Millisecond)
			r.Settle()
			assert.Equal(t, env.RouteAccount, r.Route(), "the next one goes back")
		})
		t.Run("no keyboard", func(t *testing.T) {
			r, sh := atPassword(t, ph, false)
			r.Tester.View().Focus(sh.New().Field()) // the field has the focus, the keyboard is not up (Back hid it already)
			assert.True(t, r.Tester.Key(keys.KeyBack, 0))
			r.Tester.Elapse(400 * time.Millisecond)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "Back with no keyboard closes the sheet at once")
		})
	})
}
