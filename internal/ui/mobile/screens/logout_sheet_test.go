//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atLogoutSheet opens SH.logout over Home (screen 15 is a placeholder until P6.1)
// and lets it slide in.
func atLogoutSheet(tb testing.TB, ph kittest.Phone, real bool) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	r := shelltest.MountRig(tb, ph, real, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	r.Shell.AskLogout()
	r.Settle()
	slideIn(r)
	require.True(tb, r.Shell.Nav().Sheet().IsOpen())

	return r
}

// TestLogoutSheetBoxesMatchPrototype lays SH.logout out with the iPhone's real
// text engine and compares the elements of the sheet of M15-logout with the
// prototype's within 1 DIP: the panel, the question, the line, the two buttons.
// What is behind the dim is screen 15, not this test's.
func TestLogoutSheetBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atLogoutSheet(t, kittest.Phones[0], true)
	d := r.Dump("M15-logout")
	want := shelltest.ProtoLayout(t, "M15-logout")
	panel, ok := d.Find("div.sheet")
	require.True(t, ok)
	for _, key := range []string{"div.sheet", "div.d#2", "p.muted#2", "logout", "closeSheet#2"} {
		w, ok := want.Find(key)
		require.True(t, ok, key)
		// the names behind the dim repeat (Home has a div.d#2 of its own): the
		// element is the one inside the panel
		found := false
		for _, g := range d.Elements {
			if g.Key != key || g.Y < panel.Y-.5 {
				continue
			}
			found = true
			assert.InDelta(t, w.X, g.X, 1, key+" x")
			assert.InDelta(t, w.Y, g.Y, 1, key+" y")
			assert.InDelta(t, w.W, g.W, 1, key+" w")
			assert.InDelta(t, w.H, g.H, 1, key+" h")
		}
		assert.True(t, found, key)
	}
}

// TestLogoutSheetActions checks the buttons: the red "Đăng xuất" is `logout` (the
// sign-out, the login with no banner), "Huỷ" and the dim close the sheet and sign
// nothing out.
func TestLogoutSheetActions(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("texts", func(t *testing.T) {
			r := atLogoutSheet(t, ph, false)
			_, ok := r.Shell.Nav().Sheet().Content().(*screens.LogoutSheet)
			require.True(t, ok)
			for _, text := range []string{"Đăng xuất?", "File âm thanh đã tải vẫn được giữ trên máy.", "Đăng xuất", "Huỷ"} {
				assert.True(t, r.Tester.Shows(text), text)
			}
		})
		t.Run("logout", func(t *testing.T) {
			r := atLogoutSheet(t, ph, false)
			tap(r, named(r.Shell, "logout", 1))
			r.Settle()
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, env.RouteLogin, r.Route())
			assert.True(t, r.Fake.Account().LogoutBegan())
		})
		t.Run("cancel", func(t *testing.T) {
			r := atLogoutSheet(t, ph, false)
			tap(r, named(r.Shell, "closeSheet#2", 1))
			r.Settle()
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, "home", r.Route())
			assert.False(t, r.Fake.Account().LogoutBegan())
		})
		t.Run("dim", func(t *testing.T) {
			r := atLogoutSheet(t, ph, false)
			tap(r, named(r.Shell, "closeSheet", 1))
			r.Settle()
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.False(t, r.Fake.Account().LogoutBegan())
		})
	})
}

// TestLogoutSheetSteadyFrameDoesNotAllocate checks that a frame of the open sheet
// standing still allocates nothing, on both phones.
func TestLogoutSheetSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atLogoutSheet(t, ph, false)
		r.Tester.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
