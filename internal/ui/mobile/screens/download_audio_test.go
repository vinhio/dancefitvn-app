//go:build !tvos

package screens_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"testing"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toastNoAudio is toast.audioMissing, the new text of a lesson with no audio file.
const toastNoAudio = "Bài này chưa có file âm thanh"

// patchedCourse is the mock folder with the lessons of courses/set-35.json edited by
// edit (the lesson objects as the API sends them: has_audio, audio_size_bytes, null
// where the server has no value).
func patchedCourse(t testing.TB, edit func(lessons []any)) fs.FS {
	t.Helper()
	base := shelltest.MockFS(t)
	data, err := fs.ReadFile(base, "courses/set-35.json")
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	lessons, ok := doc["lessons"].([]any)
	require.True(t, ok)
	edit(lessons)
	out, err := json.Marshal(doc)
	require.NoError(t, err)

	return overlayFS{FS: base, files: map[string]string{"courses/set-35.json": string(out)}}
}

// detailOver mounts the shell on the patched mock folder and opens the detail of Set 35.
func detailOver(t *testing.T, ph kittest.Phone, fsys fs.FS) *shelltest.Rig {
	t.Helper()
	f, err := shelltest.New(fsys)
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	r := shelltest.MountRig(t, ph, false, f, shell.Options{})
	r.Boot()
	require.Equal(t, "home", r.Route())
	r.Shell.Detail("set-35")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(t, "detail", r.Route())

	return r
}

// lessonAt returns the lesson object of number no of the course's data.
func lessonAt(lessons []any, no int) map[string]any {
	l, _ := lessons[no-1].(map[string]any)

	return l
}

// TestDownloadSheetUnknownSize checks the sheet for a lesson whose audio facts the
// API leaves null (audio_size_bytes null, so 0 in the model): "—" where the size
// would be, never "0,0 MB", and the button says "Tải xuống" alone. A size that is
// known is as before.
func TestDownloadSheetUnknownSize(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		fsys := patchedCourse(t, func(lessons []any) {
			lessonAt(lessons, 5)["audio_size_bytes"] = nil
		})
		r := detailOver(t, ph, fsys)
		r.Shell.DlTap("set-35", "l_3505", 5)
		r.Settle()
		slideIn(r)
		require.True(t, r.Shell.Nav().Sheet().IsOpen())
		s := downloadSheetOf(t, r)
		assert.Equal(t, "—", radio(t, r, "dlChoice:one").Trailing)
		assert.Equal(t, "Tải xuống", confirmText(t, r))
		assert.NotContains(t, r.Tester.Shown(), "0,0 MB")

		// "Cả khoá": lessons 5, 8, 9 and 10 are left; 5 has no size, the others do
		tap(r, named(r.Shell, "dlChoice:all", 1))
		r.Settle()
		n, bytes := s.Missing()
		assert.Equal(t, 4, n)
		assert.Equal(t, "Tải xuống · "+format.Size(bytes), confirmText(t, r), "the sum of the sizes that are known")
		assert.NotEqual(t, "—", radio(t, r, "dlChoice:all").Trailing)
	})
}

// TestDownloadSheetNoSizeAtAll checks "Cả khoá" when no lesson has a size: "—" and the
// button without one.
func TestDownloadSheetNoSizeAtAll(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		fsys := patchedCourse(t, func(lessons []any) {
			for _, l := range lessons {
				m, _ := l.(map[string]any)
				m["audio_size_bytes"] = nil
			}
		})
		r := detailOver(t, ph, fsys)
		r.Shell.DlAll("set-35")
		r.Settle()
		slideIn(r)
		assert.Equal(t, "—", radio(t, r, "dlChoice:all").Trailing)
		assert.Equal(t, "—", radio(t, r, "dlChoice:one").Trailing)
		assert.Equal(t, "Tải xuống", confirmText(t, r))
	})
}

// TestLessonWithoutAudioHasNoDownload checks has_audio false (or null): the row of
// the lesson has no download button, its room stays so that nothing moves, a tap on
// where it was does nothing, and "Cả khoá" counts the other lessons only.
func TestLessonWithoutAudioHasNoDownload(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		fsys := patchedCourse(t, func(lessons []any) {
			lessonAt(lessons, 5)["has_audio"] = false
			lessonAt(lessons, 8)["has_audio"] = nil
		})
		r := detailOver(t, ph, fsys)
		assert.False(t, has(r, "dlTap:set35-5"), "no download button for lesson 5")
		assert.False(t, has(r, "dlTap:set35-8"), "nor for lesson 8, whose has_audio is null")
		other := named(r.Shell, "dlTap:set35-9", 1)
		require.NotNil(t, other, "lesson 9 has one")
		row := named(r.Shell, "playVideo:set35-5", 1)
		require.NotNil(t, row)

		r.Shell.DlTap("set-35", "l_3505", 5) // the video screen's side button, a stale tap
		r.Settle()
		assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "no sheet to fail late in")
		assert.True(t, shows(r, toastNoAudio))

		r.Shell.DlAll("set-35")
		r.Settle()
		slideIn(r)
		s := downloadSheetOf(t, r)
		n, _ := s.Missing()
		assert.Equal(t, 2, n, "5, 8, 9 and 10 have no file; 5 and 8 have no audio either, so 9 and 10 are left")
		assert.Equal(t, "l_3509", s.Params.LessonID, "the sheet opens on the first lesson that has an audio")
	})
}

// TestAudioNotFoundIsNotARetry checks a lesson whose audio the server answers
// not_found for after the download started (has_audio true, lessons.audio 404): the
// toast says the audio is not available instead of "Tải lỗi, thử lại", the lesson's
// download button goes for the rest of the run, and a tap on a stale one does not open
// the sheet again.
func TestAudioNotFoundIsNotARetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		tap(r, named(r.Shell, "dlConfirm", 1))
		r.Settle()
		require.Equal(t, []string{"l_3505"}, r.Fake.Dl().Started())
		require.True(t, has(r, "dlTap:set35-5"))

		r.Fake.Dl().FailDownload("l_3505", fmt.Errorf("download: %w", env.ErrNoAudio))
		r.Settle()
		assert.True(t, shows(r, toastNoAudio))
		assert.False(t, r.Tester.Shows("Tải lỗi, thử lại"))
		st, _ := r.Fake.Dl().State("l_3505")
		assert.Equal(t, kit.DlUnavailable, st)
		assert.False(t, has(r, "dlTap:set35-5"), "no button to retry with")
		assert.True(t, has(r, "dlTap:set35-6"), "the other lessons keep theirs")

		r.Shell.DlTap("set-35", "l_3505", 5)
		r.Settle()
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
	})
}

// TestOtherDownloadFailuresStillRetry checks that the toast of the audio missing is
// for that failure only: a checksum or a server error still says "Tải lỗi, thử lại"
// and leaves the button.
func TestOtherDownloadFailuresStillRetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		tap(r, named(r.Shell, "dlConfirm", 1))
		r.Settle()
		r.Fake.Dl().FailDownload("l_3505", env.ErrChecksum)
		r.Settle()
		assert.True(t, shows(r, "Tải lỗi, thử lại"))
		assert.False(t, r.Tester.Shows(toastNoAudio))
		st, _ := r.Fake.Dl().State("l_3505")
		assert.Equal(t, kit.DlNone, st)
		assert.True(t, has(r, "dlTap:set35-5"))
	})
}

var _ = screens.NewDownloadSheet
