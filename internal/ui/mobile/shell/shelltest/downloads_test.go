//go:build !tvos

package shelltest_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errRefused is the refusal the knob SetStartErr makes.
var errRefused = errors.New("403")

// course reads Set 35 from the mock's folder.
func course(t *testing.T) model.Course {
	t.Helper()
	raw, err := fs.ReadFile(shelltest.MockFS(t), "courses/set-35.json")
	require.NoError(t, err)
	var c model.Course
	require.NoError(t, json.Unmarshal(raw, &c))

	return c
}

// newFake returns a Fake with a session and the demo downloads.
func newFake(t *testing.T) *shelltest.Fake {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)

	return f
}

// record collects the events of the downloads.
func record(f *shelltest.Fake) *[]env.DownloadEvent {
	var got []env.DownloadEvent
	f.Dl().Events().Subscribe(func(ev env.DownloadEvent) { got = append(got, ev) })

	return &got
}

// TestFakeDownloadsDemo checks the state the fake starts in: the prototype's
// (`S.dl`, M11): five files of 48,4 MB, lesson 7 at 64 %, 18,2 GB free of a disk
// 71 % full.
func TestFakeDownloadsDemo(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	d := f.Dl()
	assert.Same(t, d, f.Downloads())
	assert.Equal(t, 5, d.DoneCount())
	assert.Equal(t, int64(48_400_000), d.TotalBytes())
	var nos []int
	for _, it := range d.Items() {
		nos = append(nos, it.No)
		assert.Equal(t, kit.DlDone, it.State)
		assert.Equal(t, "DanceFIT Set 35", it.CourseName)
		assert.Equal(t, 10, it.CourseLessons)
	}
	assert.Equal(t, []int{1, 2, 3, 4, 6}, nos, "oldest first")
	jobs := d.Jobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, 7, jobs[0].No)
	assert.InDelta(t, .64, jobs[0].Progress, 1e-6)
	assert.Equal(t, int64(9_800_000), jobs[0].Bytes)
	st, p := d.State("l_3507")
	assert.Equal(t, kit.DlIng, st)
	assert.InDelta(t, .64, p, 1e-6)
	free, total, ok := d.FreeSpace()
	require.True(t, ok)
	assert.Equal(t, int64(18_200_000_000), free)
	assert.InDelta(t, .71, 1-float64(free)/float64(total), 1e-4)

	d.Clear()
	assert.Zero(t, d.DoneCount())
	assert.Empty(t, d.Jobs())
}

// TestFakeDownloadsStart checks Start as the manager answers it: what it queues,
// and the errors that queue nothing or queue paused.
func TestFakeDownloadsStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(f *shelltest.Fake)
		wantErr error
		started []string
		paused  bool
	}{
		{name: "skips done and running", started: []string{"l_3505", "l_3508"}},
		{name: "offline", setup: func(f *shelltest.Fake) { f.SetOnline(false) }, wantErr: api.ErrOffline},
		{name: "no space", setup: func(f *shelltest.Fake) { f.Dl().SetSpace(1, 10) }, wantErr: env.ErrNoSpace},
		{name: "wifi only on mobile data", setup: func(f *shelltest.Fake) { f.Dl().SetOnWiFi(false) },
			wantErr: env.ErrWiFiOnly, started: []string{"l_3505", "l_3508"}, paused: true},
		{name: "mobile data, preference off", setup: func(f *shelltest.Fake) {
			f.Dl().SetOnWiFi(false)
			f.FakePrefs().SetWiFiOnly(false)
		}, started: []string{"l_3505", "l_3508"}},
		{name: "the knob", setup: func(f *shelltest.Fake) { f.Dl().SetStartErr(errRefused) }, wantErr: errRefused},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t)
			c := course(t)
			if tc.setup != nil {
				tc.setup(f)
			}
			got := record(f)
			lessons := []model.Lesson{c.Lessons[0], c.Lessons[4], c.Lessons[6], c.Lessons[7]} // 1 done, 5, 7 running, 8
			err := f.Dl().Start(c, lessons)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, append([]string(nil), tc.started...), append([]string(nil), f.Dl().Started()...))
			for _, id := range tc.started {
				st, _ := f.Dl().State(id)
				assert.Equal(t, kit.DlIng, st, id)
			}
			for _, j := range f.Dl().Jobs() {
				if j.LessonID == "l_3507" {
					continue
				}
				assert.Equal(t, tc.paused, j.Paused, j.LessonID)
			}
			if len(tc.started) > 0 {
				require.NotEmpty(t, *got)
				assert.Equal(t, env.DownloadEventChanged, (*got)[0].Kind)
			}
		})
	}
}

// TestFakeDownloadsKnobs checks the knobs and the events they publish: progress,
// finish, fail, pause, forget, delete, the space.
func TestFakeDownloadsKnobs(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	d := f.Dl()
	got := record(f)

	d.SetDownload("l_3507", kit.DlIng, .8)
	d.FinishDownload("l_3507")
	d.SetDownload("l_3509", kit.DlIng, 0)
	d.FailDownload("l_3509", env.ErrChecksum)
	d.PauseDownload("l_3509", api.ErrOffline)
	d.Forget("l_3509")
	require.NoError(t, d.Delete("l_3501"))
	d.SetSpace(5, 10)
	d.ClearSpace()
	d.RefreshSpace()

	kinds := make([]env.DownloadEventKind, len(*got))
	for i, ev := range *got {
		kinds[i] = ev.Kind
	}
	assert.Equal(t, []env.DownloadEventKind{
		env.DownloadEventProgress, env.DownloadEventFinished,
		env.DownloadEventChanged, env.DownloadEventFailed, env.DownloadEventPaused, env.DownloadEventChanged,
		env.DownloadEventChanged, env.DownloadEventSpace, env.DownloadEventSpace, env.DownloadEventSpace,
	}, kinds)
	assert.Equal(t, 7, (*got)[1].No, "the finish carries the number")
	assert.ErrorIs(t, (*got)[3].Err, env.ErrChecksum)
	assert.Equal(t, []string{"l_3509"}, d.Cancelled())
	assert.Equal(t, []string{"l_3501"}, d.Deleted())
	assert.Equal(t, 5, d.DoneCount(), "1 deleted, 7 finished")
	_, _, ok := d.FreeSpace()
	assert.False(t, ok)
	_, _, refreshes := d.Calls()
	assert.Equal(t, 1, refreshes)

	assert.Panics(t, func() { d.FinishDownload("l_3501") }, "no download of it")
	assert.Panics(t, func() { d.SetDownload("nope", kit.DlDone, 1) }, "unknown lesson")

	d.SetDeleteErr(errors.New("disk"))
	require.Error(t, d.Delete("l_3502"))
	assert.Equal(t, 5, d.DoneCount())
}

// TestFakeDownloadsWiFiRule checks that mobile data pauses what runs when the
// preference is on, and Wi-Fi goes on with it, and that the preference off does
// not pause.
func TestFakeDownloadsWiFiRule(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	d := f.Dl()
	d.SetOnWiFi(false)
	jobs := d.Jobs()
	require.Len(t, jobs, 1)
	assert.True(t, jobs[0].Paused)
	assert.ErrorIs(t, jobs[0].Err, env.ErrWiFiOnly)
	d.SetOnWiFi(true)
	assert.False(t, d.Jobs()[0].Paused)

	f.FakePrefs().SetWiFiOnly(false)
	d.SetOnWiFi(false)
	assert.False(t, d.Jobs()[0].Paused, "the preference is off")

	d.Pause()
	assert.True(t, d.Jobs()[0].Paused)
	d.Resume()
	assert.False(t, d.Jobs()[0].Paused)
	pauses, resumes, _ := d.Calls()
	assert.Equal(t, 1, pauses)
	assert.Equal(t, 1, resumes)
}
