//go:build !tvos

package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// fakePicker is the system's picker of a test: it answers what it is told,
// through the dispatch, as sys.PickImage does.
type fakePicker struct {
	mu     sync.Mutex
	calls  int
	canvas platform.Canvas
	start  error  // returned by the call: the picker did not show
	data   []byte // the picture answered
	err    error  // the error answered (sys.ErrCanceled)
	hold   chan struct{}
}

// pickAnswer is what a fakePicker answers.
type pickAnswer struct {
	start error
	data  []byte
	err   error
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestWireAccountReadsAndWrites checks every route of the account through the
// wire over the mock and its overlay (05 section 4): what a write changed is
// what the next read answers (the overlay), a device deleted is gone from the
// list until a TV code brings it back, a code that is not one sends nothing,
// and the session's user is the one the vault holds.
func TestWireAccountReadsAndWrites(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	_, ok := pr.w.SessionUser()
	assert.False(t, ok, "nobody before the parts are made")
	pr.signIn()
	ctx := t.Context()
	w := pr.w

	u, ok := w.SessionUser()
	require.True(t, ok)
	assert.Equal(t, "u_412", u.ID)
	assert.NotEmpty(t, u.Email)

	me, err := w.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "u_412", me.User.ID)

	up, err := w.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "Lan", LastName: "Trần", Email: me.User.Email, Phone: "0912345678"})
	require.NoError(t, err)
	assert.Equal(t, "Lan", up.User.FirstName, "the answer is the user as the overlay keeps it")
	assert.Equal(t, "Trần Lan", up.User.FullName)
	me, err = w.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Lan", me.User.FirstName, "kept for the next read")
	assert.Equal(t, "0912345678", me.User.Phone)

	pw, err := w.ChangePassword(ctx, "secret1", "secret22")
	require.NoError(t, err)
	assert.True(t, pw.OK)
	_, signed := w.mgr.Session()
	assert.True(t, signed, "the session is kept (Q-P6-23)")
	assert.Equal(t, 1, pr.st.calledN("me.password"))

	c, err := w.Coach(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, c.AllowedPlatforms)
	c2, err := w.UpdateCoach(ctx, model.CoachUpdateRequest{Bio: "Mới", Places: []string{"CLB A"},
		Socials: []model.SocialURL{{URL: "https://www.tiktok.com/@x"}}})
	require.NoError(t, err)
	assert.Equal(t, "Mới", c2.Bio)
	assert.Equal(t, []string{"CLB A"}, c2.Places)
	require.Len(t, c2.Socials, 1)
	assert.Equal(t, "tiktok", c2.Socials[0].Platform)

	l, err := w.Devices(ctx)
	require.NoError(t, err)
	require.True(t, hasDevice(l, "d_201"), "the mock's TV")
	require.NoError(t, w.DeleteDevice(ctx, "d_201"))
	l, err = w.Devices(ctx)
	require.NoError(t, err)
	assert.False(t, hasDevice(l, "d_201"), "deleted through the overlay")

	_, err = w.PairConfirm(ctx, "abc")
	require.ErrorIs(t, err, core.ErrInvalid)
	assert.Zero(t, pr.st.calledN("tv.pair.confirm"), "nothing sent for what is not a code")
	pc, err := w.PairConfirm(ctx, "7k4-m2p")
	require.NoError(t, err)
	assert.Equal(t, "d_201", pc.Device.ID)
	bodies := pr.st.bodiesOf("tv.pair.confirm")
	require.Len(t, bodies, 1)
	assert.Contains(t, string(bodies[0]), `"7K4M2P"`, "normalised")
	l, err = w.Devices(ctx)
	require.NoError(t, err)
	assert.True(t, hasDevice(l, "d_201"), "the TV paired again is listed")
}

// TestWireAccountErrors checks that the account's calls give the mock's
// refusals as *api.Error with their code, and a dead network as offline.
func TestWireAccountErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		route, scenario, code string
		call                  func(ctx context.Context, w *mobileWire) error
	}{
		{"me.update", "validation_failed", "validation_failed", func(ctx context.Context, w *mobileWire) error {
			_, err := w.UpdateMe(ctx, model.MeUpdateRequest{})

			return err
		}},
		{"me.password", "wrong_password", "wrong_password", func(ctx context.Context, w *mobileWire) error {
			_, err := w.ChangePassword(ctx, "a", "bbbbbbbb")

			return err
		}},
		{"tv.pair.confirm", "invalid_code", "pair_code_invalid", func(ctx context.Context, w *mobileWire) error {
			_, err := w.PairConfirm(ctx, "7K4M2P")

			return err
		}},
		{"tv.pair.confirm", "device_limit", "device_limit", func(ctx context.Context, w *mobileWire) error {
			_, err := w.PairConfirm(ctx, "7K4M2P")

			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.route+"/"+tc.scenario, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			pr.signIn()
			require.NoError(t, pr.w.mock.SetScenario(tc.route, tc.scenario))
			err := tc.call(t.Context(), pr.w)
			var ae *api.Error
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, tc.code, ae.Code)
		})
	}
	t.Run("offline", func(t *testing.T) {
		t.Parallel()

		pr := newPlayRig(t, "", nil)
		pr.signIn()
		pr.w.tr.setOffline(true)
		_, err := pr.w.UpdateMe(t.Context(), model.MeUpdateRequest{FirstName: "x", Email: "a@b"})
		require.ErrorIs(t, err, api.ErrOffline)
		assert.Equal(t, env.FailureOffline, env.FailureOf(err).Kind)
	})
}

// TestWireUploadAvatarPrimesTheCache checks PUT /me/avatar: the picture is sent
// as the multipart field "file", the URL answered becomes the user's (the
// overlay), and the image cache holds the picture sent under it, so that the
// screens draw it without reading it back.
func TestWireUploadAvatarPrimesTheCache(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	ctx := t.Context()
	pic := solidJPEG(t, 512, 512, color.RGBA{R: 200, G: 40, B: 90, A: 255})
	a, err := pr.w.UploadAvatar(ctx, pic)
	require.NoError(t, err)
	require.NotEmpty(t, a.AvatarURL)
	img, ok := pr.w.images.Image(a.AvatarURL)
	require.True(t, ok, "primed")
	assert.Equal(t, 512, img.Width)
	bodies := pr.st.bodiesOf("me.avatar")
	require.Len(t, bodies, 1)
	assert.True(t, bytes.Contains(bodies[0], []byte(`name="file"`)))
	assert.True(t, bytes.Contains(bodies[0], pic))
	me, err := pr.w.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, a.AvatarURL, me.User.AvatarURL)
	got, err := pr.w.Avatar(ctx, a.AvatarURL)
	require.NoError(t, err)
	assert.Same(t, img, got, "no read of the URL")

	pr.w.tr.setOffline(true)
	_, err = pr.w.UploadAvatar(ctx, pic)
	require.ErrorIs(t, err, api.ErrOffline)
}

// TestWireAvatarReadsBadges checks that Avatar draws the badges of GET /me
// (mock-media://badges/..., the mock's media server once rewritten, through
// the overlay's edges) and the user's avatar, and that the app's version is
// gdesk.json's (screens.Versioned).
func TestWireAvatarReadsBadges(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	ctx := t.Context()
	me, err := pr.w.Me(ctx)
	require.NoError(t, err)
	require.Len(t, me.Badges, 3)
	for _, b := range me.Badges {
		require.True(t, strings.HasPrefix(b.ImageURL, "http://"), "rewritten: %s", b.ImageURL)
		assert.NotContains(t, b.ImageURL, mockMediaScheme)
		img, err := pr.w.Avatar(ctx, b.ImageURL)
		require.NoError(t, err, b.ImageURL)
		assert.Positive(t, img.Width)
	}
	img, err := pr.w.Avatar(ctx, me.User.AvatarURL)
	require.NoError(t, err)
	assert.Positive(t, img.Width)
	assert.Equal(t, appVersion, pr.w.AppVersion())
}

// TestWireOverlayKeepsMediaURLsPlain checks that the overlay records the
// media URLs as the mock's files write them (plainMedia): an avatar uploaded
// in one launch still loads in the next, whose media server has another port.
func TestWireOverlayKeepsMediaURLsPlain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r := newRig(t, secrets, dir)
	w := r.wire(wireKnobs{})
	ctx := t.Context()
	_, err := w.Restore(ctx)
	require.NoError(t, err)
	require.NoError(t, w.Login(ctx, "coach@example.invalid", "secret1"))
	a, err := w.UploadAvatar(ctx, solidJPEG(t, 512, 512, color.RGBA{G: 200, A: 255}))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(a.AvatarURL, w.mock.Media().BaseURL()+"/"))
	raw, err := os.ReadFile(filepath.Join(dir, "data", store.OverlayKey+".json"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), mockMediaScheme+"avatars/")
	assert.NotContains(t, string(raw), "127.0.0.1")

	r2 := newRig(t, secrets, dir)
	w2 := r2.wire(wireKnobs{})
	_, err = w2.Restore(ctx)
	require.NoError(t, err)
	me, err := w2.Me(ctx)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(me.User.AvatarURL, w2.mock.Media().BaseURL()+"/"), me.User.AvatarURL)
	img, err := w2.Avatar(ctx, me.User.AvatarURL)
	require.NoError(t, err, "the next launch's server")
	assert.Positive(t, img.Width)
}

// TestWireOverlayPersists checks the overlay of the app as it ships: it lives in
// <data>/mock-overlay.json and the next launch reads it (a profile saved stays
// saved, Q-P1-13); ResetOverlay forgets it; a shot state's overlay is in memory
// and leaves no file.
func TestWireOverlayPersists(t *testing.T) {
	t.Parallel()

	t.Run("file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		secrets := &memSecrets{vals: map[string][]byte{}}
		r := newRig(t, secrets, dir)
		w := r.wire(wireKnobs{})
		ctx := t.Context()
		_, err := w.Restore(ctx)
		require.NoError(t, err)
		require.NoError(t, w.Login(ctx, "coach@example.invalid", "secret1"))
		_, err = w.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "Lan", LastName: "Trần", Email: "lan@example.invalid"})
		require.NoError(t, err)
		require.NoError(t, w.DeleteDevice(ctx, "d_201"))
		_, err = os.Stat(filepath.Join(dir, "data", store.OverlayKey+".json"))
		require.NoError(t, err, "<data>/mock-overlay.json")

		r2 := newRig(t, secrets, dir)
		w2 := r2.wire(wireKnobs{})
		signed, err := w2.Restore(ctx)
		require.NoError(t, err)
		require.True(t, signed)
		me, err := w2.Me(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Lan", me.User.FirstName, "the next launch reads it")
		l, err := w2.Devices(ctx)
		require.NoError(t, err)
		assert.False(t, hasDevice(l, "d_201"))

		require.NoError(t, w2.ResetOverlay())
		me, err = w2.Me(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, "Lan", me.User.FirstName, "forgotten")
		l, err = w2.Devices(ctx)
		require.NoError(t, err)
		assert.True(t, hasDevice(l, "d_201"))
	})
	t.Run("memory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		r := newRig(t, nil, dir)
		w := r.wire(wireKnobs{memOverlay: true})
		ctx := t.Context()
		require.ErrorIs(t, w.ResetOverlay(), core.ErrUnsupported, "before the parts are made")
		_, err := w.Restore(ctx)
		require.NoError(t, err)
		require.NoError(t, w.Login(ctx, "coach@example.invalid", "secret1"))
		require.NoError(t, w.DeleteDevice(ctx, "d_201"))
		l, err := w.Devices(ctx)
		require.NoError(t, err)
		assert.False(t, hasDevice(l, "d_201"))
		_, err = os.Stat(filepath.Join(dir, "data", store.OverlayKey+".json"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

// TestWireOfflineReadsOverlaidAnswers checks that the answers the overlay
// changed are still kept by the API's cache (overlayTags gives them an ETag
// of their body): offline, the profile read last is answered from it.
func TestWireOfflineReadsOverlaidAnswers(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	ctx := t.Context()
	_, err := pr.w.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "Lan", LastName: "Trần", Email: "lan@example.invalid"})
	require.NoError(t, err)
	_, err = pr.w.Me(ctx)
	require.NoError(t, err)
	pr.w.tr.setOffline(true)
	me, err := pr.w.Me(ctx)
	require.NoError(t, err, "from the cache")
	assert.Equal(t, "Lan", me.User.FirstName)
}

// TestOverlayTags checks the tags of overlaid answers: a GET 200 without an ETag
// gets one made of its body, the same body the same tag, a request that carries
// it 304; an answer with its own ETag, an error and a write are left as they are.
func TestOverlayTags(t *testing.T) {
	t.Parallel()

	inner := transportFunc(func(_ context.Context, req *api.Request) (*api.Response, error) {
		switch req.Path {
		case "/tagged":
			return &api.Response{Status: http.StatusOK, Header: http.Header{"Etag": {`"m"`}}, Body: []byte("a")}, nil
		case "/fail":
			return &api.Response{Status: http.StatusNotFound, Body: []byte("x")}, nil
		default:
			return &api.Response{Status: http.StatusOK, Body: []byte(req.Path)}, nil
		}
	})
	tr := overlayTags{inner: inner}
	get := func(path, inm string) *api.Response {
		h := http.Header{}
		if inm != "" {
			h.Set("If-None-Match", inm)
		}
		resp, err := tr.Do(t.Context(), &api.Request{Method: http.MethodGet, Path: path, Header: h})
		require.NoError(t, err)

		return resp
	}
	a := get("/me", "")
	tag := a.Header.Get("ETag")
	require.NotEmpty(t, tag)
	assert.Equal(t, "/me", string(a.Body))
	assert.Equal(t, tag, get("/me", "").Header.Get("ETag"), "same body, same tag")
	assert.NotEqual(t, tag, get("/other", "").Header.Get("ETag"))
	nm := get("/me", tag)
	assert.Equal(t, http.StatusNotModified, nm.Status)
	assert.Nil(t, nm.Body)
	assert.Equal(t, http.StatusOK, get("/me", `"old"`).Status)
	assert.Equal(t, `"m"`, get("/tagged", "").Header.Get("ETag"))
	assert.Empty(t, get("/fail", "").Header.Get("ETag"))
	resp, err := tr.Do(t.Context(), &api.Request{Method: http.MethodPatch, Path: "/me", Header: http.Header{}})
	require.NoError(t, err)
	assert.Empty(t, resp.Header.Get("ETag"))
}

// TestWirePickPhoto checks the photo picker of sheet 15D (F2): it is shown on
// the UI thread over the window's canvas, its picture comes back upright and
// reduced to the working picture (a JPEG; a PNG when it has transparent parts),
// and its failures are classified as the screens need them.
func TestWirePickPhoto(t *testing.T) {
	t.Parallel()

	opaque := solidJPEG(t, 2400, 1800, color.RGBA{R: 10, G: 120, B: 200, A: 255})
	clear := clearPNG(t, 300, 200)
	tests := []struct {
		name     string
		p        pickAnswer
		noPicker bool
		check    func(t *testing.T, b []byte, err error)
	}{
		{name: "jpeg", p: pickAnswer{data: opaque}, check: func(t *testing.T, b []byte, err error) {
			t.Helper()
			require.NoError(t, err)
			cfg, format, derr := image.DecodeConfig(bytes.NewReader(b))
			require.NoError(t, derr)
			assert.Equal(t, "jpeg", format)
			assert.Equal(t, avatar.WorkSide, min(cfg.Width, cfg.Height), "the shorter side reduced (Q-P6-10)")
			assert.Equal(t, 2080, cfg.Width)
		}},
		{name: "transparent png", p: pickAnswer{data: clear}, check: func(t *testing.T, b []byte, err error) {
			t.Helper()
			require.NoError(t, err)
			_, format, derr := image.DecodeConfig(bytes.NewReader(b))
			require.NoError(t, derr)
			assert.Equal(t, "png", format)
		}},
		{name: "canceled", p: pickAnswer{err: sys.ErrCanceled}, check: func(t *testing.T, _ []byte, err error) {
			t.Helper()
			require.ErrorIs(t, err, env.ErrPhotoCanceled)
			assert.True(t, env.PhotoSilent(err))
		}},
		{name: "showing already", p: pickAnswer{start: fmtErr("sys: pick image: already showing", core.ErrDuplicate)}, check: func(t *testing.T, _ []byte, err error) {
			t.Helper()
			require.ErrorIs(t, err, core.ErrDuplicate)
			assert.True(t, env.PhotoSilent(err))
		}},
		{name: "unreadable", p: pickAnswer{data: []byte("not a picture")}, check: func(t *testing.T, _ []byte, err error) {
			t.Helper()
			require.Error(t, err)
			assert.False(t, env.PhotoSilent(err), "Không đọc được ảnh này")
		}},
		{name: "no picker", noPicker: true, check: func(t *testing.T, _ []byte, err error) {
			t.Helper()
			require.ErrorIs(t, err, core.ErrUnsupported)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fp := &fakePicker{start: tc.p.start, data: tc.p.data, err: tc.p.err}
			pr := newPlayRig(t, "", func(d *mobileDeps) {
				if !tc.noPicker {
					d.PickImage = fp.pick
				}
			})
			b, err := pickNow(pr, t.Context())
			tc.check(t, b, err)
			if !tc.noPicker {
				fp.mu.Lock()
				assert.Equal(t, 1, fp.calls)
				assert.Same(t, pr.tt.View().Canvas(), fp.canvas, "over the window")
				fp.mu.Unlock()
			}
		})
	}
	t.Run("context ends while it shows", func(t *testing.T) {
		t.Parallel()

		fp := &fakePicker{data: opaque, hold: make(chan struct{})}
		pr := newPlayRig(t, "", func(d *mobileDeps) { d.PickImage = fp.pick })
		ctx, cancel := context.WithCancel(t.Context())
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, err = pr.w.PickPhoto(ctx)
		}()
		pr.drive("the picker shows", func() bool {
			fp.mu.Lock()
			defer fp.mu.Unlock()

			return fp.calls == 1
		})
		cancel()
		<-done
		require.ErrorIs(t, err, context.Canceled)
		assert.True(t, env.PhotoSilent(err))
		close(fp.hold)
		pr.q.drain() // the late answer is dropped
	})
	t.Run("no window", func(t *testing.T) {
		t.Parallel()

		fp := &fakePicker{data: opaque}
		r := newRig(t, nil, "")
		r.deps.PickImage = fp.pick
		w := r.wire(wireKnobs{})
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, err = w.PickPhoto(t.Context())
		}()
		r.drive("the answer", func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		})
		require.ErrorIs(t, err, core.ErrUnsupported)
		assert.Zero(t, fp.calls)
	})
}

// TestWireSetAutoNext checks `pref:auto` (15): the player reads the new value at
// once and the next launch reads it from the disk.
func TestWireSetAutoNext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pr := newPlayRig(t, dir, nil)
	pr.signIn()
	p := pr.w.Prefs()
	require.True(t, p.AutoNext(), "on by default")
	an, ok := p.(env.AutoNextPrefs)
	require.True(t, ok)
	an.SetAutoNext(false)
	assert.False(t, p.AutoNext())
	pr.w.bg.Wait()

	ps := store.OpenPrefs(store.NewDir(filepath.Join(dir, "data", stateFolder)), true)
	require.NoError(t, ps.LoadErr())
	assert.False(t, ps.Get().AutoNext, "saved")
}

// TestWireLogoutWaitsForTheLastSave checks R-P6-4: the save of the position
// that Leave started (the Shell calls it on the UI thread first) is waited for
// before the session ends, so that it goes out with the user's own token; the
// server is told after it.
func TestWireLogoutWaitsForTheLastSave(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	list := pr.lessons()
	p := pr.player()
	p.PlayVideo(list[2], list)
	pr.state(player.StatePlaying)
	pr.tt.Elapse(3 * time.Second)
	saves := pr.st.calledN("lessons.progress")
	release := pr.st.holdRequest("lessons.progress")
	p.Pause()
	p.Leave()
	pr.drive("the save of the leave", func() bool { return pr.st.calledN("lessons.progress") > saves })

	done, errc := logoutAsync(pr.w)
	for range 50 {
		pr.q.drain()
		runtime.Gosched()
	}
	select {
	case <-done:
		t.Fatal("the logout did not wait for the save")
	default:
	}
	assert.Zero(t, pr.st.calledN("auth.logout"), "the server is told after the save")
	_, signed := pr.w.mgr.Session()
	assert.True(t, signed)

	release()
	pr.drive("the logout", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	require.NoError(t, <-errc)
	assert.Equal(t, pr.st.calledN("lessons.progress"), pr.st.doneN("lessons.progress"), "the save returned first")
	assert.Equal(t, 1, pr.st.calledN("auth.logout"))
	_, signed = pr.w.mgr.Session()
	assert.False(t, signed)
	assert.Zero(t, pr.w.queue.Load().Len())
}

// TestWireLoginWaitsForLogout checks that a sign-in begun while the user before
// is being signed out waits for it: the sign-out cannot end the new session.
func TestWireLoginWaitsForLogout(t *testing.T) {
	t.Parallel()

	t.Run("password", func(t *testing.T) {
		t.Parallel()

		pr := newPlayRig(t, "", nil)
		pr.signIn()
		release := pr.st.holdRequest("auth.logout")
		done, errc := logoutAsync(pr.w)
		pr.drive("the server told", func() bool { return pr.st.calledN("auth.logout") == 1 })

		logins := pr.st.calledN("auth.login") + pr.st.calledN("auth.refresh")
		loginDone := make(chan error, 1)
		go func() {
			loginDone <- pr.w.Login(t.Context(), "coach@example.invalid", "secret1")
		}()
		for range 50 {
			pr.q.drain()
			runtime.Gosched()
		}
		assert.Equal(t, logins, pr.st.calledN("auth.login")+pr.st.calledN("auth.refresh"), "the sign-in waits")

		release()
		<-done
		require.NoError(t, <-errc)
		require.NoError(t, <-loginDone)
		pr.q.drain()
		_, signed := pr.w.mgr.Session()
		assert.True(t, signed, "the new session stays")
	})
	// A kept biometric copy: the server is not told (Q-bio-logout), so the
	// copy's refresh token stays valid for the sign-in that follows.
	t.Run("biometry", func(t *testing.T) {
		t.Parallel()

		pr := newPlayRig(t, "", nil)
		pr.bio.SetKind(sys.BiometryFace)
		pr.signIn()
		require.NoError(t, pr.w.Logout(t.Context()))
		pr.q.drain()
		assert.Zero(t, pr.st.calledN("auth.logout"), "a kept copy: the server is not told")
		_, signed := pr.w.mgr.Session()
		assert.False(t, signed)
		require.NoError(t, pr.w.BioLogin(t.Context(), sys.BioPrompt{Reason: "x"}))
		pr.q.drain()
		_, signed = pr.w.mgr.Session()
		assert.True(t, signed)
	})
	t.Run("context ends", func(t *testing.T) {
		t.Parallel()

		pr := newPlayRig(t, "", nil)
		pr.signIn()
		release := pr.st.holdRequest("auth.logout")
		done, _ := logoutAsync(pr.w)
		defer func() {
			release()
			<-done
		}()
		pr.drive("the server told", func() bool { return pr.st.calledN("auth.logout") == 1 })
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, pr.w.Login(ctx, "coach@example.invalid", "secret1"), context.Canceled)
	})
}

// TestWireBeginLogoutMarksAtOnce checks that the sign-out is marked by BeginLogout
// itself, before the function it returns runs (the Shell calls it on the UI thread
// and runs the function in ui.Go): a sign-in begun in that window waits, and goes
// through once the sign-out ended.
func TestWireBeginLogoutMarksAtOnce(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	logout := pr.w.BeginLogout() // marked; nothing of the sign-out has run
	pr.w.mu.Lock()
	marked := pr.w.loggingOut != nil
	pr.w.mu.Unlock()
	assert.True(t, marked, "marked by BeginLogout itself")
	assert.Zero(t, pr.st.calledN("auth.logout"))

	logins := pr.st.calledN("auth.login") + pr.st.calledN("auth.refresh")
	loginDone := make(chan error, 1)
	go func() { loginDone <- pr.w.Login(t.Context(), "coach@example.invalid", "secret1") }()
	for range 50 {
		pr.q.drain()
		runtime.Gosched()
	}
	select {
	case err := <-loginDone:
		t.Fatalf("the sign-in did not wait: %v", err)
	default:
	}
	assert.Equal(t, logins, pr.st.calledN("auth.login")+pr.st.calledN("auth.refresh"), "the sign-in waits")

	done, errc := make(chan struct{}), make(chan error, 1)
	go func() {
		errc <- logout(context.Background())
		close(done)
	}()
	pr.drive("the first sign-out", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	require.NoError(t, <-errc)
	pr.drive("the sign-in", func() bool {
		select {
		case err := <-loginDone:
			require.NoError(t, err)

			return true
		default:
			return false
		}
	})
	_, signed := pr.w.mgr.Session()
	assert.True(t, signed, "the new session stays")
}

// TestFlowLogoutOfflineLeavesNothingQueued follows a user who signs out from
// screen 15 while a lesson plays and the network is gone (R-P6-4): the Shell
// pauses and leaves the lesson, the save of its position is queued offline and
// waited for, the sign-out empties the queue; the next sign-in sends nothing of
// the user before.
func TestFlowLogoutOfflineLeavesNothingQueued(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pb := pf.startLesson3()
		q := pf.w.queue.Load()
		require.NotNil(t, q)
		pf.tt.Elapse(3 * time.Second)
		pf.quiet()
		pf.w.tr.setOffline(true)

		pf.sh.Logout()
		assert.Equal(t, env.RouteLogin, pf.route(), "the login at once")
		assert.False(t, pb.Playing())
		pf.drive("the sign-out", func() bool {
			_, signed := pf.w.mgr.Session()
			pf.w.mu.Lock()
			going := pf.w.loggingOut != nil
			pf.w.mu.Unlock()

			return !signed && !going
		})
		pf.q.drain()
		assert.True(t, slices.ContainsFunc(pf.log.all(), func(l string) bool {
			return strings.HasPrefix(l, "dancefitvn: progress flush at logout: 0 sent, 1 dropped")
		}), "the save of the leave was waited for and queued, then dropped: %v", pf.log.all())
		assert.Zero(t, q.Len(), "nothing of the user before is queued")
		_, banner, shown := pf.login().Banner()
		assert.False(t, shown, "no banner: %q", banner)

		pf.w.tr.setOffline(false)
		pf.signIn("coach@example.invalid", "secret1")
		pf.homeLoaded()
		pf.w.flushProgress()
		pf.settleFor(5)
		pf.w.bg.Wait()
		assert.Zero(t, q.Len())
		assert.Zero(t, pf.st.calledN("progress.batch"), "nothing sent with the next token")
	})
}

// TestDownloadPauseReasonsWrapCoreSentinels checks N4: the reasons a download
// waits for that are the app's own wrap a sentinel of core.
func TestDownloadPauseReasonsWrapCoreSentinels(t *testing.T) {
	t.Parallel()

	for _, err := range []error{errDlPaused, errDlBackground} {
		require.ErrorIs(t, err, core.ErrClosed)
	}
	require.ErrorIs(t, errDlNoUser, core.ErrUnsupported)
	require.ErrorIs(t, errDlSignedOut, api.ErrSignedOut)
	require.ErrorIs(t, errDlNoNetwork, api.ErrOffline)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pick is the fake sys.PickImage.
func (p *fakePicker) pick(c platform.Canvas, d sys.Dispatch, done func(data []byte, mime string, err error)) error {
	p.mu.Lock()
	p.calls++
	p.canvas = c
	start, data, err, hold := p.start, p.data, p.err, p.hold
	p.mu.Unlock()
	if start != nil {
		return start
	}
	go func() {
		if hold != nil {
			<-hold
		}
		d(func() { done(data, "image/jpeg", err) })
	}()

	return nil
}

// pickNow runs PickPhoto off the test's goroutine, which runs the UI thread's
// work meanwhile, and returns its answer.
func pickNow(pr *playRig, ctx context.Context) ([]byte, error) { //nolint:revive // the rig first, as the other helpers
	var b []byte
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		b, err = pr.w.PickPhoto(ctx)
	}()
	pr.drive("the picker's answer", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})

	return b, err
}

// logoutAsync starts w.Logout on its own goroutine, as the Shell's ui.Go does;
// done closes when it returned, its error then in errc.
func logoutAsync(w *mobileWire) (done <-chan struct{}, errc <-chan error) {
	d := make(chan struct{})
	e := make(chan error, 1)
	go func() {
		e <- w.Logout(context.Background())
		close(d)
	}()

	return d, e
}

// logoutNow signs out as the Shell does (off the UI thread) while the test's
// goroutine runs the UI thread's work, and returns the error.
func logoutNow(r *rig) error {
	done, errc := logoutAsync(r.w)
	r.drive("the logout", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})

	return <-errc
}

// hasDevice tells whether the list holds the device.
func hasDevice(l model.DeviceList, id string) bool {
	return slices.ContainsFunc(l.Items, func(d model.Device) bool { return d.ID == id })
}

// solidJPEG is a JPEG of w x h of one colour.
func solidJPEG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}))

	return buf.Bytes()
}

// clearPNG is a PNG of w x h whose left half is transparent.
func clearPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := w / 2; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// fmtErr is an error of text wrapping sentinel.
func fmtErr(text string, sentinel error) error { return errors.Join(errors.New(text), sentinel) }

// transportFunc is an api.Transport of a function.
type transportFunc func(ctx context.Context, req *api.Request) (*api.Response, error)

// Do calls f.
func (f transportFunc) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	return f(ctx, req)
}
