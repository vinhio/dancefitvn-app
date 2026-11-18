//go:build tvos

package screens_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settings returns the settings screen showing.
func (r *rig) settings() *screens.Settings {
	r.t.Helper()
	s, ok := r.sh.Screen().(*screens.Settings)
	require.True(r.t, ok, "the screen showing is TV12")

	return s
}

// TestSettingsBoxesMatchPrototype compares every element of TV12.layout.json: the
// nav bar (nav0 focused, as the rule of the settings screen has it), the name, the
// email, the badges' heading and the six rows.
func TestSettingsBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, playerState(t), env.RouteSettings, true) // the capture has the beat bar on
	r.settle()
	require.Equal(t, "nav0", r.key())
	r.tt.Elapse(tvkit.Transition)
	// the width of "Tự động" at 28 px is 2.1 px under Chrome's (the stacked accents
	// of "ự"), which the width of the title that gives way to it follows
	r.boxesLoose(t, "TV12", map[string]float32{"span.tt": 2.5, "b": 2.5})
}

// TestSettingsText: the strings are the prototype's, filled from GET /me, the
// preferences and the support's config; the version line is the app's (K2).
func TestSettingsText(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewPlayerServices(t), env.RouteSettings, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"div.d":               "NGUYỄN THẢO VY",
		"tv.settings.since":   "Thành viên từ 12/03/2024",
		"div.muted":           "thaovy.hlv@gmail.com",
		"div.eb":              "HUY HIỆU",
		"s0":                  "Chất lượng video\nTự động",
		"s1":                  "Tự phát bài tiếp theo\nBật",
		"s2":                  "Hiển thị đếm nhịp\nTắt",
		"s3":                  "Ngôn ngữ\nTiếng Việt",
		"s4":                  "Hỗ trợ\n036 800 8268",
		"s5":                  "Đăng xuất khỏi Apple TV",
		"tv.settings.hint":    "Sửa hồ sơ trên điện thoại hoặc website",
		"tv.settings.version": "DanceFITVN cho Apple TV · Phiên bản 0.1.0",
	}
	for key, txt := range want {
		assert.Equal(t, txt, find(t, lay, key).Text, key)
	}
	assert.Len(t, r.settings().Card().Medals(), 3)
}

// TestSettingsBadgesStayOnTheCard is defect 3.1: with any number of badges, the medals
// (with their captions) and the hint stay inside the 600 by 840 card, apart from one
// another on a page and short of the settings list; they are the prototype's 140 DIPs
// at any number, the ones past three taking turns (K44, medal_row_test.go).
func TestSettingsBadgesStayOnTheCard(t *testing.T) {
	t.Parallel()
	names := []string{"HDV_DANCEFIT", "HDV_STEPX", "HDV_BODYX", "FOUNDER_CEO", "DANCEFIT_MASTER_TRAINER", "HDV_ZUMBA", "HDV_AEROBIC", "HDV_YOGA"}
	for n := 1; n <= len(names); n++ {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewPlayerServices(t)
			svc.MeData.Badges = nil
			for _, c := range names[:n] {
				svc.MeData.Badges = append(svc.MeData.Badges, model.Badge{Code: strings.ToLower(c), Name: c})
			}
			r := openFast(t, svc, env.RouteSettings, true)
			r.settle()
			card := r.settings().Card()
			cf := card.Node().WindowFrame()
			medals := card.Medals()
			require.Len(t, medals, n)
			prev := cf.X
			last := cf.Y
			row := card.Row()
			for i, m := range medals {
				f := m.Node().WindowFrame()
				if i == 0 || row.PageOf(i) != row.PageOf(i-1) {
					prev = cf.X // a new page starts over from the card's edge
				}
				last = max(last, f.Y+f.H)
				assert.GreaterOrEqual(t, f.X, cf.X, "medal %d inside the card", i)
				assert.LessOrEqual(t, f.X+f.W, cf.X+cf.W+.5, "medal %d inside the card", i)
				assert.LessOrEqual(t, f.Y+f.H, cf.Y+cf.H, "medal %d above the card's end", i)
				assert.GreaterOrEqual(t, f.X, prev-.5, "medal %d clear of the one before", i)
				prev = f.X + f.W
				assert.InDelta(t, tvkit.MedalSize, m.Size(), .01, "the prototype's size")
			}
			hint, ok := r.layout().Find("tv.settings.hint")
			require.True(t, ok)
			assert.LessOrEqual(t, float32(hint.Y+hint.H), cf.Y+cf.H, "the hint on the card")
			assert.LessOrEqual(t, last, float32(hint.Y)+.5, "the medals over the hint")
			list, ok := r.layout().Find("s0")
			require.True(t, ok)
			for _, m := range medals {
				f := m.Node().WindowFrame()
				assert.Less(t, f.X+f.W, float32(list.X), "short of the settings list")
			}
		})
	}
}

// TestSettingsRows: `s0` cycles the quality, `s1` the next-lesson preference, `s2`
// the beat bar; `s3` and `s4` do nothing.
func TestSettingsRows(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := openFast(t, svc, env.RouteSettings, false)
	r.settle()
	prefs := svc.PrefsV.(*tvtest.Prefs)
	pb := r.playback()
	value := func(k string) string { return lastLine(find(t, r.layout(), k).Text) }

	require.True(t, r.press(keys.KeyDown))
	require.Equal(t, "s0", r.key())
	for _, want := range []string{"1080p", "720p", "Tự động"} {
		require.True(t, r.press(keys.KeyEnter))
		assert.Equal(t, want, value("s0"))
	}
	assert.Equal(t, 3, prefs.Cycles)
	prefs.Q = store.QualityP480
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, "Tự động", value("s0"), "a stored 480p goes to Tự động")

	require.True(t, r.press(keys.KeyDown))
	require.Equal(t, "s1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.False(t, prefs.Auto)
	assert.Equal(t, "Tắt", value("s1"))
	require.True(t, r.press(keys.KeyEnter))
	assert.True(t, prefs.Auto)
	assert.Equal(t, "Bật", value("s1"))

	require.True(t, r.press(keys.KeyDown))
	require.Equal(t, "s2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["SetBeat"])
	assert.True(t, pb.BeatV)
	assert.Equal(t, "Bật", value("s2"))

	for _, k := range []string{"s3", "s4"} {
		require.True(t, r.press(keys.KeyDown))
		require.Equal(t, k, r.key())
		r.press(keys.KeyEnter)
	}
	assert.Equal(t, env.RouteSettings, r.sh.Stack().Screen())
	assert.Equal(t, 4, prefs.Cycles)
}

// TestSettingsLeftGoesToTheActiveNavItem: Left from every row lands on `nav3`, the
// item that is on, as the prototype's pick does (the layout's nearest would be
// `nav1`).
func TestSettingsLeftGoesToTheActiveNavItem(t *testing.T) {
	t.Parallel()
	for i, k := range []string{"s0", "s1", "s2", "s3", "s4", "s5"} {
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			r := openFast(t, tvtest.NewPlayerServices(t), env.RouteSettings, false)
			r.settle()
			for range i + 1 {
				require.True(t, r.press(keys.KeyDown))
			}
			require.Equal(t, k, r.key())
			require.True(t, r.press(keys.KeyLeft))
			assert.Equal(t, "nav3", r.key(), "Left from "+k)
			assert.True(t, r.settings().Nav().Item(3).IsFocused())
		})
	}
}

// TestSettingsLogout: `s5` signs the TV out and the welcome screen is the root with
// an empty stack, on the same step and whatever the answer: the call to the services
// is made once and goes on while TV01 is up, as the prototype's `tlogout` does not
// wait (index.html:748).
func TestSettingsLogout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
	}{{"signed out", nil}, {"the server failed", assert.AnError}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewPlayerServices(t)
			r := openFast(t, svc, env.RouteSettings, false)
			r.settle()
			svc.ReadErr = nil
			release := svc.HoldLogout() // a slow flush and server call
			for range 6 {
				r.press(keys.KeyDown)
			}
			require.Equal(t, "s5", r.key())
			svc.ReadErr = tc.err
			require.True(t, r.tt.Key(keys.KeyEnter, 0))
			// the same step: TV01 is the root while the call has not answered
			assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
			assert.Zero(t, r.sh.Stack().Depth())
			r.tt.Elapse(tvtest.Frame)
			assert.Equal(t, "w1", r.key())
			select {
			case <-svc.LogoutDone():
				t.Fatal("the call finished before it was released")
			default:
			}
			release()
			select {
			case <-svc.LogoutDone():
			case <-time.After(5 * time.Second):
				t.Fatal("the sign-out was cancelled with the screen")
			}
			r.settle()
			assert.Equal(t, 1, svc.Logouts())
			assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
		})
	}
}

// TestSettingsReadFailureShowsTheError: a user that cannot be read shows TV11 (K28).
func TestSettingsReadFailureShowsTheError(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	svc.ReadErr = assert.AnError
	r := openFast(t, svc, env.RouteSettings, false)
	r.settle()
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
}

// TestSettingsLogoutAlwaysLandsOnWelcome is defect 3.4: after "Đăng xuất khỏi Apple TV"
// the welcome screen is the root, whatever the press does next (a held Select, a
// repeated or late press of the same click); only a press after the screen's guard
// goes on to the code screen, and then back from it is TV01 again, over any number of
// sign-outs.
func TestSettingsLogoutAlwaysLandsOnWelcome(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := openFast(t, svc, env.RouteSettings, false)
	r.settle()
	for round := range 3 {
		for range 6 {
			r.press(keys.KeyDown)
		}
		require.Equal(t, "s5", r.key(), "round %d", round)
		switch round {
		case 0: // a tap, then the same click's second press at once
			r.tt.Key(keys.KeyEnter, 0)
			r.tt.Key(keys.KeyEnter, 0)
		case 1: // a hold
			r.tt.KeyDown(keys.KeyEnter, 0)
			r.tt.Elapse(700 * time.Millisecond)
			r.tt.KeyUp(keys.KeyEnter, 0)
		default: // a tap, and the second press a moment later
			r.tt.Key(keys.KeyEnter, 0)
			r.tt.Elapse(2 * tvtest.Frame)
			r.tt.Key(keys.KeyEnter, 0)
		}
		r.tt.Elapse(tvtest.Frame)
		assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen(), "round %d: TV01", round)
		assert.Zero(t, r.sh.Stack().Depth(), "round %d: the root", round)
		assert.Equal(t, "w1", r.key())
		assert.Equal(t, 1, svc.Logouts()-round, "round %d: signed out once", round)

		// a deliberate Select, after the guard, opens the code screen; Back is TV01
		r.armed()
		require.True(t, r.press(keys.KeyEnter))
		assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
		require.True(t, r.press(keys.KeyEscape))
		assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
		assert.Zero(t, r.sh.Stack().Depth())

		// signed in again by the phone: the settings, the stack empty
		r.sh.Go(nav.Route{Screen: env.RouteSettings}, true)
		r.settle()
		require.Zero(t, r.sh.Stack().Depth())
	}
}
