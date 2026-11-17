//go:build !tvos

package env_test

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nilDl is a Services that has no downloads (Downloads returns nil), noPrefs one
// that has no Prefs, and noFg one that is no Foregrounder (only the interface's
// methods show through the embedded Services).
type (
	nilDl   struct{ *shelltest.Fake }
	noPrefs struct{ *shelltest.Fake }
	noFg    struct{ env.Services }
)

func (nilDl) Downloads() env.DownloadManager { return nil }
func (noPrefs) Prefs() env.Prefs             { return nil }

// newFake returns a Fake over the repository's mock.
func newFake(t *testing.T) *shelltest.Fake {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)

	return f
}

// TestDownloaderFindsTheManager checks that Env.Downloads and Env.Downloader are
// the manager the services' Downloads() is, and a manager with nothing in it for
// services that return nil: the lists are empty, Start and Delete refuse, and the
// same value comes back each time.
func TestDownloaderFindsTheManager(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	texts := locales.MustTexts()

	t.Run("a manager", func(t *testing.T) {
		t.Parallel()

		e := env.New(f, texts)
		assert.Same(t, f.Dl(), e.Downloader())
		assert.Same(t, f.Dl(), e.Downloads())
		assert.Len(t, e.Downloader().Items(), 5)
		assert.Equal(t, 5, e.Downloads().DoneCount())
	})
	t.Run("none at all", func(t *testing.T) {
		t.Parallel()

		e := env.New(nilDl{f}, texts)
		dm := e.Downloads()
		require.NotNil(t, dm)
		st, p := dm.State("l_3501")
		assert.Equal(t, kit.DlNone, st)
		assert.Zero(t, p)
		assert.Zero(t, dm.DoneCount())
		assert.Empty(t, dm.Items())
		assert.Empty(t, dm.Jobs())
		assert.Zero(t, dm.TotalBytes())
		_, _, ok := dm.FreeSpace()
		assert.False(t, ok)
		assert.ErrorIs(t, dm.Start(model.Course{}, nil), core.ErrUnsupported)
		assert.ErrorIs(t, dm.Delete("l_3501"), core.ErrUnsupported)
		dm.Pause()
		dm.Resume()
		dm.Forget("l_3501")
		dm.RefreshSpace()
		require.NotNil(t, dm.Events())
		dm.Events().Publish(env.DownloadEvent{}) // nobody listens, nothing happens
		assert.Same(t, dm, e.Downloader())
	})
}

// TestForeground checks Env.Foreground: the services' answer when they are a
// Foregrounder, in front when they are not.
func TestForeground(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	texts := locales.MustTexts()

	t.Run("a foregrounder follows the app", func(t *testing.T) {
		t.Parallel()

		g := newFake(t)
		e := env.New(g, texts)
		assert.True(t, e.Foreground(), "in front at first")
		g.SetForeground(false)
		assert.False(t, e.Foreground())
		g.SetForeground(true)
		assert.True(t, e.Foreground())
	})
	t.Run("services without it count as in front", func(t *testing.T) {
		t.Parallel()

		assert.True(t, env.New(noFg{f}, texts).Foreground())
	})
}

// TestDownloaderDoesNotAllocate is the gate of Env.Downloader: a screen asks for it on
// each event and each frame of a list.
func TestDownloaderDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	f := newFake(t)
	for name, svc := range map[string]env.Services{"manager": f, "none": nilDl{f}} {
		e := env.New(svc, locales.MustTexts())
		e.Downloader() // the first call makes the default
		assert.Zero(t, testing.AllocsPerRun(100, func() { _ = e.Downloader() }), name)
		assert.Zero(t, testing.AllocsPerRun(100, func() { _ = e.Foreground() }), name)
	}
}

// TestWiFiOnlyPreference checks the preference "Chỉ tải qua Wi‑Fi": on at first,
// kept by the services' Prefs when they have it, in memory otherwise.
func TestWiFiOnlyPreference(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	texts := locales.MustTexts()
	tests := []struct {
		name string
		svc  env.Services
		kept func() bool
	}{
		{name: "in the fake's prefs", svc: f, kept: f.FakePrefs().WiFiOnly},
		{name: "in memory", svc: noPrefs{f}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := env.New(tc.svc, texts)
			assert.True(t, e.WiFiOnly(), "the prototype's default")
			e.SetWiFiOnly(false)
			assert.False(t, e.WiFiOnly())
			if tc.kept != nil {
				assert.False(t, tc.kept(), "the services' prefs hold it")
			}
			e.SetWiFiOnly(true)
			assert.True(t, e.WiFiOnly())
		})
	}
}

// TestGroupAndHelpers checks the groups of screen 11 (first-seen order, sizes
// summed) and the two questions sheet 09 asks of a course.
func TestGroupAndHelpers(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	items := f.Dl().Items()
	other := env.DownloadItem{LessonID: "x1", CourseID: "set-34", CourseName: "DanceFIT Set 34", CourseLessons: 8, Bytes: 100}
	mixed := []env.DownloadItem{items[0], other, items[1]}

	t.Run("groups in first-seen order", func(t *testing.T) {
		t.Parallel()

		g := env.Group(mixed)
		require.Len(t, g, 2)
		assert.Equal(t, "set-35", g[0].CourseID)
		assert.Equal(t, 10, g[0].CourseLessons)
		assert.Len(t, g[0].Items, 2)
		assert.Equal(t, items[0].Bytes+items[1].Bytes, g[0].Bytes)
		assert.Equal(t, int64(100), g[1].Bytes)
		assert.Nil(t, env.Group(nil))
	})
	t.Run("the demo is one group of five, 48,4 MB", func(t *testing.T) {
		t.Parallel()

		g := env.Group(items)
		require.Len(t, g, 1)
		assert.Len(t, g[0].Items, 5)
		assert.Equal(t, int64(48_400_000), g[0].Bytes)
		assert.Equal(t, int64(48_400_000), f.Dl().TotalBytes())
	})

	e := env.New(f, locales.MustTexts())
	c := model.Course{ID: "set-35"}
	c.Lessons = allLessons()
	t.Run("lessons to download skip done, running and no audio", func(t *testing.T) {
		t.Parallel()

		c := c
		c.Lessons = append([]model.Lesson(nil), c.Lessons...)
		c.Lessons[9].HasAudio = false // 10 has no audio
		got, bytes := env.LessonsToDownload(c, e.Downloads())
		var ids []string
		for _, l := range got {
			ids = append(ids, l.ID)
		}
		assert.Equal(t, []string{"l_3505", "l_3508", "l_3509"}, ids)
		assert.Equal(t, int64(8_400_000+8_000_000+7_800_000), bytes)
	})
	t.Run("first missing", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "l_3505", env.FirstMissing(c, e.Downloads()))
		assert.Equal(t, "", env.FirstMissing(model.Course{}, e.Downloads()))
	})
}

// allLessons are the ten lessons of Set 35 as the mock has them.
func allLessons() []model.Lesson {
	sizes := []int64{12_800_000, 7_400_000, 8_800_000, 10_000_000, 8_400_000, 9_400_000, 9_800_000, 8_000_000, 7_800_000, 11_200_000}
	ids := []string{"l_3501", "l_3502", "l_3503", "l_3504", "l_3505", "l_3506", "l_3507", "l_3508", "l_3509", "l_3510"}
	out := make([]model.Lesson, len(ids))
	for i := range ids {
		out[i] = model.Lesson{ID: ids[i], No: i + 1, HasAudio: true, AudioSizeBytes: sizes[i]}
	}

	return out
}
