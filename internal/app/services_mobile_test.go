//go:build !tvos

package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAppVersionIsGdeskJSONs checks that the version the client sends is the
// one the build stamps.
func TestAppVersionIsGdeskJSONs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../gdesk.json")
	require.NoError(t, err)
	var cfg struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(raw, &cfg))
	assert.Equal(t, cfg.Version, appVersion)
}

// TestFaceIDUsageIsGdeskJSONs checks that the sentence iOS shows for Face
// ID (gdesk.json's ios.privacy.faceID, NSFaceIDUsageDescription) is the
// app's bio.faceIdUsage of locales/vi.json.
func TestFaceIDUsageIsGdeskJSONs(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../gdesk.json")
	require.NoError(t, err)
	var cfg struct {
		IOS struct {
			Privacy struct {
				FaceID string `json:"faceID"`
			} `json:"privacy"`
		} `json:"ios"`
	}
	require.NoError(t, json.Unmarshal(raw, &cfg))
	raw, err = os.ReadFile("../../locales/vi.json")
	require.NoError(t, err)
	var vi map[string]string
	require.NoError(t, json.Unmarshal(raw, &vi))
	require.NotEmpty(t, vi["bio.faceIdUsage"])
	assert.Equal(t, vi["bio.faceIdUsage"], cfg.IOS.Privacy.FaceID)
}

// TestWireSessionLifecycle walks a phone through two launches: no session,
// a sign-in that keeps the session and the email, then a second launch on
// the same vault that finds both.
func TestWireSessionLifecycle(t *testing.T) {
	t.Parallel()

	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	r := newRig(t, secrets, dir)
	w := r.wire(wireKnobs{})
	ctx := t.Context()

	signed, err := w.Restore(ctx)
	require.NoError(t, err)
	assert.False(t, signed)
	assert.Empty(t, w.LastEmail())
	cfg, err := w.Config(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Links.ForgotPassword)

	require.NoError(t, w.Login(ctx, "coach@example.invalid", "x"))
	assert.Equal(t, "coach@example.invalid", w.LastEmail())
	assert.True(t, secrets.has("test-install"), "the session is in the vault under the install id")
	assert.False(t, secrets.has("test-install.bio"), "no biometry: no copy")
	home, err := w.Home(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, home.MyCourses)
	course, err := w.Course(ctx, "set-35")
	require.NoError(t, err)
	assert.NotEmpty(t, course.Lessons)
	st, share := w.Downloads().State(course.Lessons[0].ID)
	assert.Equal(t, kit.DlNone, st)
	assert.Zero(t, share)
	assert.Zero(t, w.Downloads().DoneCount())

	again := newRig(t, secrets, dir).wire(wireKnobs{})
	signed, err = again.Restore(ctx)
	require.NoError(t, err)
	assert.True(t, signed)
	assert.Equal(t, "coach@example.invalid", again.LastEmail())
}

// TestWireLoginErrors checks that the server's refusals reach the screen as
// *api.Error with their code.
func TestWireLoginErrors(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"invalid_credentials", "account_locked", "device_limit"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			w := newRig(t, nil, "").wire(wireKnobs{})
			require.NoError(t, w.setup(t.Context()))
			require.NoError(t, w.mock.SetScenario("auth.login", code))
			err := w.Login(t.Context(), "a@b.c", "x")
			var ae *api.Error
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, code, ae.Code)
			assert.Empty(t, w.LastEmail(), "a refused sign-in remembers nothing")
		})
	}
}

// TestWireCoursesMergesPages checks that every page is read and that a cursor
// seen before ends the loop (the mock's paged scenario repeats c_6).
func TestWireCoursesMergesPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		atLeast  int
	}{
		{"default", 15},
		{"paged", 6},
		{"search_empty", 0},
	}
	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			w := newRig(t, nil, "").wire(wireKnobs{})
			ctx := t.Context()
			require.NoError(t, w.setup(ctx))
			require.NoError(t, w.Login(ctx, "a@b.c", "x"))
			require.NoError(t, w.mock.SetScenario("me.courses", tc.scenario))
			done := make(chan struct{})
			var n int
			go func() {
				defer close(done)
				p, err := w.Courses(ctx)
				assert.NoError(t, err)
				assert.Nil(t, p.Page.NextCursor)
				n = len(p.Items)
			}()
			select {
			case <-done:
			case <-time.After(driveLimit):
				t.Fatal("Courses does not end")
			}
			assert.GreaterOrEqual(t, n, tc.atLeast)
		})
	}
}

// TestWireBiometry checks K13 (the button shows with a biometry and a kept
// copy), a sign-in with the face after a logout, and a dismissed sheet.
func TestWireBiometry(t *testing.T) {
	t.Parallel()

	r := newRig(t, nil, "")
	r.bio.SetKind(sys.BiometryFace)
	w := r.wire(wireKnobs{})
	ctx := t.Context()
	assert.Equal(t, sys.BiometryNone, w.BioReady(), "not ready before the session is read")
	_, err := w.Restore(ctx)
	require.NoError(t, err)
	assert.Equal(t, sys.BiometryNone, w.BioReady(), "no copy before a sign-in")

	require.NoError(t, w.Login(ctx, "face@example.invalid", "x"))
	require.NoError(t, w.mgr.Logout(ctx))
	assert.Equal(t, sys.BiometryFace, w.BioReady(), "the logout keeps the copy")

	r.bio.Answer(sys.ErrBioCanceled)
	err = w.BioLogin(ctx, sys.BioPrompt{Reason: "r"})
	require.ErrorIs(t, err, context.Canceled)
	_, signed := w.mgr.Session()
	assert.False(t, signed)

	require.NoError(t, w.BioLogin(ctx, sys.BioPrompt{Reason: "r"}))
	_, signed = w.mgr.Session()
	assert.True(t, signed)
	assert.Equal(t, "r", r.bio.Prompt().Reason)

	r.bio.SetKind(sys.BiometryNone)
	assert.Equal(t, sys.BiometryNone, w.BioReady(), "no biometry enrolled: no button")
}

// TestWireForcedSignOut checks a session the server ends: the watchers hear
// it on the UI thread, the requests of the session are canceled, the
// biometric copy goes (Q-P3.2b-1).
func TestWireForcedSignOut(t *testing.T) {
	t.Parallel()

	r := newRig(t, nil, "")
	r.bio.SetKind(sys.BiometryFingerprint)
	w := r.wire(wireKnobs{})
	ctx := t.Context()
	_, err := w.Restore(ctx)
	require.NoError(t, err)
	require.NoError(t, w.Login(ctx, "a@b.c", "x"))
	require.Equal(t, sys.BiometryFingerprint, w.BioReady())
	r.q.drain() // what the setup posted (the preferences opened)
	var reasons []error
	stop := w.OnSignedOut(func(err error) { reasons = append(reasons, err) })
	defer stop()
	inFlight, done := w.scoped(ctx)
	defer done()

	w.mgr.SignOut(nil)
	select { // context.AfterFunc cancels on a goroutine of its own
	case <-inFlight.Done():
	case <-time.After(driveLimit):
		t.Fatal("the session's requests are not canceled")
	}
	assert.Empty(t, reasons, "told on the UI thread, not on the caller's")
	require.Equal(t, 3, r.q.drain(), "the downloads' reset, the playback's stop and the watcher's call")
	require.Len(t, reasons, 1)
	require.ErrorIs(t, reasons[0], auth.ErrSignedOut)
	assert.Equal(t, sys.BiometryNone, w.BioReady())

	next, done2 := w.scoped(ctx)
	defer done2()
	assert.NoError(t, next.Err(), "the next session's requests run")
}

// TestWireOnline checks the network going (the watchers told, the probe
// started) and coming back through the probe (03 §2).
func TestWireOnline(t *testing.T) {
	t.Parallel()

	r := newRig(t, nil, "")
	w := r.wire(wireKnobs{})
	ctx := t.Context()
	require.NoError(t, w.setup(ctx))
	w.probe.spawn = func(fn func()) { fn() }
	var seen []bool
	stop := w.WatchOnline(func(on bool) { seen = append(seen, on) })
	defer stop()

	w.tr.setOffline(true)
	_, err := w.Config(ctx)
	require.ErrorIs(t, err, api.ErrOffline)
	assert.False(t, w.Online())
	r.q.drain()
	assert.Equal(t, []bool{false}, seen)
	assert.Equal(t, 1, r.clk.Pending(), "a probe is armed")

	r.clk.Advance(probeSteps[0])
	assert.False(t, w.Online(), "the probe failed: still offline")
	assert.Equal(t, 1, r.clk.Pending(), "the next probe is armed")

	w.tr.setOffline(false)
	r.clk.Advance(probeSteps[1])
	assert.True(t, w.Online())
	r.q.drain()
	assert.Equal(t, []bool{false, true}, seen)
	assert.Zero(t, r.clk.Pending(), "the probe stopped")
}

// TestWireOpenURL checks that a URL goes to the system with the view's
// canvas, and is only logged without a window.
func TestWireOpenURL(t *testing.T) {
	t.Parallel()

	r := newRig(t, nil, "")
	w := r.wire(wireKnobs{})
	w.OpenURL("https://example.com/a")
	assert.Empty(t, r.opened())
	assert.NotEmpty(t, r.log.all())

	r.mount(kittest.Phones[0], "", false)
	r.w.OpenURL("https://example.com/b")
	calls := r.opened()
	require.Len(t, calls, 1)
	assert.Equal(t, "https://example.com/b", calls[0].raw)
	assert.NotNil(t, calls[0].canvas)
}

// TestWireAvatar loads the mock user's avatar from the mock's media server.
func TestWireAvatar(t *testing.T) {
	t.Parallel()

	w := newRig(t, nil, "").wire(wireKnobs{})
	ctx := t.Context()
	require.NoError(t, w.setup(ctx))
	require.NoError(t, w.Login(ctx, "a@b.c", "x"))
	s, _ := w.mgr.Session()
	require.NotEmpty(t, s.User.AvatarURL)
	img, err := w.Avatar(ctx, s.User.AvatarURL)
	require.NoError(t, err)
	assert.Positive(t, img.Width)

	_, err = w.Avatar(ctx, "http://127.0.0.1:1/none.jpg")
	require.Error(t, err)
}

// TestWireKnobs checks the clock and downloads a shot state pins.
func TestWireKnobs(t *testing.T) {
	t.Parallel()

	plain := newRig(t, nil, "").wire(wireKnobs{})
	assert.WithinDuration(t, time.Now(), plain.Now(), time.Minute)
	_, ok := plain.Downloads().(*wireDownloads)
	assert.True(t, ok)

	var k wireKnobs
	demo(&k)
	w := newRig(t, nil, "").wire(k)
	assert.Equal(t, demoNow, w.Now())
	ctx := t.Context()
	require.NoError(t, w.setup(ctx))
	require.NoError(t, w.Login(ctx, "a@b.c", "x"))
	require.NoError(t, k.demo.fill(ctx, w))
	c, err := w.Course(ctx, demoCourse)
	require.NoError(t, err)
	want := map[int]kit.LessonDownload{1: kit.DlDone, 5: kit.DlNone, 6: kit.DlDone, 7: kit.DlIng}
	for _, l := range c.Lessons {
		if st, ok := want[l.No]; ok {
			got, share := w.Downloads().State(l.ID)
			assert.Equal(t, st, got, "lesson %d", l.No)
			if st == kit.DlIng {
				assert.InDelta(t, demoIngShare, share, 1e-6)
			}
		}
	}
	assert.Equal(t, 5, w.Downloads().DoneCount())
}

// TestWireClosed checks that a closed wire makes nothing.
func TestWireClosed(t *testing.T) {
	t.Parallel()

	w := newRig(t, nil, "").wire(wireKnobs{})
	w.close()
	_, err := w.Restore(t.Context())
	require.ErrorIs(t, err, core.ErrClosed)
	w.close() // twice is fine
}

// TestWireSetupFailures checks that a part that cannot be made fails every
// blocking call with its error.
func TestWireSetupFailures(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	tests := []struct {
		name string
		edit func(d *mobileDeps)
		want error
	}{
		{"no data dir", func(d *mobileDeps) { d.DataDir = func() (string, error) { return "", boom } }, boom},
		{"no cache dir", func(d *mobileDeps) { d.CacheDir = func() (string, error) { return "", boom } }, boom},
		{"no secrets", func(d *mobileDeps) { d.Secrets = func() (auth.SecretStore, error) { return nil, boom } }, boom},
		{"no install id", func(d *mobileDeps) { d.InstallID = func() (string, error) { return "", boom } }, boom},
		{"bad platform", func(d *mobileDeps) { d.Platform = "web" }, core.ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, nil, "")
			tc.edit(&r.deps)
			w := r.wire(wireKnobs{})
			_, err := w.Restore(t.Context())
			require.ErrorIs(t, err, tc.want)
			require.ErrorIs(t, w.Login(t.Context(), "a", "b"), tc.want)
			_, err = w.Home(t.Context())
			require.ErrorIs(t, err, tc.want)
			assert.Equal(t, sys.BiometryNone, w.BioReady())
		})
	}
}
