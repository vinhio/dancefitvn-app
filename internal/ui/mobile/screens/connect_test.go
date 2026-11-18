//go:build !tvos

package screens_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectUnlinked checks screen 16 with no TV linked (M16), on both phones: the texts,
// the empty boxes with the first one focused, the button disabled, GET /me/devices read.
func TestConnectUnlinked(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		assert.Equal(t, -1, c.Tab())
		assert.False(t, c.Linked())
		assert.Empty(t, c.Boxes().Value)
		assert.Equal(t, 0, c.Boxes().FocusedBox(), "the first box is the one the next character goes to")
		assert.True(t, c.Link().Disabled, "`opacity:.45` below six characters")
		for _, text := range []string{"Kết nối Apple TV", "Mã trên TV", "7K4-M2P", "Nhập mã trên TV",
			"Mở app DanceFITVN trên Apple TV, chọn Đăng nhập rồi nhập mã 6 ký tự đang hiển thị.", "Kết nối TV"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		assert.False(t, r.Tester.Shows("Mã hết hạn sau 04:18 · gợi ý: 7K4M2P"), "K24: the expiry line is not drawn")
		assert.False(t, r.Tester.Shows("Đã kết nối TV"))
		assert.Positive(t, r.Fake.Account().Count("Devices"), "GET /me/devices was read")
		assert.Zero(t, r.Fake.Account().Count("PairConfirm"))
	})
}

// TestConnectTyping checks the boxes (`$P:569-570`): the next box takes the focus mark as
// characters come, backspace goes back, a paste of "7k4-m2p" is the code, nothing but 0-9
// and A-Z is kept, no more than six, and the button takes its colour back at the sixth.
func TestConnectTyping(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		typeCode(r, c, "7")
		assert.Equal(t, "7", c.Boxes().Value)
		assert.Equal(t, 1, c.Boxes().FocusedBox(), "auto-advance")
		assert.True(t, r.Tester.Shows("7"))
		r.Tester.Type("k4")
		assert.Equal(t, "7K4", c.Boxes().Value, "lower case is upper-cased")
		assert.Equal(t, 3, c.Boxes().FocusedBox())
		assert.True(t, c.Link().Disabled)
		r.Tester.Type("M2")
		assert.Equal(t, 5, c.Boxes().FocusedBox())
		assert.True(t, c.Link().Disabled, "five characters: not yet")
		r.Tester.Type("P")
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
		assert.Equal(t, 5, c.Boxes().FocusedBox(), "min(len, 5)")
		assert.False(t, c.Link().Disabled)
		r.Tester.Type("9")
		assert.Equal(t, "7K4M2P", c.Boxes().Value, "no more than six")

		require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		assert.Equal(t, "7K4M2", c.Boxes().Value)
		assert.Equal(t, 5, c.Boxes().FocusedBox())
		assert.True(t, c.Link().Disabled, "disabled again below six")
		require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		assert.Equal(t, "7K4M", c.Boxes().Value)
		assert.Equal(t, 4, c.Boxes().FocusedBox())

		// a paste: dashes, spaces and lower case go, only the code stays
		c.Boxes().SetValue("")
		r.Tester.Type(" 7k4-m2p ")
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
		assert.False(t, c.Link().Disabled)
	})
}

// TestConnectWrongCodeCanBeDeletedAndRetyped is the report of a phone: a wrong character
// could not be deleted. A wrong code is typed, the user taps the boxes (which put a
// caret inside the invisible field, when it had one), deletes it all with the keyboard's
// Backspace and retypes it, and the right code is the one the server gets.
func TestConnectWrongCodeCanBeDeletedAndRetyped(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		tap(r, c.Boxes())
		require.True(t, c.Boxes().Field().Node().Focused())
		r.Tester.Type("7K4M2x")
		assert.Equal(t, "7K4M2X", c.Boxes().Value)
		assert.False(t, c.Link().Disabled)

		// a tap on the first box, as the user does to see where the caret is
		f := c.Boxes().Node().WindowFrame()
		r.Tester.ClickAt(geom.Pt(f.X+20, f.Center().Y))
		require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		assert.Equal(t, "7K4M2", c.Boxes().Value, "Backspace takes back the last character")
		assert.True(t, c.Link().Disabled, "below six again")
		assert.False(t, r.Tester.Shows("X"))

		// the keyboard has no caret to move or range to select: the field lends it no text
		r.Tester.TextSource().SelectText(platform.TextRange{})
		r.Tester.TextSource().SelectText(platform.TextRange{Start: 0, End: 3})
		for range 5 {
			require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		}
		assert.Empty(t, c.Boxes().Value)
		assert.Equal(t, 0, c.Boxes().FocusedBox())
		assert.True(t, c.Link().Disabled)
		r.Tester.Key(keys.KeyBackspace, 0) // on an empty row it does nothing
		assert.Empty(t, c.Boxes().Value)

		r.Tester.Type("7k4-m2p")
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
		assert.False(t, c.Link().Disabled)
		tap(r, c.Link())
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("PairConfirm:7K4M2P"), "the retyped code is the one sent")
		assert.Zero(t, r.Fake.Account().Count("PairConfirm:7K4M2X"))
		assert.True(t, c.Linked())
	})
}

// TestConnectTapOnTheRowFocusesTheField checks that a tap anywhere on the boxes takes the
// keyboard (`<label>` around the boxes and the hidden input).
func TestConnectTapOnTheRowFocusesTheField(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		tap(r, c.Boxes())
		assert.True(t, c.Boxes().Field().Node().Focused())
	})
}

// TestConnectShortCodeDoesNothing checks `linkTV`'s first rule: below six characters a tap
// sends nothing, shows nothing and the screen stays.
func TestConnectShortCodeDoesNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		typeCode(r, c, "7K4M2")
		tap(r, c.Link())
		r.Settle()
		assert.Zero(t, r.Fake.Account().Count("PairConfirm"))
		assert.False(t, c.Busy())
		assert.False(t, c.Linked())
		assert.False(t, r.Tester.Shows(toastCodeWrong))
		assert.False(t, c.Boxes().Error)
		c.Link().OnPress() // the callback itself checks too: a keyboard action must not send it
		r.Settle()
		assert.Zero(t, r.Fake.Account().Count("PairConfirm"))
	})
}

// TestConnectLinks checks `linkTV` with the right code: the code goes to the server as typed
// in its normal form, the screen shows the linked state with the toast, the account view
// holds the TV, and screen 15 says "Đã kết nối" after Back.
func TestConnectLinks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		_, linked := r.Shell.Env().TVDevice()
		require.False(t, linked)
		typeCode(r, c, "7k4-m2p")
		tap(r, c.Link())
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("PairConfirm:7K4M2P"), "the code as the pairing sends it")
		assert.True(t, c.Linked())
		assert.False(t, c.Busy())
		assert.True(t, shows(r, toastTVLinked))
		assert.True(t, r.Tester.Shows("Đã kết nối TV"))
		assert.True(t, r.Tester.Shows("Apple TV đang dùng tài khoản "+meEmail))
		assert.True(t, r.Tester.Shows("Ngắt kết nối"))
		assert.False(t, r.Tester.Shows("Nhập mã trên TV"))
		tv, linked := r.Shell.Env().TVDevice()
		assert.True(t, linked, "the account view holds the TV")
		assert.NotEmpty(t, tv.Name)

		tapKey(t, r, "back")
		require.Equal(t, env.RouteAccount, r.Route())
		assert.Equal(t, "Đã kết nối", accountOf(t, r).TV())
	})
}

// TestConnectLinksOnce checks that a second tap while the code is out is ignored.
func TestConnectLinksOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		typeCode(r, c, "7K4M2P")
		release := r.Fake.Gate("tv.pair.confirm")
		t.Cleanup(release)
		tap(r, c.Link())
		r.WaitFor(c.Busy, "the code under way")
		tap(r, c.Link())
		tap(r, c.Link())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("tv.pair.confirm"))
		assert.True(t, c.Linked())
	})
}

// TestConnectRefusals runs the refusals of POST /tv/pair/confirm and the network: the screen
// stays on the code entry, the code stays and the boxes do not go red (Q-P6.4-2: the
// prototype shows the toast alone).
func TestConnectRefusals(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("invalid_code", func(t *testing.T) {
			r, c := atConnect(t, ph, false)
			require.NoError(t, r.Fake.SetScenario("tv.pair.confirm", "invalid_code"))
			typeCode(r, c, "ABCDEF")
			tap(r, c.Link())
			r.Settle()
			assert.True(t, shows(r, toastCodeWrong))
			assert.False(t, c.Boxes().Error, "the boxes do not go red")
			assert.Equal(t, "ABCDEF", c.Boxes().Value, "the code stays")
			assert.False(t, c.Linked())
			assert.False(t, c.Busy())
			assert.Equal(t, env.RouteConnect, r.Route())
			assert.True(t, c.Boxes().Field().Node().Focused(), "the field has the focus back, to correct the code")
			require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
			assert.Equal(t, "ABCDE", c.Boxes().Value)
		})
		t.Run("expired", func(t *testing.T) {
			r, c := atConnect(t, ph, false)
			r.Fake.Fail("tv.pair.confirm", 410, "errors/pair_code_expired.json")
			typeCode(r, c, "ABCDEF")
			tap(r, c.Link())
			r.Settle()
			assert.True(t, shows(r, toastCodeExpired))
			assert.False(t, c.Boxes().Error)
			assert.False(t, c.Linked())
			assert.Equal(t, "ABCDEF", c.Boxes().Value)
		})
		t.Run("offline", func(t *testing.T) {
			r, c := atConnect(t, ph, false)
			typeCode(r, c, "7K4M2P")
			r.Fake.SetOnline(false)
			tap(r, c.Link())
			r.Settle()
			assert.True(t, shows(r, toastOffline))
			assert.False(t, c.Boxes().Error, "a network that is down says nothing of the code")
			assert.False(t, c.Linked())
			r.Fake.SetOnline(true)
			tap(r, c.Link())
			r.Settle()
			assert.True(t, c.Linked(), "links when the network is back")
		})
		t.Run("server error", func(t *testing.T) {
			r, c := atConnect(t, ph, false)
			r.Fake.Fail("tv.pair.confirm", 500, "errors/server_error.json")
			typeCode(r, c, "7K4M2P")
			tap(r, c.Link())
			r.Settle()
			assert.True(t, shows(r, toastBusy))
			assert.False(t, c.Linked())
			assert.False(t, c.Boxes().Error)
		})
		t.Run("upgrade required", func(t *testing.T) {
			r, c := atConnect(t, ph, false)
			r.Fake.Fail("tv.pair.confirm", 426, "errors/upgrade_required.json")
			typeCode(r, c, "7K4M2P")
			tap(r, c.Link())
			r.Settle()
			assert.Equal(t, env.RouteUpgrade, r.Route())
		})
	})
}

// TestConnectUnlinks checks `unlinkTV` (`$P:623`): the TV is deleted from the account, the
// code entry comes back with an empty code and a disabled button, screen 15 says
// "Chưa kết nối" after Back; a TV already gone (404) is gone all the same.
func TestConnectUnlinks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, notFound := range []bool{false, true} {
			name := "default"
			if notFound {
				name = "not_found"
			}
			t.Run(name, func(t *testing.T) {
				r, c := atConnectLinked(t, ph, false)
				require.True(t, c.Linked(), "the TV of the mock is linked")
				assert.True(t, r.Tester.Shows("Apple TV đang dùng tài khoản "+meEmail))
				if notFound {
					require.NoError(t, r.Fake.SetScenario("devices.delete", "not_found"))
				}
				tap(r, c.Unlink())
				r.Settle()
				assert.Equal(t, 1, r.Fake.Account().Count("DeleteDevice:d_201"))
				assert.False(t, c.Linked())
				assert.False(t, c.Busy())
				assert.Empty(t, c.Boxes().Value, "the code is cleared")
				assert.True(t, c.Link().Disabled)
				assert.True(t, r.Tester.Shows("Nhập mã trên TV"))
				assert.False(t, r.Tester.Shows("Ngắt kết nối"))
				_, linked := r.Shell.Env().TVDevice()
				assert.False(t, linked)

				tapKey(t, r, "back")
				assert.Equal(t, "Chưa kết nối", accountOf(t, r).TV())
			})
		}
	})
}

// TestConnectUnlinkOnce checks that taps while the removal is out are ignored, and that a
// failure leaves the linked state with its toast.
func TestConnectUnlinkOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnectLinked(t, ph, false)
		release := r.Fake.Gate("devices.delete")
		t.Cleanup(release)
		tap(r, c.Unlink())
		r.WaitFor(c.Busy, "the removal under way")
		tap(r, c.Unlink())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("devices.delete"))
		assert.False(t, c.Linked())

		r, c = atConnectLinked(t, ph, false)
		r.Fake.SetOnline(false)
		tap(r, c.Unlink())
		r.Settle()
		assert.True(t, shows(r, toastOffline))
		assert.True(t, c.Linked(), "the TV is still linked")
		assert.False(t, c.Busy())
	})
}

// TestConnectLinkedFromTheAccount checks that the state follows the account: the screen is
// built linked from the view that holds the TV, and a read of the devices that finds none
// turns it to the code entry.
func TestConnectLinkedFromTheAccount(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnectLinked(t, ph, false)
		assert.True(t, c.Linked())
		assert.True(t, r.Tester.Shows("Đã kết nối TV"))
		assert.False(t, r.Tester.Shows("Mã trên TV"))
		assert.True(t, c.Link().Hidden, "the fixed button is not for the linked state")

		tapKey(t, r, "back")
		_, c = toConnect(t, r)
		assert.True(t, c.Linked(), "a screen built again draws the linked state at once")
	})
}

// TestConnectBackOnTheKeyboard checks that the system's Back leaves for screen 15.
func TestConnectBackOnTheKeyboard(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atConnect(t, ph, false)
		r.Shell.Back()
		r.Settle()
		assert.Equal(t, env.RouteAccount, r.Route())
	})
}

// TestConnectToast checks M-toast's toast over this screen: it goes after 2200 ms.
func TestConnectToast(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		typeCode(r, c, "7K4M2P")
		tap(r, c.Link())
		r.Settle()
		require.True(t, shows(r, toastTVLinked))
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows(toastTVLinked), "the toast goes after 2200 ms")
	})
}

// TestConnectSteadyFrameDoesNotAllocate checks that a frame of screen 16 standing still
// allocates nothing in each state (code entry empty, typed, linked, the sheet of devices
// over it), with the keyboard down and up, on both phones.
func TestConnectSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	settle := func(r *shelltest.Rig) {
		for range 40 { // the caret's blink and the ring settle
			r.Tester.Elapse(kittest.Frame)
		}
	}
	for _, ph := range kittest.Phones {
		r, c := atConnect(t, ph, false)
		r.Tester.View().Focus(c.Boxes().Field())
		settle(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": empty, focused")

		r.Tester.Type("7K4M2P")
		settle(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": typed")

		r.Tester.SetKeyboard(geom.Insets{Bottom: 336})
		settle(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": keyboard up")

		r, _ = atConnectLinked(t, ph, false)
		settle(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": linked")

		r, _, _ = atLimit(t, ph, false)
		settle(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": the sheet of devices")
	}
}

// TestConnectKeystrokeDoesNotAllocate checks that the screen adds no allocation to a key: the
// field's own (the tester's events, the field's string) are what a key into a bare row of boxes
// costs, and the filter, the button's colour, the layout and the frame cost nothing more.
func TestConnectKeystrokeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, c := atConnect(t, ph, false)
		r.Tester.View().Focus(c.Boxes().Field())
		key := func() {
			r.Tester.Type("S")
			r.Tester.Frame()
			r.Tester.Key(keys.KeyBackspace, 0)
			r.Tester.Frame()
		}
		for range 2 {
			key()
		}
		withScreen := testing.AllocsPerRun(5, key)

		bare := kit.NewCodeBoxes("")
		bt := kittest.Mount(t, ph, bare)
		kittest.PinMotion(bt)
		bt.ClickAt(bare.Node().WindowFrame().Center())
		bareKey := func() {
			bt.Type("S")
			bt.Frame()
			bt.Key(keys.KeyBackspace, 0)
			bt.Frame()
		}
		for range 2 {
			bareKey()
		}
		assert.LessOrEqual(t, withScreen, testing.AllocsPerRun(5, bareKey), ph.Name+": the screen adds nothing to a key")

		// the sixth character turns the button on, the fifth off again
		c.Boxes().SetValue("7K4M2")
		r.Tester.Frame()
		six := func() {
			r.Tester.Type("P")
			r.Tester.Frame()
			r.Tester.Key(keys.KeyBackspace, 0)
			r.Tester.Frame()
		}
		for range 2 {
			six()
		}
		assert.LessOrEqual(t, testing.AllocsPerRun(5, six), withScreen+testing.AllocsPerRun(5, bareKey), ph.Name+": the button's colour costs nothing")
	}
}

// BenchmarkConnectKeystroke is a character into the boxes and the frame after it.
func BenchmarkConnectKeystroke(b *testing.B) {
	r, c := atConnect(b, kittest.Phones[0], false)
	r.Tester.View().Focus(c.Boxes().Field())
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Type("S")
		r.Tester.Frame()
		r.Tester.Key(keys.KeyBackspace, 0)
		r.Tester.Frame()
	}
}

// TestConnectEntryAfterUnlinkIsLaidOut checks the code entry that "Ngắt kết nối" brings
// in place of the linked state: every part has a frame (the picture of the TV, the title,
// the lead, the boxes, the button), is on the screen, and the boxes take typing; the
// linked state comes back the same way after a code (P6.6: the parts shown again kept a
// 0x0 frame, as the flip of Hidden left their parents' layout cached).
func TestConnectEntryAfterUnlinkIsLaidOut(t *testing.T) {
	t.Parallel()

	const lead = "Mở app DanceFITVN trên Apple TV, chọn Đăng nhập rồi nhập mã 6 ký tự đang hiển thị."
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnectLinked(t, ph, false)
		require.True(t, c.Linked())
		assert.True(t, r.Tester.Shows("Đã kết nối TV"))
		assert.Zero(t, c.Boxes().Node().WindowFrame().H, "hidden: no room")

		tap(r, c.Unlink())
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		require.False(t, c.Linked())
		for _, w := range []ui.Widget{c.Boxes(), c.Link()} {
			f := w.Node().WindowFrame()
			assert.Positivef(t, f.W, "%T has a width", w)
			assert.Positivef(t, f.H, "%T has a height", w)
		}
		for _, text := range []string{"Mã trên TV", "7K4-M2P", "Nhập mã trên TV", lead, "Kết nối TV"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is painted", text)
		}
		assert.False(t, r.Tester.Shows("Đã kết nối TV"))
		assert.False(t, c.Unlink().Node().WindowFrame().H > 0, "the button of the other state has no room")
		// the parts stand one under the other, none over the topbar
		top := c.Boxes().Node().WindowFrame().Y
		assert.Greater(t, top, float32(54), "under the bar and the texts")

		typeCode(r, c, "7k4-m2p")
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
		assert.False(t, c.Link().Disabled)

		// and linked again: the linked parts have their frames back
		tap(r, c.Link())
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		require.True(t, c.Linked())
		assert.Positive(t, c.Unlink().Node().WindowFrame().H)
		assert.True(t, r.Tester.Shows("Đã kết nối TV"))
		assert.False(t, r.Tester.Shows("Nhập mã trên TV"))
	})
}

// TestConnectSixthCharacterTakesTheKeyboardDown is 3.7 of the device report: the
// keyboard covers "Kết nối TV", which is fixed over the page's end. With the sixth
// character the keyboard goes down and the button, on, is in sight; a tap on a box
// brings the keyboard back for a correction, and the sixth character takes it down
// again.
func TestConnectSixthCharacterTakesTheKeyboardDown(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		typeCode(r, c, "7K4M2")
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, c.Boxes().Field().Node().Focused(), "five characters: the keyboard stays")
		r.Tester.Type("P")
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, c.Boxes().Field().Node().Focused(), "the sixth takes the keyboard down")
		assert.False(t, c.Link().Disabled)
		assert.Equal(t, "7K4M2P", c.Boxes().Value)

		tap(r, c.Boxes())
		assert.True(t, c.Boxes().Field().Node().Focused(), "a tap on the boxes brings it back")
		require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, c.Boxes().Field().Node().Focused(), "five again: the keyboard stays")
		r.Tester.Type("x")
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, c.Boxes().Field().Node().Focused())
		assert.Equal(t, "7K4M2X", c.Boxes().Value)
	})
}

// TestConnectAsksForTheCodeKeyboard is the device report of an iPhone with the
// Vietnamese keyboard: the characters in the boxes jumped around from the fourth key
// on, the keyboard rewriting the letters typed before (kit.CodeBoxes). The field asks
// the system for gDesk's keyboard of a code (platform.KeyboardCode), which on iOS
// lends the keyboard no text and has it forget each key, and on Android asks for a
// visible password without suggestions. The code is typed as the keys read, and the
// sixth takes the keyboard down.
func TestConnectAsksForTheCodeKeyboard(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		tap(r, c.Boxes())
		in := r.Tester.TextInput()
		assert.True(t, in.Enabled, "the keyboard is up")
		assert.Equal(t, platform.KeyboardCode, in.Keyboard)
		assert.Equal(t, -1, r.Tester.TextSource().TextLength(), "the keyboard is lent no text")
		for _, ch := range "7as4r" {
			r.Tester.Type(string(ch))
			r.Tester.Elapse(kittest.Frame)
		}
		require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		r.Tester.Elapse(kittest.Frame)
		r.Tester.Type("rx")
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, "7AS4RX", c.Boxes().Value)
		assert.False(t, c.Boxes().Field().Node().Focused(), "the sixth takes the keyboard down")
	})
}

// TestConnectComposingKeyboard is the device report with a keyboard that composes
// every letter (Vietnamese Telex): nothing is committed until the keyboard goes away,
// yet the six letters in the boxes are the code. The button turns on at the sixth
// letter, the keyboard goes down, and the code that is sent is the one on show.
func TestConnectComposingKeyboard(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c := atConnect(t, ph, false)
		tap(r, c.Boxes())
		word := ""
		for _, ch := range "7k4m2p" {
			word += string(ch)
			r.Tester.Compose(word, len(word))
			assert.Equal(t, len(word) == kit.CodeLength, !c.Link().Disabled, "the button follows the composition: %q", word)
		}
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, c.Boxes().Field().Node().Focused(), "the sixth letter takes the keyboard down")
		assert.Equal(t, "7K4M2P", c.Boxes().Value, "losing the focus committed the composition")
		assert.Equal(t, 5, c.Boxes().FocusedBox())

		tap(r, c.Link())
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("PairConfirm:7K4M2P"))
		assert.True(t, c.Linked())
	})
}

// flakyDevices is a Fake whose reads of the devices are told apart: the answer
// goes out after the kept view was reset (a sign-out landing late), or the first read
// fails.
type flakyDevices struct {
	*shelltest.Fake

	e         *env.Env
	resetLate bool // forget the kept view once the list is read
	failFirst bool // the first read fails
	reads     int
}

// Devices reads the list, then misbehaves as asked.
func (f *flakyDevices) Devices(ctx context.Context) (model.DeviceList, error) {
	f.reads++
	if f.failFirst && f.reads == 1 {
		return model.DeviceList{}, fmt.Errorf("flaky: %w", api.ErrServer)
	}
	d, err := f.Fake.Devices(ctx)
	if f.resetLate {
		f.e.ResetAccount()
	}

	return d, err
}

// mountConnect mounts screen 16 alone over services that carry a TV.
func mountConnect(t *testing.T, ph kittest.Phone, fl *flakyDevices) (*screens.Connect, *ui.Tester) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	_, err = f.Restore(ctx())
	require.NoError(t, err)
	f.Account().SetTVLinked(true)
	fl.Fake = f
	e := env.New(fl, texts())
	fl.e = e
	e.Act = &recorder{}
	c := screens.NewConnect(e)
	tt := kittest.MountReal(t, ph, c)
	kittest.PinMotion(tt)
	tt.RunUntilIdle()

	return c, tt
}

// TestConnectFollowsTheServerNotTheKeptView is defect 3.3: the screen takes the state
// from the answer of GET /me/devices, so that a TV that is paired shows as linked,
// and can be unlinked, even when the kept account view was forgotten (a sign-out and a
// sign-in in between) and so holds no device; a first read that fails is read again.
func TestConnectFollowsTheServerNotTheKeptView(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("view forgotten after the read", func(t *testing.T) {
			fl := &flakyDevices{resetLate: true}
			c, tt := mountConnect(t, ph, fl)
			_, kept := fl.e.TVDevice()
			require.False(t, kept, "the kept view holds no device")
			assert.True(t, c.Linked(), "the TV of the account shows as linked")
			assert.True(t, tt.Shows("Ngắt kết nối"))
			assert.True(t, tt.Shows("Đã kết nối TV"))
		})
		t.Run("unlink with the view forgotten", func(t *testing.T) {
			fl := &flakyDevices{resetLate: true}
			c, tt := mountConnect(t, ph, fl)
			require.True(t, c.Linked())
			tt.ClickAt(c.Unlink().Node().WindowFrame().Center())
			tt.RunUntilIdle()
			assert.Equal(t, 1, fl.Fake.Account().Count("DeleteDevice:d_201"), "the TV is deleted on the server")
			assert.False(t, c.Linked())
		})
		t.Run("first read fails", func(t *testing.T) {
			fl := &flakyDevices{failFirst: true}
			c, tt := mountConnect(t, ph, fl)
			assert.False(t, c.Linked(), "nothing is known yet")
			tt.Elapse(2*time.Second + kittest.Frame)
			tt.RunUntilIdle()
			assert.Equal(t, 2, fl.reads, "read once more")
			assert.True(t, c.Linked())
		})
	})
}
