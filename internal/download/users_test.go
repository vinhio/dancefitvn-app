package download_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
)

func TestTwoUsersShareOneFolder(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	a := r.mgr // u1
	require.NoError(t, a.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	ea, _ := a.Lookup("l_3503")

	r.fake.Advance(time.Hour)
	b := r.open(t, r.dir, "u2")
	_, ok := b.Lookup("l_3503")
	require.False(t, ok, "u2 does not see u1's file")
	require.NoError(t, b.Start(lesson("l_3503"), course("set-35")), "the same lesson for the second user")
	r.ev.wait(t, download.EventFinished, "l_3503")

	eb, ok := b.Lookup("l_3503")
	require.True(t, ok)
	require.Equal(t, "u2", eb.UserID)
	ea2, ok := a.Lookup("l_3503")
	require.True(t, ok, "finish of u2 keeps u1's entry")
	require.Equal(t, ea, ea2)
	require.Len(t, a.List(), 1)
	require.Len(t, b.List(), 1)

	// u1 deletes: u2 keeps the file.
	require.NoError(t, a.Delete("l_3503"))
	require.Empty(t, a.List())
	_, ok = a.Path("l_3503")
	require.False(t, ok)
	p, ok := b.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, r.body, readAll(t, p))

	// u2 deletes: nobody holds it, the file goes.
	require.NoError(t, b.Delete("l_3503"))
	require.NoFileExists(t, p)
	require.Empty(t, b.List())

	// And it survives a reopen the same way.
	r2 := r.open(t, r.dir, "u1")
	require.Empty(t, r2.List())
}

func TestDeleteByOneUserLeavesTheOthersEntry(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	b := r.open(t, r.dir, "u2")
	require.NoError(t, b.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	r.mgr.Close()
	b.Close()

	// Reopened, u2 deletes first.
	b2 := r.open(t, r.dir, "u2")
	require.NoError(t, b2.Delete("l_3503"))
	a2 := r.open(t, r.dir, "u1")
	p, ok := a2.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, r.body, readAll(t, p))
}

func TestNewVersionDropsTheOtherUsersStaleEntry(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")

	// The server's file changed: the next download replaces the shared file.
	body2, sum2 := content(60_000)
	r.srv.body = body2
	r.audio = func(_ int, id string) model.Audio {
		return model.Audio{LessonID: id, URL: r.srv.URL + "/" + id + ".mp3", SizeBytes: int64(len(body2)), SHA256: sum2, ExpiresAt: t0.Add(time.Hour)}
	}
	b := r.open(t, r.dir, "u2")
	require.NoError(t, b.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")

	_, ok := r.mgr.Lookup("l_3503")
	require.False(t, ok, "u1's entry describes a file that is gone")
	_, ok = r.mgr.Path("l_3503")
	require.False(t, ok)
	p, ok := b.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, body2, readAll(t, p))
}

func TestLegacyIndexOfOneUser(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	dir := t.TempDir()
	// An index and file written before the entries were keyed by user.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "l_3503.mp3"), r.body, 0o600))
	doc := map[string]any{"items": []map[string]any{{
		"lesson_id": "l_3503", "course_id": "set-35", "user_id": "u1", "size": len(r.body), "sha256": r.sum,
		"downloaded_at": t0, "no": 3, "title": "Bài 3",
	}}}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.json"), raw, 0o600))

	a := r.open(t, dir, "u1")
	e, ok := a.Lookup("l_3503")
	require.True(t, ok)
	require.Equal(t, "Bài 3", e.Title)
	p, ok := a.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, filepath.Join(dir, "l_3503.mp3"), p)

	b := r.open(t, dir, "u2")
	require.Empty(t, b.List())
	require.NoError(t, b.Delete("l_3503"), "nothing of u2's to delete")
	require.FileExists(t, filepath.Join(dir, "l_3503.mp3"))
	_, ok = a.Path("l_3503")
	require.True(t, ok)
}

func TestCancelKeepsThePartOfAnotherUsersPending(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	cutOnce(r.srv)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventPaused, "l_3503")
	r.mgr.PauseAll(nil)
	r.mgr.Close()
	part := filepath.Join(r.dir, "l_3503.mp3.part")
	require.FileExists(t, part)

	b := r.open(t, r.dir, "u2")
	require.NoError(t, b.Start(lesson("l_3503"), course("set-35")))
	b.Cancel("l_3503")
	require.FileExists(t, part, "u1 still has a pending download using it")
}

func TestValidatorPersistsAcrossRestart(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	cutOnce(r.srv)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventPaused, "l_3503")
	have := partSize(r.dir, "l_3503")
	require.Positive(t, have)
	r.mgr.Close()

	raw, err := os.ReadFile(filepath.Join(r.dir, "index.json"))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"validator":"\"v1\""`)

	m2 := r.open(t, r.dir, "u1")
	m2.ResumeAll()
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.Equal(t, []string{"", "bytes=" + strconv.FormatInt(have, 10) + "-"}, r.srv.requests())
	require.Equal(t, []string{"", `"v1"`}, r.srv.ifRanges(), "If-Range survives the restart")
	p, ok := m2.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, r.body, readAll(t, p))
}

func TestErrorsWrapCoreSentinels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		core error
	}{
		{"checksum", download.ErrChecksum, core.ErrInvalid},
		{"no space", download.ErrNoSpace, core.ErrInvalid},
		{"wifi only", download.ErrWiFiOnly, core.ErrUnsupported},
	}
	all := []error{download.ErrChecksum, download.ErrNoSpace, download.ErrWiFiOnly}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tc.err, tc.core)
			require.ErrorIs(t, tc.err, tc.err)
			for _, other := range all {
				if other != tc.err {
					require.False(t, errors.Is(tc.err, other), "identity stays distinct")
				}
			}
		})
	}
}
