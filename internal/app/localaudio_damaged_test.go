//go:build !tvos

package app

import (
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocalAudioVerdictAndFingerprint checks K49's rule: a file handed over twice with
// no play in between is judged damaged, a play in between clears the suspicion, a
// file that is another one (another fingerprint) is judged anew, forget drops the
// verdict, and the verdicts survive a load.
func TestLocalAudioVerdictAndFingerprint(t *testing.T) {
	t.Parallel()

	fp := "a"
	var told []string
	l := &localAudio{print: func(string) string { return fp }, changed: func(id string) { told = append(told, id) }}
	assert.True(t, l.use("x"), "first hand-over")
	l.played("x")
	assert.True(t, l.use("x"), "it played: sound")
	assert.False(t, l.damaged("x"))
	assert.False(t, l.use("x"), "twice with no play")
	assert.True(t, l.damaged("x"))
	assert.Equal(t, []string{"x"}, told, "the wire is told once")
	assert.False(t, l.use("x"), "streams from now on")
	assert.Equal(t, []string{"x"}, told)

	fp = "b" // downloaded again under the same lesson
	assert.False(t, l.damaged("x"), "another file: no verdict on it")
	assert.True(t, l.use("x"), "judged anew")

	l.forget("x")
	assert.False(t, l.damaged("x"))
	assert.True(t, l.use("x"))

	var other localAudio
	other.print = func(string) string { return "b" }
	other.load([]byte(`{"bad":{"y":"b","z":""}}`))
	assert.True(t, other.damaged("y"), "the verdict of a run before")
	assert.False(t, other.damaged("z"), "an empty fingerprint is no verdict")
	other.load([]byte(`not json`))
	assert.True(t, other.damaged("y"))
}

// TestDamagedFileIsMarkedAndKept checks K49 end to end: a file handed to the player
// twice that never loaded is marked (State stays DlDone, Items carry Damaged, a
// DownloadEventChanged tells the screens), the file stays on the disk, a delete clears
// the verdict and a file downloaded again is not marked.
func TestDamagedFileIsMarkedAndKept(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newDlRig(t, dir, wireKnobs{})
	c := d.signIn()
	l := lessonOf(t, c, 5)
	dm := d.manager()
	require.NoError(t, dm.Start(c, []model.Lesson{l}))
	d.drive("the file", func() bool { st, _ := dm.State(l.ID); return st == kit.DlDone })
	file := filepath.Join(dir, "nobackup", "audio", l.ID+".mp3")
	assert.False(t, env.Damaged(dm, l.ID))

	res := wireResolver{w: d.w}
	u, err := res.AudioURL(t.Context(), l.ID)
	require.NoError(t, err)
	assert.Equal(t, file, u, "the file, once")
	before := len(d.events)
	u, err = res.AudioURL(t.Context(), l.ID) // handed over again: it never loaded
	require.NoError(t, err)
	assert.NotEqual(t, file, u, "the API's address")
	d.drive("the mark", func() bool { return env.Damaged(dm, l.ID) })
	st, _ := dm.State(l.ID)
	assert.Equal(t, kit.DlDone, st, "still a downloaded file")
	require.Len(t, dm.Items(), 1)
	assert.True(t, dm.Items()[0].Damaged)
	assert.FileExists(t, file, "not deleted")
	assert.Contains(t, d.kinds()[before:], env.DownloadEventChanged, "the screens are told")

	require.NoError(t, dm.Delete(l.ID))
	assert.False(t, env.Damaged(dm, l.ID), "the delete clears the verdict")
	require.NoError(t, dm.Start(c, []model.Lesson{l}))
	d.drive("the file again", func() bool { st, _ := dm.State(l.ID); return st == kit.DlDone })
	assert.False(t, env.Damaged(dm, l.ID), "downloaded again: judged anew")
	assert.False(t, dm.Items()[0].Damaged)
	u, err = res.AudioURL(t.Context(), l.ID)
	require.NoError(t, err)
	assert.Equal(t, file, u, "the new file is handed over")
}

// TestDamagedFileSurvivesARestart checks the verdict is kept in the prefs folder with
// the fingerprint of the file, so that the next run marks the file at once (a file
// with another fingerprint is not marked: TestLocalAudioVerdictAndFingerprint).
func TestDamagedFileSurvivesARestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secrets := &memSecrets{vals: map[string][]byte{}}
	first := newDlRigOn(t, secrets, dir, wireKnobs{})
	c := first.signIn()
	l := lessonOf(t, c, 5)
	require.NoError(t, first.manager().Start(c, []model.Lesson{l}))
	first.drive("the file", func() bool { st, _ := first.manager().State(l.ID); return st == kit.DlDone })
	res := wireResolver{w: first.w}
	_, err := res.AudioURL(t.Context(), l.ID)
	require.NoError(t, err)
	_, err = res.AudioURL(t.Context(), l.ID)
	require.NoError(t, err)
	first.drive("the mark", func() bool { return env.Damaged(first.manager(), l.ID) })
	first.w.close() // the writes in flight end

	again := newDlRigOn(t, secrets, dir, wireKnobs{})
	signed, err := again.w.Restore(t.Context())
	require.NoError(t, err)
	require.True(t, signed)
	again.drive("the file listed", func() bool { return again.manager().DoneCount() == 1 })
	assert.True(t, env.Damaged(again.manager(), l.ID), "marked from the first moment")
	assert.True(t, again.manager().Items()[0].Damaged)
	again.w.close()
}

// TestEntryPrintTellsFiles checks the fingerprint changes with the size, the sha256
// and the time of the download.
func TestEntryPrintTellsFiles(t *testing.T) {
	t.Parallel()

	a := download.Entry{Size: 10, SHA256: "ab"}
	b := a
	b.Size = 11
	c := a
	c.SHA256 = "cd"
	e := a
	e.DownloadedAt = e.DownloadedAt.Add(1)
	for _, o := range []download.Entry{b, c, e} {
		assert.NotEqual(t, entryPrint(a), entryPrint(o))
	}
	assert.Equal(t, entryPrint(a), entryPrint(a))
}
