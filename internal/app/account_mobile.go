//go:build !tvos

// The account's side of the phone screens' contract (env.AccountServices):
// the profile, the coach profile, the avatar, the password, the devices,
// the TV code and the sign-out the user asks for (local/plans/P6.md
// P6.1b, docs/plan/03 sections 3, 6, 8). The pattern of PickPhoto (the
// picker on the UI thread, its answer decoded off it) is
// jplayer-mobile/internal/app/actionport.go:417-450 at 49de3c2.

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// avatarName is the file name the avatar is sent under (multipart "file").
const avatarName = "avatar.jpg"

// workJPEGQuality is the quality of the working picture PickPhoto hands
// the cropper when it is opaque: the avatar is encoded again at 90 from it.
const workJPEGQuality = 95

// avatarDumpEnv, in a developer's build, names a file the avatar sent is
// written to (an absolute path, or a name under the app's cache folder):
// the device's proof that the crop is upright and 512×512 (P6.3 step 4).
const avatarDumpEnv = "DFV_AVATAR_DUMP"

// errNoWindow is a picker asked for before the window is there.
var errNoWindow = fmt.Errorf("app: pick photo: no window: %w", core.ErrUnsupported)

// pickImageFunc is sys.PickImage: the system's picker of one picture.
type pickImageFunc func(c platform.Canvas, d sys.Dispatch, done func(data []byte, mime string, err error)) error

// picked is the answer of the system's picker.
type picked struct {
	data []byte
	err  error
}

var _ env.AccountServices = (*mobileWire)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SessionUser is the user the session was opened with; false before the
// parts are made or when nobody is signed in. Cheap; any goroutine.
func (w *mobileWire) SessionUser() (model.User, bool) {
	if !w.built.Load() {
		return model.User{}, false
	}
	s, ok := w.mgr.Session()
	if !ok {
		return model.User{}, false
	}
	u := s.User

	return model.User{
		ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, FullName: u.FullName,
		Email: u.Email, AvatarURL: u.AvatarURL,
	}, true
}

// Me is GET /me.
func (w *mobileWire) Me(ctx context.Context) (model.Me, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.Me, error) { return w.client.Me(ctx) })
}

// UpdateMe is PATCH /me. The mock answers the same user whatever was sent
// (its files are fixed): over it (in the process, or cmd/mockserve's), the
// user answered is the one GET /me reads back through the overlay, which
// kept the change, as a live server's answer would be (readBack).
func (w *mobileWire) UpdateMe(ctx context.Context, r model.MeUpdateRequest) (model.MeUpdate, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.MeUpdate, error) {
		res, err := w.client.UpdateMe(ctx, r)
		if err != nil || !w.readBack {
			return res, err
		}
		if m, merr := w.client.Me(ctx); merr == nil {
			res.User = m.User
		}

		return res, nil
	})
}

// ChangePassword is PUT /me/password; the session is kept (Q-P6-23).
func (w *mobileWire) ChangePassword(ctx context.Context, current, next string) (model.PasswordResult, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.PasswordResult, error) {
		return w.client.ChangePassword(ctx, model.PasswordRequest{CurrentPassword: current, NewPassword: next})
	})
}

// Coach is GET /me/coach-profile.
func (w *mobileWire) Coach(ctx context.Context) (model.CoachProfile, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.CoachProfile, error) { return w.client.CoachProfile(ctx) })
}

// UpdateCoach is PUT /me/coach-profile; over the mock, the profile
// answered is the one GET /me/coach-profile reads back through the overlay
// (as UpdateMe).
func (w *mobileWire) UpdateCoach(ctx context.Context, r model.CoachUpdateRequest) (model.CoachProfile, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.CoachProfile, error) {
		res, err := w.client.UpdateCoachProfile(ctx, r)
		if err != nil || !w.readBack {
			return res, err
		}
		if c, cerr := w.client.CoachProfile(ctx); cerr == nil {
			return c, nil
		}

		return res, nil
	})
}

// UploadAvatar is PUT /me/avatar. The picture sent is put in the image
// cache under the URL the server answers, so that Avatar(url) draws it at
// once, without reading it back.
func (w *mobileWire) UploadAvatar(ctx context.Context, jpegBytes []byte) (model.Avatar, error) {
	w.dumpAvatar(jpegBytes)
	a, err := callAPI(w, ctx, func(ctx context.Context) (model.Avatar, error) {
		return w.client.UploadAvatar(ctx, jpegBytes, avatarName)
	})
	if err != nil {
		return a, err
	}
	w.primeImage(ctx, a.AvatarURL, jpegBytes)
	w.deps.Logf("dancefitvn: avatar: %d bytes sent", len(jpegBytes))

	return a, nil
}

// PickPhoto shows the system's picker of one picture on the UI thread and
// waits for it; the picture is decoded, turned upright and reduced to the
// working picture of sheet 15D off the UI thread (avatar.Decode), then
// encoded again: a JPEG of quality 95, or a PNG when it has transparent
// parts (the cropper fills those with the frame's colour). The user leaving
// the picker wraps env.ErrPhotoCanceled; no window or no picker on this
// platform is core.ErrUnsupported, one showing already core.ErrDuplicate.
// The context ending stops the wait, not the picker: what it answers then
// is dropped. Blocks: call it from ui.Go.
func (w *mobileWire) PickPhoto(ctx context.Context) ([]byte, error) {
	pick := w.deps.PickImage
	if pick == nil {
		return nil, fmt.Errorf("app: pick photo: %w", core.ErrUnsupported)
	}
	answer := make(chan picked, 1)
	started := make(chan error, 1)
	queued := w.deps.Enqueue(func() {
		if ctx.Err() != nil {
			started <- ctx.Err()

			return
		}
		w.mu.Lock()
		v := w.view
		w.mu.Unlock()
		if v == nil {
			started <- errNoWindow

			return
		}
		started <- pick(v.Canvas(), w.dispatch, func(data []byte, _ string, err error) {
			answer <- picked{data: data, err: err}
		})
	})
	if !queued {
		return nil, errWireClosed
	}
	var p picked
	select {
	case err := <-started:
		if err != nil {
			return nil, fmt.Errorf("app: pick photo: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("app: pick photo: %w", ctx.Err())
	}
	select {
	case p = <-answer:
	case <-ctx.Done():
		return nil, fmt.Errorf("app: pick photo: %w", ctx.Err())
	}
	switch {
	case errors.Is(p.err, sys.ErrCanceled):
		return nil, fmt.Errorf("app: pick photo: %w", env.ErrPhotoCanceled)
	case p.err != nil:
		return nil, fmt.Errorf("app: pick photo: %w", p.err)
	default:
	}
	img, err := avatar.Decode(p.data)
	if err != nil {
		// the system hands JPEG and PNG only: anything else is a picture
		// that cannot be read, which the screen says (not a silent
		// core.ErrUnsupported)
		return nil, fmt.Errorf("app: pick photo: %w: %v", core.ErrInvalid, err) //nolint:errorlint // the picture's error is for the log
	}

	return encodeWorking(img)
}

// Devices is GET /me/devices.
func (w *mobileWire) Devices(ctx context.Context) (model.DeviceList, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.DeviceList, error) { return w.client.Devices(ctx) })
}

// DeleteDevice is DELETE /me/devices/{id}.
func (w *mobileWire) DeleteDevice(ctx context.Context, id string) error {
	_, err := callAPI(w, ctx, func(ctx context.Context) (struct{}, error) { return struct{}{}, w.client.DeleteDevice(ctx, id) })

	return err
}

// PairConfirm is POST /tv/pair/confirm through pairing.Phone: the code
// normalised, core.ErrInvalid for what is not one (nothing sent).
func (w *mobileWire) PairConfirm(ctx context.Context, code string) (model.PairConfirm, error) {
	return callAPI(w, ctx, func(ctx context.Context) (model.PairConfirm, error) {
		return pairing.NewPhone(w.client).Confirm(ctx, code)
	})
}

// Logout signs out at the user's request (Q-P4.6a-1, Q-P6-13). The Shell
// stopped the sound and made the player Leave on the UI thread before it;
// here, within logoutBudget, the save of the position that Leave started is
// waited for (progress.Tracker.Wait: online it is sent, offline it is
// queued), then the queue is sent once with the token still valid. Then the
// session ends (auth.logout told, best effort; the vault and the cache
// cleared) and the queue is emptied, so that no position of this user goes
// out with the next one's token. The downloaded files stay (Q-P5-8). A Login
// begun meanwhile waits for it. Not on the UI thread: the end of a save runs
// there.
func (w *mobileWire) Logout(ctx context.Context) error {
	return w.BeginLogout()(ctx)
}

// BeginLogout marks a sign-out under way at once and without blocking (the
// mark is what Login and BioLogin wait for), and returns the sign-out itself:
// the Shell calls BeginLogout on the UI thread before it routes to the login,
// then runs the returned function off it, so that a Login begun in the window
// between the two already waits. The function must be called, once: the mark
// ends with it. It first waits for the sign-out that was marked before (or for
// ctx).
func (w *mobileWire) BeginLogout() (logout func(ctx context.Context) error) {
	prev, end := w.markLogout()

	return func(ctx context.Context) error {
		defer end()
		select {
		case <-prev:
		case <-ctx.Done():
		}

		return w.logout(ctx)
	}
}

// logout is the sign-out of Logout, once marked and the one before it ended.
func (w *mobileWire) logout(ctx context.Context) error {
	began := time.Now()
	if err := w.setup(ctx); err != nil {
		return err
	}
	budget, cancel := context.WithTimeout(ctx, logoutBudget)
	defer cancel()
	if t := w.tracker.Load(); t != nil {
		if err := t.Wait(budget); err != nil {
			w.deps.Logf("dancefitvn: logout: the last save of the position: %v", err)
		}
	}
	w.flushBeforeLogout(budget)
	sctx, scancel := context.WithTimeout(ctx, logoutServerTimeout)
	err := w.mgr.Logout(sctx)
	scancel()
	w.clearQueue() // clearCache empties it too, later, on the UI thread
	w.deps.Logf("dancefitvn: logout: signed out in %v", time.Since(began).Round(time.Millisecond))
	if err != nil {
		return fmt.Errorf("app: logout: %w", err)
	}

	return nil
}

// AppVersion is the version of the app (gdesk.json's), which screen 15
// shows (screens.Versioned, K2).
func (w *mobileWire) AppVersion() string { return appVersion }

// ResetOverlay forgets what the user changed of the mock's data (the
// profile, the coach profile, the avatar, the progress, the devices): the
// dev menu's "Reset overlay" (docs/plan/05 section 4). The API's cache is
// emptied with it, so that no read shows the changes again.
// core.ErrUnsupported before the parts are made, and without the mock (the
// shipped live build has no overlay to reset).
func (w *mobileWire) ResetOverlay() error {
	if !devParts || !w.built.Load() || w.overlay == nil {
		return fmt.Errorf("app: reset overlay: %w", core.ErrUnsupported)
	}
	if err := w.overlay.Reset(); err != nil {
		return fmt.Errorf("app: reset overlay: %w", err)
	}
	w.cache.Clear()

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// callAPI runs fn after setup, under the session (scoped), and hands its
// error to the Manager: what every account's call does. fn reads w.client
// itself, after setup made it.
func callAPI[T any](w *mobileWire, ctx context.Context, fn func(ctx context.Context) (T, error)) (T, error) { //nolint:revive // the wire first reads as a method
	if err := w.setup(ctx); err != nil {
		var zero T

		return zero, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	v, err := fn(ctx)
	w.mgr.Handle(err)

	return v, err
}

// markLogout marks a sign-out under way and returns the channel of the one
// marked before it (closed when none, or when it ended); the returned end
// unmarks this one. Login and BioLogin wait for the mark (waitLogout).
func (w *mobileWire) markLogout() (prev <-chan struct{}, end func()) {
	ch := make(chan struct{})
	w.mu.Lock()
	before := w.loggingOut
	w.loggingOut = ch
	w.mu.Unlock()
	if before == nil {
		before = make(chan struct{})
		close(before)
	}

	return before, func() {
		w.mu.Lock()
		if w.loggingOut == ch {
			w.loggingOut = nil
		}
		w.mu.Unlock()
		close(ch)
	}
}

// waitLogout waits for the sign-out under way, if any: a sign-in begun
// while the user before is being signed out must not be ended by it.
func (w *mobileWire) waitLogout(ctx context.Context) error {
	w.mu.Lock()
	ch := w.loggingOut
	w.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("app: sign in: waiting for the sign-out: %w", ctx.Err())
	}
}

// flushBeforeLogout sends the positions saved offline once, with the token
// still valid, within ctx (best effort: a failure is logged and the
// sign-out goes on).
func (w *mobileWire) flushBeforeLogout(ctx context.Context) {
	q := w.queue.Load()
	if q == nil || q.Len() == 0 {
		return
	}
	if _, ok := w.mgr.Session(); !ok {
		return
	}
	fctx, done := w.scoped(ctx)
	defer done()
	res, err := q.Flush(fctx)
	if err != nil {
		w.deps.Logf("dancefitvn: progress flush at logout: %d sent, %d dropped: %v", res.Sent, q.Len(), err)
	}
}

// primeImage puts the picture of data in the image cache under url, in
// place of what was there, so that the screens drawing url show it.
func (w *mobileWire) primeImage(ctx context.Context, url string, data []byte) {
	if url == "" {
		return
	}
	w.images.Forget(url)
	img, err := components.DecodeImage(data)
	if err != nil {
		w.deps.Logf("dancefitvn: avatar: the picture sent: %v", err)

		return
	}
	if _, err := w.images.Loader(url, func(context.Context) (*scene.Image, error) { return img, nil })(ctx); err != nil {
		w.deps.Logf("dancefitvn: avatar: cache: %v", err)
	}
}

// dumpAvatar writes the avatar to the file of DFV_AVATAR_DUMP, in a
// developer's build.
func (w *mobileWire) dumpAvatar(data []byte) {
	path := devEnv(avatarDumpEnv)
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		cache, err := w.deps.CacheDir()
		if err != nil {
			w.deps.Logf("dancefitvn: avatar dump: %v", err)

			return
		}
		path = filepath.Join(cache, path)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		w.deps.Logf("dancefitvn: avatar dump: %v", err)

		return
	}
	w.deps.Logf("dancefitvn: avatar dump: %d bytes at %s", len(data), path)
}

// encodeWorking encodes the working picture for the cropper: a JPEG when
// it is opaque, a PNG when it has transparent parts (a JPEG would turn them
// black).
func encodeWorking(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	if img.Opaque() {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: workJPEGQuality})
	} else {
		err = png.Encode(&buf, img)
	}
	if err != nil {
		return nil, fmt.Errorf("app: pick photo: encode: %w", err)
	}

	return buf.Bytes(), nil
}
