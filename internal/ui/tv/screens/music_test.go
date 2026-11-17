//go:build tvos

package screens_test

import (
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// audioState is TV09's state: the audio of lesson 3 at 72 of 220 seconds, paused.
func audioState(t testing.TB) *tvtest.Services {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	svc.PB.(*tvtest.Playback).KindV = player.KindAudio

	return svc
}

// music returns the music screen showing.
func (r *rig) music() *screens.Music {
	r.t.Helper()
	m, ok := r.sh.Screen().(*screens.Music)
	require.True(r.t, ok, "the screen showing is TV09")

	return m
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMusicBoxesMatchPrototype compares every element of TV09.layout.json: the nav
// bar, the cover, the lesson, the clock and the three buttons (`mu0` focused).
func TestMusicBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, audioState(t), env.RouteMusic, true)
	r.settle()
	require.Equal(t, "mu0", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxes(t, "TV09")
}

// squash folds the no-break spaces of a text into plain ones, as the capture's matcher
// folds white space.
func squash(s string) string { return strings.ReplaceAll(s, "\u00a0", " ") }

// sweepNames are course names of every length from 20 to 60 letters in words of five,
// so that some of them end the kicker's line just before its number.
func sweepNames() []string {
	var names []string
	for n := 20; n <= 60; n++ {
		var b strings.Builder
		for i := 0; i < n; i++ {
			if i%6 == 5 {
				b.WriteByte(' ')
			} else {
				b.WriteByte('N')
			}
		}
		names = append(names, strings.TrimSpace(b.String()))
	}

	return names
}

// TestMusicKickerKeepsTheNumberWithItsText is defect 3.7: with a course name that makes
// the kicker wrap, its last line holds the number with its text ("BÀI 02"), never the
// number alone.
func TestMusicKickerKeepsTheNumberWithItsText(t *testing.T) {
	t.Parallel()
	for _, name := range append([]string{
		"DanceFIT® Set 35: Cùng nhảy cùng hát và một tên rất dài nữa",
		"DanceFIT Set 35 Warm Up Cardio Extra Dài Quá Mức",
		"DanceFIT Set 35 một hai ba bốn năm sáu bảy tám chín mười",
		"Set 35",
	}, sweepNames()...) {
		svc := audioState(t)
		svc.CourseData.Name = name
		pb := svc.PB.(*tvtest.Playback)
		pb.LessonV.CourseName = name
		pb.LessonV.No = 2
		r := openFast(t, svc, env.RouteMusic, true)
		r.settle()
		ly := r.music().Eyebrow().TextLayout()
		require.NotNil(t, ly, name)
		for i := range ly.Lines {
			line := squash(ly.Text[ly.Lines[i].TextStart:])
			if i+1 < len(ly.Lines) {
				line = squash(ly.Text[ly.Lines[i].TextStart:ly.Lines[i+1].TextStart])
			}
			assert.NotEqual(t, "02", strings.TrimSpace(line), "%q: the number alone on line %d", name, i)
			assert.NotEqual(t, "BÀI", strings.TrimSpace(line), "%q: the word alone on line %d", name, i)
		}
		last := squash(ly.Text[ly.Lines[len(ly.Lines)-1].TextStart:])
		assert.Contains(t, last, "BÀI 02", name)
	}
}

// TestMusicText: the strings are the prototype's, filled from the lesson.
func TestMusicText(t *testing.T) {
	t.Parallel()
	r := openFast(t, audioState(t), env.RouteMusic, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"tv.music.eyebrow": "FILE ÂM THANH · DANCEFIT SET 35 · BÀI 03",
		"div.d":            "STINK AND DUTTY",
		"p.muted":          "Bụng · 03:40",
		"div.mono.muted":   "01:12\n03:40",
		"mu0":              "‹ Bài trước",
		"mu2":              "Bài sau ›",
	}
	for key, txt := range want {
		assert.Equal(t, txt, squash(find(t, lay, key).Text), key)
	}
	assert.Equal(t, icon.Play, r.music().Toggle().Glyph())
	assert.Equal(t, 40, r.music().Wave().Lit(), "the bars i of 120 with i/120 <= 72/220 are lit: 0 to 39")
}

// TestMusicTransport: `mu0` and `mu2` ask for the lesson before and after, `mu1`
// toggles, the Play/Pause key does too (this screen only), and the screen follows
// the player.
func TestMusicTransport(t *testing.T) {
	t.Parallel()
	r := openFast(t, audioState(t), env.RouteMusic, true)
	r.settle()
	pb := r.playback()
	require.Equal(t, "mu0", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Prev"])
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "mu1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Toggle"])
	pb.Publish(player.ChangeState)
	assert.Equal(t, icon.Pause, r.music().Toggle().Glyph())
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "mu2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Next"])
	assert.True(t, r.press(keys.KeyMediaPlayPause))
	assert.Equal(t, 2, pb.Calls["Toggle"])
	pb.Publish(player.ChangeState)
	assert.Equal(t, icon.Play, r.music().Toggle().Glyph())

	// the player moves on to lesson 4: the texts follow
	pb.LessonV = env.PlayerLessons(r.svc.(*tvtest.Services).CourseData)[3]
	pb.DurV = 250
	pb.PosV = 0
	pb.Publish(player.ChangeLesson)
	r.tt.Elapse(tvtest.Frame)
	lay := r.layout()
	assert.Equal(t, "EM YÊU NGƯỜI TA", find(t, lay, "div.d").Text)
	assert.Equal(t, "Bachata · 04:10", find(t, lay, "p.muted").Text)
	assert.Equal(t, "FILE ÂM THANH · DANCEFIT SET 35 · BÀI 04", squash(find(t, lay, "tv.music.eyebrow").Text))
	assert.Empty(t, r.tt.Leaks())
}

// TestMusicEdgeLessonsAskForNothing: `mu0` at the first lesson and `mu2` at the
// last do nothing (the prototype's `l.n>1` and `l.n<lessons`).
func TestMusicEdgeLessonsAskForNothing(t *testing.T) {
	t.Parallel()
	svc := audioState(t)
	pb := svc.PB.(*tvtest.Playback)
	pb.Prev_, pb.Next_ = false, false
	r := openFast(t, svc, env.RouteMusic, false)
	r.settle()
	require.True(t, r.press(keys.KeyEnter))
	require.True(t, r.press(keys.KeyRight))
	require.True(t, r.press(keys.KeyRight))
	require.True(t, r.press(keys.KeyEnter))
	assert.Zero(t, pb.Calls["Prev"]+pb.Calls["Next"])
}

// TestMusicFromTheNavTakesTheVideosLesson: arriving with a video loaded, the screen
// asks for the lesson's audio at the video's position and pauses it unless the video
// was playing (Q-P7-10, index.html:735-736).
func TestMusicFromTheNavTakesTheVideosLesson(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t) // a paused video at 72 s
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	pb := r.playback()
	r.sh.SwitchTab(2)
	r.settle()
	assert.Equal(t, 1, pb.Calls["PlayAudio"])
	assert.Equal(t, player.KindAudio, pb.KindV)
	assert.InDelta(t, 72, pb.PosV, 0)
	assert.Equal(t, 1, pb.Calls["Pause"], "it was not playing")
	assert.Equal(t, "nav2", r.key(), "the focus stays on the nav item")
	assert.True(t, r.music().Loaded())

	// a video that plays goes on as audio
	svc2 := tvtest.NewPlayerServices(t)
	r2 := openFast(t, svc2, env.RouteHome, false)
	r2.settle()
	svc2.PB.(*tvtest.Playback).PlayingV = true
	r2.sh.SwitchTab(2)
	r2.settle()
	assert.Zero(t, svc2.PB.(*tvtest.Playback).Calls["Pause"])
	assert.True(t, svc2.PB.Playing())
}

// TestMusicWithNothingLoadedTakesTheHomesLesson: with no lesson loaded the audio is
// the one the home continues, at its position, paused (Q-P7-10).
func TestMusicWithNothingLoadedTakesTheHomesLesson(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	svc.PB = &tvtest.Playback{}
	r := openFast(t, svc, env.RouteMusic, false)
	r.settle()
	pb := r.playback()
	assert.Equal(t, 1, pb.Calls["PlayAudio"])
	assert.Equal(t, "l_3503", pb.LessonV.ID)
	assert.InDelta(t, 72, pb.PosV, 0)
	assert.Equal(t, 1, pb.Calls["Pause"])
	assert.True(t, r.music().Loaded())
}

// TestMusicWaitsWithTheNavBar: until the audio is there the screen is the nav bar
// over the background and `mu0`'s stand-in has the focus.
func TestMusicWaitsWithTheNavBar(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t) // a video: the lessons are read before the audio starts
	r := openFast(t, svc, env.RouteMusic, false)
	m := r.music()
	assert.False(t, m.Loaded())
	assert.Nil(t, m.Item("mu1"))
	assert.Equal(t, "mu0", r.key())
	r.settle()
	assert.True(t, m.Loaded())
	assert.True(t, m.Item("mu0").IsFocused(), "the real mu0 has the focus")
}

// TestMusicTickDoesNotAllocate: a tick of the player on the music screen (the wave,
// the clock) and its paint allocate nothing once the seconds have been shown.
func TestMusicTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, audioState(t), env.RouteMusic, false)
	r.settle()
	pb := r.playback()
	pos := 0.0
	step := func() {
		pos += .25
		if pos >= 220 {
			pos = 0
		}
		pb.Tick(pos)
		r.tt.Elapse(16 * time.Millisecond)
	}
	for range 4 * 221 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step), "a tick")
}

// TestWaveTickDoesNotAllocate: lighting bars and painting the wave allocate nothing.
func TestWaveTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, audioState(t), env.RouteMusic, false)
	r.settle()
	w := r.music().Wave()
	share := float32(0)
	step := func() {
		share += .004
		if share > 1 {
			share = 0
		}
		w.SetPos(share)
		r.tt.Elapse(16 * time.Millisecond)
	}
	for range 400 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step))
	r.tt.Elapse(time.Second)
	assert.Zero(t, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), "a steady frame")
}
