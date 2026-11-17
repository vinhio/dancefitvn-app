//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atLimit opens screen 16, types "7k4-m2p", and taps "Kết nối TV" with the server over the
// limit of devices (409 device_limit): the sheet of devices opens over it.
func atLimit(tb testing.TB, ph kittest.Phone, real bool) (*shelltest.Rig, *screens.Connect, *screens.DevicesSheet) {
	tb.Helper()
	r, c := atConnect(tb, ph, real)
	r.Fake.Account().SetDeviceLimit(true)
	typeCode(r, c, "7k4-m2p")
	tap(r, c.Link())
	r.Settle()
	slideIn(r)
	require.True(tb, r.Shell.Nav().Sheet().IsOpen(), "the sheet of devices is open")
	s, ok := r.Shell.Nav().Sheet().Content().(*screens.DevicesSheet)
	require.True(tb, ok, "the content is the sheet of devices")

	return r, c, s
}

// TestDevicesSheetOpens checks the sheet that a 409 device_limit opens (docs/plan/03
// section 9), on both phones: the title, the line under it, one radio row for each of
// the three devices with its symbol, name and last day seen, this phone listed and not
// to be chosen, the first TV chosen, the red button, "Huỷ"; the code kept on the screen.
func TestDevicesSheetOpens(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c, s := atLimit(t, ph, false)
		assert.Equal(t, "7K4M2P", s.Code(), "the code as it was typed, filtered")
		assert.Equal(t, 1, r.Fake.Account().Count("PairConfirm:7K4M2P"))
		assert.False(t, c.Linked())
		assert.False(t, c.Busy())
		assert.Equal(t, "7K4M2P", c.Boxes().Value, "the code stays under the sheet")
		for _, text := range []string{"Đã đạt giới hạn thiết bị", "Gỡ bớt một thiết bị để tiếp tục",
			"iPhone 15", "Samsung Galaxy A17", "Apple TV phòng khách", "Gỡ thiết bị", "Huỷ",
			"Hoạt động 14/11/2026", "Hoạt động 12/11/2026", "Hoạt động 13/11/2026", "Máy này"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		rows := s.Rows()
		require.Len(t, rows, 3)
		assert.Equal(t, []icon.Name{icon.Phone, icon.Phone, icon.Tv}, []icon.Name{rows[0].Icon, rows[1].Icon, rows[2].Icon})
		assert.True(t, rows[0].Disabled, "this phone cannot be removed")
		assert.Equal(t, "Máy này", rows[0].Trailing)
		assert.False(t, rows[1].Disabled)
		assert.False(t, rows[2].Disabled)
		assert.Equal(t, "d_201", s.Chosen(), "the first TV that is not this phone")
		assert.False(t, rows[0].On)
		assert.False(t, rows[1].On)
		assert.True(t, rows[2].On)
		assert.False(t, s.RemoveButton().Disabled)
		assert.Equal(t, "devChoice:d_101", rows[0].Name)
	})
}

// TestDevicesSheetChoice checks the radio rows: a press moves the choice, this phone's row
// takes none.
func TestDevicesSheetChoice(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, s := atLimit(t, ph, false)
		rows := s.Rows()
		tap(r, rows[1])
		assert.Equal(t, "d_102", s.Chosen())
		assert.True(t, rows[1].On)
		assert.False(t, rows[2].On, "one at a time")
		tap(r, rows[0])
		assert.Equal(t, "d_102", s.Chosen(), "this phone is not a choice")
		assert.False(t, rows[0].On)
		tap(r, rows[2])
		assert.Equal(t, "d_201", s.Chosen())
	})
}

// TestDevicesSheetRemovesAndLinks checks the flow of the P6 plan: the chosen device is
// removed, the same code is confirmed again, the TV is linked, the sheet closes and screen 16
// shows the linked state with its toast; the account holds the TV and not the device removed.
func TestDevicesSheetRemovesAndLinks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, id := range []string{"d_201", "d_102"} {
			t.Run(id, func(t *testing.T) {
				r, c, s := atLimit(t, ph, false)
				if id == "d_102" {
					tap(r, s.Rows()[1])
				}
				r.Fake.Account().ClearLog()
				tap(r, s.RemoveButton())
				r.Settle()
				slideIn(r)
				assert.Equal(t, []string{"DeleteDevice:" + id, "PairConfirm:7K4M2P"}, filterCalls(r.Fake.Account().Log()),
					"the device goes, then the same code is confirmed again")
				assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet closes")
				assert.True(t, c.Linked())
				assert.True(t, shows(r, toastTVLinked))
				assert.True(t, r.Tester.Shows("Đã kết nối TV"))
				_, linked := r.Shell.Env().TVDevice()
				assert.True(t, linked)
				if id == "d_102" { // the mock answers the TV d_201 to a confirmation, whichever is removed
					for _, d := range r.Shell.Env().AccountView().Devices.Items {
						assert.NotEqual(t, id, d.ID, "the device removed is not in the account")
					}
				}
			})
		}
	})
}

// filterCalls keeps the calls that change the account of the Fake's log.
func filterCalls(log []string) []string {
	var out []string
	for _, c := range log {
		if len(c) > 12 && (c[:12] == "DeleteDevice" || c[:11] == "PairConfirm") {
			out = append(out, c)
		}
	}

	return out
}

// TestDevicesSheetNotFound checks that a device the server no longer has (404) is gone all
// the same: the code is confirmed again and the TV is linked.
func TestDevicesSheetNotFound(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, c, s := atLimit(t, ph, false)
		require.NoError(t, r.Fake.SetScenario("devices.delete", "not_found"))
		tap(r, s.RemoveButton())
		r.Settle()
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, c.Linked())
	})
}

// TestDevicesSheetCancels checks "Huỷ" and the cross: the sheet closes, nothing is removed
// or sent, the screen keeps the code.
func TestDevicesSheetCancels(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, label := range []string{"Huỷ", "Đóng"} {
			t.Run(label, func(t *testing.T) {
				r, c, _ := atLimit(t, ph, false)
				r.Fake.Account().ClearLog()
				require.NoError(t, r.Tester.Click(label))
				slideIn(r)
				assert.False(t, r.Shell.Nav().Sheet().IsOpen())
				assert.Empty(t, filterCalls(r.Fake.Account().Log()))
				assert.False(t, c.Linked())
				assert.Equal(t, "7K4M2P", c.Boxes().Value)
				assert.Equal(t, env.RouteConnect, r.Route())
			})
		}
	})
}

// TestDevicesSheetFailures checks the sheet when the removal fails (it stays, with the
// reason), when the code runs out before it is confirmed again (it closes, the boxes go
// red, the toast says so), and when the server answers a limit again (the rows are made
// again from its list).
func TestDevicesSheetFailures(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("removal offline", func(t *testing.T) {
			r, c, s := atLimit(t, ph, false)
			r.Fake.SetOnline(false)
			tap(r, s.RemoveButton())
			r.Settle()
			assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
			assert.True(t, shows(r, toastOffline))
			assert.False(t, s.Busy())
			assert.False(t, c.Linked())
			r.Fake.SetOnline(true)
			r.Tester.Elapse(2300 * time.Millisecond) // the toast covers the button for 2200 ms
			tap(r, s.RemoveButton())
			r.Settle()
			assert.True(t, c.Linked(), "a second try goes through")
		})
		t.Run("code expired", func(t *testing.T) {
			r, c, s := atLimit(t, ph, false)
			r.Fake.Account().SetDeviceLimit(false) // the removal would lift it, and with it the 410
			r.Fake.Fail("tv.pair.confirm", 410, "errors/pair_code_expired.json")
			tap(r, s.RemoveButton())
			r.Settle()
			slideIn(r)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.False(t, c.Linked())
			assert.False(t, c.Boxes().Error, "the boxes do not go red (Q-P6.4-2)")
			assert.True(t, shows(r, toastCodeExpired))
			assert.Equal(t, "7K4M2P", c.Boxes().Value)
		})
		t.Run("limit again", func(t *testing.T) {
			r, c, s := atLimit(t, ph, false)
			r.Fake.Account().SetDeviceLimit(false)                          // the removal would lift it
			r.Fake.Fail("tv.pair.confirm", 409, "errors/device_limit.json") // the limit stays after the removal
			tap(r, s.RemoveButton())
			r.Settle()
			assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
			s2, ok := r.Shell.Nav().Sheet().Content().(*screens.DevicesSheet)
			require.True(t, ok)
			assert.Len(t, s2.Rows(), 2, "the rows are the server's list without the device removed")
			assert.False(t, c.Linked())
			assert.False(t, s2.Busy())
		})
	})
}

// TestDevicesSheetBoxesOnThePhones checks the sheet's measures on both phones (there is no
// prototype design): the rows and the buttons are as wide as the sheet's column, the rows
// are 10 apart, "Huỷ" is 48 tall.
func TestDevicesSheetBoxesOnThePhones(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, s := atLimit(t, ph, true)
		lay := r.Dump("devices")
		w := float64(ph.Size.W - 40)
		rows := s.Rows()
		var prev float64
		for i, row := range rows {
			e := element(t, lay, row.Name)
			assert.InDelta(t, 20, e.X, .01, row.Name+": x")
			assert.InDelta(t, w, e.W, .01, row.Name+": w")
			if i > 0 {
				assert.InDelta(t, 10, e.Y-prev, .01, row.Name+": the radios' margin-bottom")
			}
			prev = e.Y + e.H
		}
		for _, key := range []string{"devRemove", "closeSheet#3"} {
			e := element(t, lay, key)
			assert.InDelta(t, 20, e.X, .01, key+": x")
			assert.InDelta(t, w, e.W, .01, key+": w")
		}
		assert.InDelta(t, 54, element(t, lay, "devRemove").H, .01)
		assert.InDelta(t, 48, element(t, lay, "closeSheet#3").H, .01)
		panel := element(t, lay, "div.sheet")
		assert.InDelta(t, float64(ph.Size.H), panel.Y+panel.H, .01, "the panel rests on the bottom")
		assert.True(t, r.Tester.Shows("Máy này"))
	})
}
