//go:build !tvos

package shelltest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"strings"
	"sync"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/ui/mobile/env"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// FakeAccount is the knobs and the record of the account of a Fake
// (env.AccountServices): the calls go through the real api.Client to the mock, so
// every scenario of routes.json is the mock's (Fake.SetScenario("me.update",
// "validation_failed"), Fake.Fail, Fake.Gate, Fake.SetOnline(false)); what the
// mock cannot do, because its answers are fixed, is done here as the overlay of the
// app does: a write is kept (the next read shows it), a device deleted stays
// deleted, a TV confirmed is listed, and the photo and the sign-out are scripted.
// Get it with Fake.Account.
type FakeAccount struct {
	f *Fake

	mu     sync.Mutex
	log    []string
	tvOff  bool                    // SetTVLinked(false)
	gone   map[string]bool         // devices deleted
	added  []model.Device          // TVs confirmed
	user   *model.User             // after UpdateMe or UploadAvatar
	coach  *model.CoachProfile     // after UpdateCoach
	avatar []byte                  // the last upload
	pwds   [][2]string             // the passwords sent: current, new
	reqs   []model.MeUpdateRequest // the profiles sent
	coachR []model.CoachUpdateRequest
	limit  bool // SetDeviceLimit(true)

	photo       []byte
	photoErr    error
	photoGate   chan struct{}
	logoutGate  chan struct{}
	logoutErr   error
	logoutSaw   []string // what the player had been told when Logout began
	logoutBegan bool
	logoutDone  chan struct{} // closed when the last Logout that was marked has ended
}

var _ env.AccountServices = (*Fake)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newFakeAccount returns the account of f: the mock's devices as they are (the
// TV of the mock is linked), a picture that answers at once.
func newFakeAccount(f *Fake) *FakeAccount {
	return &FakeAccount{f: f, gone: map[string]bool{}}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Account returns the account of the Fake with its knobs and record.
func (f *Fake) Account() *FakeAccount { return f.acct }

// Log returns the calls received, in order: "Me", "UpdateMe", "ChangePassword",
// "Coach", "UpdateCoach", "UploadAvatar", "PickPhoto", "Devices",
// "DeleteDevice:<id>", "PairConfirm:<code as sent>", "Logout".
func (a *FakeAccount) Log() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string(nil), a.log...)
}

// Count returns how many calls were received that are, or begin with, name
// followed by ":".
func (a *FakeAccount) Count(name string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, c := range a.log {
		if c == name || strings.HasPrefix(c, name+":") {
			n++
		}
	}

	return n
}

// ClearLog forgets the calls received.
func (a *FakeAccount) ClearLog() {
	a.mu.Lock()
	a.log = nil
	a.mu.Unlock()
}

// SetTVLinked says whether the account starts with its Apple TV linked: the mock's
// list holds one (d_201), the prototype starts without (`tvLinked:false`); false
// takes the tvOS devices out of the list until PairConfirm puts one back.
func (a *FakeAccount) SetTVLinked(on bool) {
	a.mu.Lock()
	a.tvOff = !on
	a.mu.Unlock()
}

// SetDeviceLimit makes tv.pair.confirm answer device_limit (409, the devices of
// errors/device_limit.json) until a device is deleted, as a server over the
// limit does; false puts the route back.
func (a *FakeAccount) SetDeviceLimit(on bool) {
	a.mu.Lock()
	a.limit = on
	a.mu.Unlock()
	if on {
		a.f.Fail("tv.pair.confirm", 409, "errors/device_limit.json")

		return
	}
	a.f.Fail("tv.pair.confirm", 0, "")
}

// SetPhoto sets what PickPhoto returns: the encoded picture. nil gives the
// default, a generated 640 x 480 JPEG.
func (a *FakeAccount) SetPhoto(b []byte) {
	a.mu.Lock()
	a.photo = b
	a.mu.Unlock()
}

// SetPhotoError makes PickPhoto fail with err: env.ErrPhotoCanceled for the user
// leaving the picker, core.ErrUnsupported, core.ErrDuplicate, or any other error
// for a picture that cannot be read. nil answers the picture again.
func (a *FakeAccount) SetPhotoError(err error) {
	a.mu.Lock()
	a.photoErr = err
	a.mu.Unlock()
}

// HoldPhoto makes PickPhoto wait (the picker is on the screen) until the release is
// called or its context ends.
func (a *FakeAccount) HoldPhoto() (release func()) {
	ch := make(chan struct{})
	a.mu.Lock()
	a.photoGate = ch
	a.mu.Unlock()
	var once sync.Once

	return func() { once.Do(func() { close(ch) }) }
}

// HoldLogout makes Logout wait (the queue is being sent) until the release is called
// or its context ends: the screen must have left, and a Login must wait.
func (a *FakeAccount) HoldLogout() (release func()) {
	ch := make(chan struct{})
	a.mu.Lock()
	a.logoutGate = ch
	a.mu.Unlock()
	var once sync.Once

	return func() { once.Do(func() { close(ch) }) }
}

// SetLogoutError makes Logout return err after it did its work.
func (a *FakeAccount) SetLogoutError(err error) {
	a.mu.Lock()
	a.logoutErr = err
	a.mu.Unlock()
}

// LogoutSaw returns what the player had been told (FakePlayback.Calls) when Logout
// began, nil when it did not or the player is not a FakePlayback: "Leave" there
// means it was told first.
func (a *FakeAccount) LogoutSaw() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string(nil), a.logoutSaw...)
}

// LogoutMarked tells whether a sign-out is marked under way (BeginLogout called,
// its function not yet ended): what a Login waits for.
func (a *FakeAccount) LogoutMarked() bool {
	a.mu.Lock()
	done := a.logoutDone
	a.mu.Unlock()
	if done == nil {
		return false
	}
	select {
	case <-done:
		return false
	default:
		return true
	}
}

// LogoutBegan tells whether Logout was called.
func (a *FakeAccount) LogoutBegan() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.logoutBegan
}

// Avatar returns the last picture uploaded, nil when none.
func (a *FakeAccount) Avatar() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.avatar
}

// Passwords returns the passwords ChangePassword was given: {current, new}.
func (a *FakeAccount) Passwords() [][2]string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([][2]string(nil), a.pwds...)
}

// Updates returns the profiles UpdateMe was given.
func (a *FakeAccount) Updates() []model.MeUpdateRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]model.MeUpdateRequest(nil), a.reqs...)
}

// CoachUpdates returns the coach profiles UpdateCoach was given.
func (a *FakeAccount) CoachUpdates() []model.CoachUpdateRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]model.CoachUpdateRequest(nil), a.coachR...)
}

// SessionUser is the user of the session.
func (f *Fake) SessionUser() (model.User, bool) {
	s, ok := f.mgr.Session()
	if !ok {
		return model.User{}, false
	}
	u := s.User

	return model.User{
		ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, FullName: u.FullName,
		Email: u.Email, AvatarURL: u.AvatarURL,
	}, true
}

// Me is GET /me, with what the writes changed.
func (f *Fake) Me(ctx context.Context) (model.Me, error) {
	f.acct.note("Me")
	m, err := f.client.Me(ctx)
	f.mgr.Handle(err)
	if err == nil {
		f.acct.mu.Lock()
		if f.acct.user != nil {
			m.User = *f.acct.user
		}
		f.acct.mu.Unlock()
	}

	return m, err
}

// UpdateMe is PATCH /me; the user it answers is the one the request makes, which
// is kept.
func (f *Fake) UpdateMe(ctx context.Context, r model.MeUpdateRequest) (model.MeUpdate, error) {
	f.acct.note("UpdateMe")
	f.acct.mu.Lock()
	f.acct.reqs = append(f.acct.reqs, r)
	f.acct.mu.Unlock()
	res, err := f.client.UpdateMe(ctx, r)
	f.mgr.Handle(err)
	if err != nil {
		return res, err
	}
	f.acct.mu.Lock()
	defer f.acct.mu.Unlock()
	u := res.User
	if f.acct.user != nil {
		u = *f.acct.user
	}
	u.FirstName, u.LastName, u.Email, u.Phone = r.FirstName, r.LastName, r.Email, r.Phone
	u.FullName = strings.TrimSpace(r.LastName + " " + r.FirstName)
	f.acct.user = &u
	res.User = u

	return res, nil
}

// ChangePassword is PUT /me/password.
func (f *Fake) ChangePassword(ctx context.Context, current, next string) (model.PasswordResult, error) {
	f.acct.note("ChangePassword")
	f.acct.mu.Lock()
	f.acct.pwds = append(f.acct.pwds, [2]string{current, next})
	f.acct.mu.Unlock()
	res, err := f.client.ChangePassword(ctx, model.PasswordRequest{CurrentPassword: current, NewPassword: next})
	f.mgr.Handle(err)

	return res, err
}

// Coach is GET /me/coach-profile, with what UpdateCoach changed.
func (f *Fake) Coach(ctx context.Context) (model.CoachProfile, error) {
	f.acct.note("Coach")
	c, err := f.client.CoachProfile(ctx)
	f.mgr.Handle(err)
	if err == nil {
		f.acct.mu.Lock()
		if f.acct.coach != nil {
			c = *f.acct.coach
		}
		f.acct.mu.Unlock()
	}

	return c, err
}

// UpdateCoach is PUT /me/coach-profile; the profile it answers is the one the
// request makes (platforms read from the links), which is kept.
func (f *Fake) UpdateCoach(ctx context.Context, r model.CoachUpdateRequest) (model.CoachProfile, error) {
	f.acct.note("UpdateCoach")
	f.acct.mu.Lock()
	f.acct.coachR = append(f.acct.coachR, r)
	f.acct.mu.Unlock()
	res, err := f.client.UpdateCoachProfile(ctx, r)
	f.mgr.Handle(err)
	if err != nil {
		return res, err
	}
	f.acct.mu.Lock()
	defer f.acct.mu.Unlock()
	c := res
	if f.acct.coach != nil {
		c = *f.acct.coach
	}
	c.Bio = r.Bio
	c.Places = append([]string(nil), r.Places...)
	c.Socials = c.Socials[:0:0]
	for _, s := range r.Socials {
		c.Socials = append(c.Socials, model.Social{Platform: platformOf(s.URL, res.AllowedPlatforms), URL: s.URL})
	}
	f.acct.coach = &c

	return c, nil
}

// UploadAvatar is PUT /me/avatar; the URL it answers is kept as the user's.
func (f *Fake) UploadAvatar(ctx context.Context, jpegBytes []byte) (model.Avatar, error) {
	f.acct.note("UploadAvatar")
	res, err := f.client.UploadAvatar(ctx, jpegBytes, "avatar.jpg")
	f.mgr.Handle(err)
	if err != nil {
		return res, err
	}
	f.acct.mu.Lock()
	defer f.acct.mu.Unlock()
	f.acct.avatar = append([]byte(nil), jpegBytes...)
	if f.acct.user == nil {
		m, merr := f.client.Me(ctx)
		if merr != nil {
			return res, nil
		}
		f.acct.user = &m.User
	}
	f.acct.user.AvatarURL = res.AvatarURL

	return res, nil
}

// PickPhoto answers as the knobs say: after the hold of HoldPhoto, the error of
// SetPhotoError, else the picture (SetPhoto, or a generated one).
func (f *Fake) PickPhoto(ctx context.Context) ([]byte, error) {
	f.acct.note("PickPhoto")
	f.acct.mu.Lock()
	gate, err, photo := f.acct.photoGate, f.acct.photoErr, f.acct.photo
	f.acct.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, fmt.Errorf("shelltest: pick photo: %w", ctx.Err())
		}
	}
	if err != nil {
		return nil, fmt.Errorf("shelltest: pick photo: %w", err)
	}
	if photo == nil {
		photo = GeneratedPhoto(640, 480)
	}

	return photo, nil
}

// Devices is GET /me/devices: the mock's list without the devices deleted and
// without the TV when SetTVLinked(false), with the TVs confirmed.
func (f *Fake) Devices(ctx context.Context) (model.DeviceList, error) {
	f.acct.note("Devices")
	l, err := f.client.Devices(ctx)
	f.mgr.Handle(err)
	if err != nil {
		return l, err
	}
	a := f.acct
	a.mu.Lock()
	defer a.mu.Unlock()
	items := make([]model.Device, 0, len(l.Items)+len(a.added))
	for _, d := range l.Items {
		if a.gone[d.ID] || (a.tvOff && d.Platform == model.PlatformTVOS && !a.listed(d.ID)) {
			continue
		}
		items = append(items, d)
	}
	for _, d := range a.added {
		if !containsDevice(items, d.ID) {
			items = append(items, d)
		}
	}
	l.Items = items

	return l, nil
}

// DeleteDevice is DELETE /me/devices/{id}: a device deleted (or the server's 404)
// stays out of the list, and the limit of device_limit is lifted.
func (f *Fake) DeleteDevice(ctx context.Context, id string) error {
	f.acct.note("DeleteDevice:" + id)
	err := f.client.DeleteDevice(ctx, id)
	f.mgr.Handle(err)
	// the mock's scenario not_found says the device is not there: gone all the same
	var ae *api.Error
	gone := err == nil || (errors.As(err, &ae) && ae.Status == http.StatusNotFound)
	a := f.acct
	a.mu.Lock()
	if gone {
		a.gone[id] = true
		a.added = dropDevice(a.added, id)
	}
	lift := gone && a.limit
	if lift {
		a.limit = false
	}
	a.mu.Unlock()
	if lift {
		f.Fail("tv.pair.confirm", 0, "")
	}

	return err
}

// PairConfirm is pairing.Phone.Confirm over the mock: the code is normalised
// (core.ErrInvalid for what is not one, nothing sent), the device of the answer
// is listed from then on.
func (f *Fake) PairConfirm(ctx context.Context, code string) (model.PairConfirm, error) {
	f.acct.note("PairConfirm:" + code)
	res, err := pairing.NewPhone(f.client).Confirm(ctx, code)
	f.mgr.Handle(err)
	if err != nil {
		return res, err
	}
	a := f.acct
	a.mu.Lock()
	delete(a.gone, res.Device.ID)
	a.added = dropDevice(a.added, res.Device.ID)
	a.added = append(a.added, model.Device{ID: res.Device.ID, Platform: res.Device.Platform, Name: res.Device.Name})
	a.mu.Unlock()

	return res, nil
}

// Logout is BeginLogout()(ctx).
func (f *Fake) Logout(ctx context.Context) error { return f.BeginLogout()(ctx) }

// BeginLogout marks the sign-out under way at once (a Login waits from here: see
// LogoutMarked) and returns the sign-out, which waits for the one marked before,
// notes what the player had been told, waits for the hold of HoldLogout, ends the
// session as auth.Manager.Logout does (the server told, the vault cleared,
// OnSignedOut not called) and answers the error of SetLogoutError.
func (f *Fake) BeginLogout() func(ctx context.Context) error {
	a := f.acct
	done := make(chan struct{})
	a.mu.Lock()
	prev := a.logoutDone
	a.logoutDone = done
	a.mu.Unlock()

	return func(ctx context.Context) error {
		defer close(done)
		if prev != nil {
			select {
			case <-prev:
			case <-ctx.Done():
			}
		}
		a.note("Logout")
		var saw []string
		if pb := f.FakePlayback(); pb != nil {
			saw = pb.Calls()
		}
		a.mu.Lock()
		a.logoutBegan, a.logoutSaw = true, saw
		gate, err := a.logoutGate, a.logoutErr
		a.mu.Unlock()
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
				return fmt.Errorf("shelltest: logout: %w", ctx.Err())
			}
		}
		if lerr := f.mgr.Logout(ctx); lerr != nil {
			return lerr
		}

		return err
	}
}

// waitLogout holds a Login while a Logout that began has not ended, as the app's
// does.
func (a *FakeAccount) waitLogout(ctx context.Context) error {
	a.mu.Lock()
	done := a.logoutDone
	a.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shelltest: login waits for logout: %w", ctx.Err())
	}
}

// GeneratedPhoto returns a JPEG of w x h made of a diagonal gradient and a white
// disc: a picture that is not anyone's (05 section 7), where a crop shows.
func GeneratedPhoto(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy, r := w/2, h/2, min(w, h)/4
	for y := range h {
		for x := range w {
			c := color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 160, A: 255}
			if dx, dy := x-cx, y-cy; dx*dx+dy*dy < r*r {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil
	}

	return buf.Bytes()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// note writes a call down.
func (a *FakeAccount) note(s string) {
	a.mu.Lock()
	a.log = append(a.log, s)
	a.mu.Unlock()
}

// listed tells whether a TV was confirmed (PairConfirm) and is in the list: it
// stays when SetTVLinked(false) took the mock's out.
func (a *FakeAccount) listed(id string) bool { return containsDevice(a.added, id) }

// containsDevice tells whether the list has the device.
func containsDevice(list []model.Device, id string) bool {
	for _, d := range list {
		if d.ID == id {
			return true
		}
	}

	return false
}

// dropDevice returns the list without the device.
func dropDevice(list []model.Device, id string) []model.Device {
	out := make([]model.Device, 0, len(list))
	for _, d := range list {
		if d.ID != id {
			out = append(out, d)
		}
	}

	return out
}

// platformOf returns the first platform of the list that the link names, "link"
// for none, as the prototype's `plat`.
func platformOf(url string, allowed []string) string {
	low := strings.ToLower(url)
	for _, p := range allowed {
		if strings.Contains(low, p) {
			return p
		}
	}

	return "link"
}
