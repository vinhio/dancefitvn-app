//go:build !tvos

package app

import (
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestStatesReachTheirScreen draws every state on both phones (fake shaper)
// as a developer's build does: through the real flow over the mock to its
// screen, then, once the app rested, its layout written to the cache
// folder (devshot_mobile.go).
func TestStatesReachTheirScreen(t *testing.T) {
	t.Parallel()
	skipLiveBuild(t)

	type check func(t *testing.T, r *rig)
	route := func(want string) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			assert.Equal(t, want, r.sh.Nav().Current().Screen)
		}
	}
	banner := func(t *testing.T, r *rig) {
		t.Helper()
		l, ok := r.sh.Nav().Screen().(*screens.Login)
		require.True(t, ok)
		title, _, shown := l.Banner()
		assert.True(t, shown)
		assert.NotEmpty(t, title)
		assert.False(t, l.Busy())
	}
	bio := func(want bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			l, ok := r.sh.Nav().Screen().(*screens.Login)
			require.True(t, ok)
			assert.Equal(t, want, l.BioShown())
			assert.Equal(t, "thaovy.hlv@gmail.com", l.Email().Text(), "the mock user's email is remembered")
		}
	}
	home := func(loaded bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			h, ok := r.sh.Nav().Screen().(*screens.Home)
			require.True(t, ok)
			assert.Equal(t, loaded, h.Content() != nil)
		}
	}
	// detail checks the course is in: the detail's column of a course the
	// user is in, the locked course's of one they are not; never the skeleton.
	detail := func(locked bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			d, ok := r.sh.Nav().Screen().(*screens.Detail)
			require.True(t, ok)
			assert.Equal(t, !locked, d.Content() != nil, "the detail's column")
			assert.Equal(t, locked, d.Locked() != nil, "the locked course's column")
		}
	}
	// video checks screen 06 shows, with its controls shown or hidden and the
	// options sheet open or not.
	video := func(shown, sheet bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			v, ok := r.sh.Nav().Screen().(*screens.Video)
			require.True(t, ok)
			assert.Equal(t, shown, v.Shown(), "the controls")
			assert.Equal(t, sheet, r.sh.Nav().Sheet().IsOpen(), "the options sheet")
		}
	}
	// playing checks the player plays (the controls of such a state hide at 3 s of
	// the view's time, which the test's driver runs ahead of the clock of a device).
	playing := func(want bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			pb := r.sh.Env().Playback()
			require.NotNil(t, pb)
			assert.Equal(t, want, pb.Playing())
			assert.Equal(t, 72.0, pb.Pos(), "01:12, as the prototype's M06")
		}
	}
	// audio checks screen 08 shows the sound of lesson 3 paused at 01:12, speed .75,
	// beats on, loop off (the prototype's M08): 23 bars of the wave are lit.
	audio := func(t *testing.T, r *rig) {
		t.Helper()
		a, ok := r.sh.Nav().Screen().(*screens.Audio)
		require.True(t, ok)
		pb := r.sh.Env().Playback()
		require.NotNil(t, pb)
		assert.Equal(t, player.KindAudio, pb.Kind())
		assert.Equal(t, "l_3503", pb.Lesson().ID)
		assert.False(t, pb.Playing())
		assert.Equal(t, 72.0, pb.Pos())
		assert.Equal(t, 220.0, pb.Dur())
		assert.Equal(t, .75, pb.Rate())
		assert.True(t, pb.Beat())
		assert.False(t, pb.Loop())
		assert.Equal(t, 23, a.WaveLit(), "bars 0 to 22 at 72 s of 220")
	}
	// done checks screen 10 shows after lesson 3 with its countdown held at 5, the
	// player at 01:12 so that the lesson is not watched (the prototype's M10).
	done := func(t *testing.T, r *rig) {
		t.Helper()
		d, ok := r.sh.Nav().Screen().(*screens.Done)
		require.True(t, ok)
		assert.Equal(t, 5, d.Seconds())
		assert.False(t, d.Counting(), "the countdown is held")
		pb := r.sh.Env().Playback()
		require.NotNil(t, pb)
		assert.Equal(t, 72.0, pb.Pos())
		assert.Equal(t, "l_3503", pb.Lesson().ID)
	}
	offline := func(t *testing.T, r *rig) {
		t.Helper()
		assert.False(t, r.w.Online())
	}
	forgot := func(t *testing.T, r *rig) {
		t.Helper()
		calls := r.opened()
		require.Len(t, calls, 1)
		assert.Equal(t, "https://dancefitvn.com/quen-mat-khau", calls[0].raw, "app/config's links.forgot_password")
	}
	// sheet checks a sheet (download, delete) is open over the route.
	sheet := func(t *testing.T, r *rig) {
		t.Helper()
		assert.True(t, r.sh.Nav().Sheet().IsOpen(), "the sheet")
	}
	// devices checks the sheet of devices of a sign-in at the limit (K31) over the login: the
	// two phones of the error, the device chosen (none for "").
	devices := func(chosen string) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			s, ok := r.sh.Nav().Sheet().Content().(*screens.DevicesSheet)
			require.True(t, ok, "the sheet of devices")
			assert.True(t, s.Login())
			assert.Len(t, s.Rows(), 2)
			assert.Equal(t, chosen, s.Chosen())
		}
	}
	// privacy checks that screen 15 shows the row of the privacy policy (K32), or not.
	privacy := func(shown bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			a, ok := r.sh.Nav().Screen().(*screens.Account)
			require.True(t, ok)
			assert.Equal(t, shown, a.PrivacyShown())
		}
	}
	// files checks the downloads the screens read: the demo's, or none.
	files := func(items, jobs int) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			dm := r.sh.Env().Downloader()
			assert.Len(t, dm.Items(), items)
			assert.Len(t, dm.Jobs(), jobs)
		}
	}
	// tv checks whether the account shows its Apple TV linked (the kept list of
	// devices).
	tv := func(linked bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			v := r.sh.Env().AccountView()
			assert.True(t, v.HasMe && v.HasDevices, "the account read")
			_, ok := r.sh.Env().TVDevice()
			assert.Equal(t, linked, ok, "the TV linked")
		}
	}
	// linked checks the state screen 16 shows.
	linked := func(want bool) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			c, ok := r.sh.Nav().Screen().(*screens.Connect)
			require.True(t, ok)
			assert.Equal(t, want, c.Linked(), "16 linked")
		}
	}
	// noBanner checks the login after a sign-out the user asked for.
	noBanner := func(t *testing.T, r *rig) {
		t.Helper()
		l, ok := r.sh.Nav().Screen().(*screens.Login)
		require.True(t, ok)
		_, _, shown := l.Banner()
		assert.False(t, shown)
		_, signed := r.w.mgr.Session()
		assert.False(t, signed)
	}
	// toast checks the toast showing.
	toast := func(msg string) check {
		return func(t *testing.T, r *rig) {
			t.Helper()
			assert.Equal(t, msg, r.sh.Nav().Toasts().Message())
		}
	}
	// infoAgain checks that 15B, opened again, shows the name saved (the overlay).
	infoAgain := func(t *testing.T, r *rig) {
		t.Helper()
		f, ok := r.sh.Nav().Screen().(*screens.Info)
		require.True(t, ok)
		assert.Equal(t, infoFirst, f.First().Text())
		assert.Equal(t, infoLast, f.Last().Text())
	}
	// uploaded checks the avatar sent through sheet 15D: the URL PUT /me/avatar
	// answered is the kept user's, and the image cache holds the picture.
	uploaded := func(t *testing.T, r *rig) {
		t.Helper()
		url := r.sh.Env().AccountView().Me.User.AvatarURL
		_, ok := r.w.images.Image(url)
		assert.True(t, ok, "primed: %s", url)
		assert.False(t, r.sh.Nav().Sheet().IsOpen())
	}
	tests := map[string][]check{
		"M15":             {route(env.RouteAccount), tv(false), privacy(false)},
		"M15-privacy":     {route(env.RouteAccount), tv(false), privacy(true)},
		"M15B":            {route(env.RouteInfo), tv(false)},
		"M15B-password":   {route(env.RouteAccount), sheet},
		"M15C":            {route(env.RouteCoach)},
		"M15D":            {route(env.RouteAccount), sheet},
		"M15-logout":      {route(env.RouteAccount), sheet},
		"M16":             {route(env.RouteConnect), tv(false)},
		"M16-code":        {route(env.RouteConnect), tv(false)},
		"M16-linked":      {route(env.RouteConnect), tv(true)},
		"PAIR":            {route(env.RouteConnect), tv(true), linked(true)},
		"PAIR-CHECK":      {route(env.RouteConnect), tv(true), linked(true)},
		"M-toast":         {route(env.RouteDetail), toast("Đã tải xong bài 03")},
		"AVATAR-PICK":     {route(env.RouteAccount), uploaded},
		"INFO-SAVE":       {route(env.RouteInfo), infoAgain},
		"LOGOUT":          {route(env.RouteLogin), noBanner},
		"ACCOUNT":         {route(env.RouteAccount)},
		"OVERLAY-RESET":   {route(env.RouteAccount), tv(true)},
		"M09":             {route(env.RouteDetail), detail(false), sheet, files(5, 1)},
		"M11":             {route(env.RouteDownloads), files(5, 1)},
		"M12":             {route(env.RouteDownloads), files(0, 0)},
		"M13":             {route(env.RouteOffline), offline, files(5, 1)},
		"M14":             {route(env.RouteDownloads), sheet, files(5, 1)},
		"M01":             {route(env.RouteSplash)},
		"M02":             {route(env.RouteLogin), bio(true)},
		"M02B":            {route(env.RouteLogin), banner, bio(true)},
		"M02-locked":      {banner},
		"M02-device":      {banner, sheet, devices("")},
		"M02-device-pick": {banner, sheet, devices("d_102")},
		"M02-offline":     {banner, offline},
		"M02-expired":     {banner},
		"M03B":            {route(env.RouteHome), home(false)},
		"M03":             {route(env.RouteHome), home(true)},
		"M03-offline":     {home(true), offline},
		"M04":             {route(env.RouteCourses)},
		"M04-offline":     {route(env.RouteCourses), offline},
		"M04-special":     {route(env.RouteCourses)},
		"M04-empty":       {route(env.RouteCourses)},
		"M05":             {route(env.RouteDetail), detail(false)},
		"M05B":            {route(env.RouteDetail), detail(true)},
		"M06":             {route(env.RouteVideo), video(true, false), playing(false)},
		"M06-playing":     {route(env.RouteVideo), playing(true)},
		"M06-hidden":      {route(env.RouteVideo), video(false, false), playing(true)},
		"M07":             {route(env.RouteVideo), video(true, true), playing(false)},
		"M08":             {route(env.RouteAudio), audio},
		"M10":             {route(env.RouteDone), done},
		"UPGRADE":         {route(env.RouteUpgrade)},
		"LOGIN":           {route(env.RouteHome), home(true)},
		"FORGOT":          {route(env.RouteLogin), forgot},
	}
	// The smoke and performance states of a developer's build
	// (devsmoke_mobile.go, devperf_mobile.go) play through the system's
	// engine or run for minutes: the simulator runs them.
	ids := slices.DeleteFunc(stateIDs(), func(id string) bool {
		return strings.HasPrefix(id, "SMOKE-") || strings.HasPrefix(id, "PERF-")
	})
	assert.ElementsMatch(t, ids, mapKeys(tests), "every state is checked")
	for id, checks := range tests {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := newRig(t, nil, "")
				r.deps.PickImage = (&fakePicker{data: solidJPEG(t, 640, 480, color.RGBA{R: 200, A: 255})}).pick
				r.mount(ph, id, false)
				file := filepath.Join(must(r.deps.CacheDir()), shotFolder, id+".layout.json")
				r.drive("the layout of "+id, func() bool {
					_, err := os.Stat(file)

					return err == nil
				})
				for _, c := range checks {
					c(t, r)
				}
				lay := readLayout(t, file)
				assert.Equal(t, id, lay.ID)
				assert.InDelta(t, ph.Size.W, lay.Screen.W, 0)
				assert.NotEmpty(t, lay.Elements)
			})
		})
	}
}

// TestLookupState checks an unknown state's error, which lists the known.
func TestLookupState(t *testing.T) {
	t.Parallel()
	skipLiveBuild(t)

	_, err := lookupState("M99")
	require.ErrorIs(t, err, core.ErrNotFound)
	assert.Contains(t, err.Error(), "M02B")
	st, err := lookupState("M01")
	require.NoError(t, err)
	assert.True(t, st.hold)
}

// TestLogLayout checks the base64 lines of the fallback of a phone whose
// cache folder cannot be read.
func TestLogLayout(t *testing.T) {
	t.Parallel()

	l := &testLog{}
	lay := layoutdump.Layout{ID: "M02", Elements: make([]layoutdump.Element, 40)}
	logLayout(lay, l.add)
	lines := l.all()
	require.Greater(t, len(lines), 1)
	assert.Contains(t, lines[0], "dfv:L M02 1/")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mapKeys returns the keys of m.
func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

// readLayout reads a layout.json.
func readLayout(t *testing.T, path string) layoutdump.Layout {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var lay layoutdump.Layout
	require.NoError(t, jsonUnmarshal(data, &lay))

	return lay
}
