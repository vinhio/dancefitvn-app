//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/port.go at 49de3c2 (the
// pattern: an optional interface the screens read a part of the app through,
// the kept view a rebuilt screen draws at once, the errors of the core packages
// classified here so a screen imports none of them). The routes are those of
// docs/plan/05 section 2; the behaviour is docs/plan/03 section 3, and the
// error table is local/plans/P6.md section 5.

package env

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The ids of the sheets of the account (kit.Sheet content; the prototype's
// `S.m.sheet`, the `data-v` of the buttons that open them). The layout.json of
// M15-logout and the tests call them by these.
const (
	// SheetPassword is SH.password (M15B-password), SheetLogout SH.logout
	// (M15-logout), SheetAvatar SH.avatar (15D, M15D) and SheetDevices the sheet
	// of the devices a TV code cannot be confirmed over (03 section 9).
	SheetPassword = "password"
	SheetLogout   = "logout"
	SheetAvatar   = "avatar"
	SheetDevices  = "devices"
)

// ErrPhotoCanceled is the user leaving the photo picker without a picture
// (AccountServices.PickPhoto). The app wraps sys.ErrCanceled with it; a screen
// shows nothing for it.
var ErrPhotoCanceled = errors.New("env: photo picker canceled")

// AccountServices is the part of Services that holds the account: the profile
// (screens 15, 15B), the coach profile (15C), the avatar (15D), the password
// (sheet), the devices and the TV code (16), the sign-out. An Env with no
// Services fails each of these with core.ErrUnsupported (Env.Account). The Shell
// and the screens go through Env.Account, never the Services' own, so that Env
// keeps what they answered (Env.AccountView).
//
// Every method that takes a context blocks: call it from a ui.Go work function,
// never from the UI thread; SessionUser is cheap and for the UI thread. Every
// error path runs auth.Manager.Handle(err) inside the implementation, as the
// methods of Services do, and a context canceled (the screen was left) ends with
// context.Canceled, which no screen shows. The failures of the server are
// *api.Error (FailureOf says which); a network that is down wraps api.ErrOffline.
type AccountServices interface {
	// SessionUser is the user the session was opened with, false when signed out:
	// the name, the email and the avatar a screen draws before GET /me answers.
	SessionUser() (model.User, bool)
	// Me is GET /me (me.get).
	Me(ctx context.Context) (model.Me, error)
	// UpdateMe is PATCH /me (me.update): the phone as digits, the other fields as
	// typed and trimmed. The answer's EmailChangePending is set when the server
	// waits for the new address to be confirmed.
	UpdateMe(ctx context.Context, r model.MeUpdateRequest) (model.MeUpdate, error)
	// ChangePassword is PUT /me/password (me.password). The session and the
	// biometric copy are kept (Q-P6-23). A wrong current password is an
	// *api.Error with Code "wrong_password".
	ChangePassword(ctx context.Context, current, next string) (model.PasswordResult, error)
	// Coach is GET /me/coach-profile (me.coach.get).
	Coach(ctx context.Context) (model.CoachProfile, error)
	// UpdateCoach is PUT /me/coach-profile (me.coach.update). A link of a platform
	// the server does not take is an *api.Error with Code "invalid_social" whose
	// Body.Fields name "socials[i].url".
	UpdateCoach(ctx context.Context, r model.CoachUpdateRequest) (model.CoachProfile, error)
	// UploadAvatar is PUT /me/avatar (me.avatar): jpeg is the square the screen
	// cut, at most 2 MB (03 section 6). The implementation puts the picture in the
	// image cache under the URL it answers, so that the screens that draw
	// Services.Avatar(url) show it at once.
	UploadAvatar(ctx context.Context, jpeg []byte) (model.Avatar, error)
	// PickPhoto asks the system for one picture and returns it encoded (JPEG or
	// PNG), its orientation applied and its shorter side reduced to what the
	// cropper needs (Q-P6-10); the implementation shows the system's picker on the
	// UI thread and waits for it, so a screen calls it from ui.Go. "Chụp ảnh mới"
	// and "Chọn từ thư viện" both open the library (K7, Q-P6-11). The errors: the
	// user leaving the picker is ErrPhotoCanceled; a platform with no picker is
	// core.ErrUnsupported and a picker already shown core.ErrDuplicate (both
	// silent, PhotoSilent); a file that cannot be read is any other error
	// ("Không đọc được ảnh này").
	PickPhoto(ctx context.Context) ([]byte, error)
	// Devices is GET /me/devices (devices.list).
	Devices(ctx context.Context) (model.DeviceList, error)
	// DeleteDevice is DELETE /me/devices/{id} (devices.delete). A device that is
	// not there (404) is an error here; Env.Account turns it into success.
	DeleteDevice(ctx context.Context, id string) error
	// PairConfirm is POST /tv/pair/confirm (tv.pair.confirm) by pairing.Phone: it
	// upper-cases the input and drops dashes and spaces, and answers core.ErrInvalid
	// for what is not a six-character code, sending nothing. A code the server
	// does not know is "pair_code_invalid" (404), one that ran out
	// "pair_code_expired" (410), a TV over the limit "device_limit" (409, with
	// Body.Devices).
	PairConfirm(ctx context.Context, code string) (model.PairConfirm, error)
	// Logout ends the session: the position the player was saving is waited for,
	// the queue of positions is sent (5 s at most), the server is told (auth.logout)
	// and the vault, the cache and the queue are cleared; the downloaded files stay
	// (Q-P5-8). It blocks, so it runs off the UI thread; the Shell has already
	// stopped the sound and called Playback.Leave on the UI thread, before it, so
	// that the last position reaches the queue first. A Login started while it is
	// under way waits for it. A user who asked to sign out is signed out whatever
	// the error: it is for the log.
	Logout(ctx context.Context) error
	// BeginLogout marks a sign-out under way, on the calling thread and without
	// blocking, and returns the sign-out itself (what Logout does, with its
	// context): the Shell calls BeginLogout on the UI thread before it shows the
	// login and runs the returned function off it, so that a Login started in
	// between already waits. The returned function must be called, once.
	BeginLogout() (logout func(ctx context.Context) error)
}

// AutoNextPrefs is the optional part of Prefs that can set "Tự phát bài tiếp
// theo" (`pref:auto`, 15). Prefs that lack it keep the setting in memory (Env).
type AutoNextPrefs interface {
	SetAutoNext(on bool)
}

// AccountState is what Env keeps of the account between two builds of a screen
// (Back rebuilds, nav.go): what GET /me, GET /me/coach-profile and GET
// /me/devices answered last, with what the writes changed, so that a screen draws
// at once and refreshes. The Has flags say which of them was read since the
// session began.
type AccountState struct {
	Me      model.Me
	Coach   model.CoachProfile
	Devices model.DeviceList

	HasMe, HasCoach, HasDevices bool
}

// DevicesSheetParams is the Params of the sheet of devices (SheetDevices): a TV
// code that the server refused for the limit of devices, or a sign-in that it
// refused for the same reason (Login set; K31).
type DevicesSheetParams struct {
	// Code is the code the user typed, as it was sent; the sheet confirms it again
	// once a device is gone. Empty for the sheet of a sign-in.
	Code string
	// Devices are the devices of the answer (Body.Devices of "device_limit").
	Devices []model.Device
	// Login, when set, makes the sheet the sign-in's (K31): no device is chosen at
	// first, and the red button signs in again with the chosen device to be replaced
	// instead of deleting it and confirming Code.
	Login *LoginReplace
}

// LoginReplace is what the sheet of devices needs of the sign-in it was opened
// by: the request to send again and where its outcome goes. The email and the
// password are inside Retry, held by the login screen for this one retry and never
// kept elsewhere.
type LoginReplace struct {
	// Retry signs in again with the device (Services.LoginReplacing) and the
	// email and password of the form. It blocks: the sheet calls it off the UI
	// thread.
	Retry func(ctx context.Context, deviceID string) error
	// Done is called on the UI thread once the sheet is closed, with the outcome that
	// belongs to the form: nil is the sign-in (the screen goes on, as after any
	// sign-in), another error is the form's to show (a wrong password, a locked
	// account, the server in trouble).
	Done func(err error)
	// Cancel is called on the UI thread when the sheet is closed without an outcome
	// (the cross, "Huỷ", the backdrop, Back): the form stays as it was.
	Cancel func()
}

// FailureKind says what an error of the account means to the user.
type FailureKind uint8

// The kinds of failure. FailureUnknown is the zero value, what FailureOf(nil)
// is; FailureBusy is any other failure (5xx, 429, an answer that cannot be read).
const (
	FailureUnknown FailureKind = iota
	// FailureCanceled is a context that ended or a picker left: nothing is shown.
	FailureCanceled
	// FailureOffline is the network being down.
	FailureOffline
	// FailureValidation is me.update refusing fields (400 validation_failed).
	FailureValidation
	// FailureWrongPassword is me.password refusing the current one.
	FailureWrongPassword
	// FailureFileTooLarge is me.avatar refusing the size (413).
	FailureFileTooLarge
	// FailureInvalidSocial is me.coach.update refusing a link (400).
	FailureInvalidSocial
	// FailureCodeInvalid is a TV code the server does not know (404), or one that is
	// not six characters.
	FailureCodeInvalid
	// FailureCodeExpired is a TV code that ran out (410).
	FailureCodeExpired
	// FailureDeviceLimit is a TV that does not fit the limit of devices (409).
	FailureDeviceLimit
	// FailureBusy is any other failure.
	FailureBusy
)

// Failure is an error of the account, classified.
type Failure struct {
	// Kind says what it means.
	Kind FailureKind
	// Fields are the server's messages by field, the keys of the request ("first_name",
	// "email", "current_password", "socials[1].url"); empty when it named none.
	Fields map[string]string
	// Devices are the devices of FailureDeviceLimit.
	Devices []model.Device
}

// accountPort is Env.Account: the Services' AccountServices (or none), keeping
// what they answer.
type accountPort struct {
	svc AccountServices // nil for an Env with no Services

	mu    sync.Mutex
	state AccountState
	gen   uint64 // counts ResetAccount: an answer to a call begun before one is dropped
	// devWrites counts the writes to the kept device list (PairConfirm,
	// DeleteDevice): a read of the list begun before one is dropped, its answer
	// is older than the write.
	devWrites uint64
}

var _ AccountServices = (*accountPort)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Account returns the account of the services, never nil: for an Env with no
// Services every method fails with core.ErrUnsupported (SessionUser has none).
// What it answers is kept (AccountView), a device the server says is gone (404)
// is success, and the avatar, the profile and the devices that a write changed
// are changed in the kept view.
func (e *Env) Account() AccountServices {
	e.acctOnce.Do(func() {
		a := &accountPort{}
		if e.Services != nil {
			a.svc = e.Services
		}
		e.acct = a
	})

	return e.acct
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccountView returns a copy of what the account screens read last (the kept
// view), safe from any goroutine. It allocates the copy: read it when a screen is
// built and when an answer lands, never in a frame.
func (e *Env) AccountView() AccountState {
	a := e.Account().(*accountPort)
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.state.clone()
}

// ResetAccount forgets the kept view: the session ended, the next user's screens
// must not draw this one's; what a call begun before it answers later is dropped.
// The Shell calls it at every sign-out.
func (e *Env) ResetAccount() {
	a := e.Account().(*accountPort)
	a.mu.Lock()
	a.state = AccountState{}
	a.gen++
	a.mu.Unlock()
}

// AccountUser is the user the screens draw: the one GET /me answered, with what
// the writes changed, else the one of the session; false when neither is known.
func (e *Env) AccountUser() (model.User, bool) {
	v := e.AccountView()
	if v.HasMe {
		return v.Me.User, true
	}

	return e.Account().SessionUser()
}

// TVDevice is the Apple TV linked to the account: the first device of the list
// that is a tvOS one (the TV limit is 1). False while the list is unread or holds
// none: screen 15 says "Chưa kết nối" and 16 asks for a code.
func (e *Env) TVDevice() (model.Device, bool) {
	v := e.AccountView()
	if !v.HasDevices {
		return model.Device{}, false
	}
	for _, d := range v.Devices.Items {
		if d.Platform == model.PlatformTVOS {
			return d, true
		}
	}

	return model.Device{}, false
}

// SetAutoNext sets "Tự phát bài tiếp theo": the services' Prefs when they can,
// else the memory of Env.
func (e *Env) SetAutoNext(on bool) {
	if p, ok := e.Prefs().(AutoNextPrefs); ok {
		p.SetAutoNext(on)
	}
}

// FailureToast is the text a failure is shown as, in a toast: the strings of
// P6 section 5 ("Mã không đúng. Kiểm tra lại mã trên TV"...); "" for the failures
// that show nothing (canceled, the sheet of devices, unknown).
func (e *Env) FailureToast(f Failure) string {
	switch f.Kind {
	case FailureOffline:
		return e.T("toast.offline")
	case FailureValidation:
		return e.T("toast.infoInvalid")
	case FailureWrongPassword:
		return e.T("toast.pwWrong")
	case FailureFileTooLarge:
		return e.T("toast.photoTooLarge")
	case FailureInvalidSocial:
		return e.T("toast.badSocial")
	case FailureCodeInvalid:
		return e.T("toast.codeWrong")
	case FailureCodeExpired:
		return e.T("toast.codeExpired")
	case FailureBusy:
		return e.T("common.errorTitle")
	case FailureUnknown, FailureCanceled, FailureDeviceLimit:
		return ""
	default:
		return ""
	}
}

// PhotoToast is the toast of a failed PickPhoto: "" for a picker left, one that
// is not on this platform or one already shown (all silent, Q-P6-10), "Không đọc
// được ảnh này" for a picture that cannot be read. A canceled context is silent.
func (e *Env) PhotoToast(err error) string {
	if err == nil || PhotoSilent(err) {
		return ""
	}

	return e.T("toast.photoUnreadable")
}

// SessionUser is the session's user.
func (a *accountPort) SessionUser() (model.User, bool) {
	if a.svc == nil {
		return model.User{}, false
	}

	return a.svc.SessionUser()
}

// Me is GET /me, kept.
func (a *accountPort) Me(ctx context.Context) (model.Me, error) {
	if a.svc == nil {
		return model.Me{}, unsupported("me")
	}
	gen := a.generation()
	m, err := a.svc.Me(ctx)
	if err == nil {
		a.keep(gen, func(s *AccountState) { s.Me, s.HasMe = m, true })
	}

	return m, err
}

// UpdateMe is PATCH /me; the user it answers replaces the kept one.
func (a *accountPort) UpdateMe(ctx context.Context, r model.MeUpdateRequest) (model.MeUpdate, error) {
	if a.svc == nil {
		return model.MeUpdate{}, unsupported("update me")
	}
	gen := a.generation()
	u, err := a.svc.UpdateMe(ctx, r)
	if err == nil {
		a.keep(gen, func(s *AccountState) { s.Me.User = u.User })
	}

	return u, err
}

// ChangePassword is PUT /me/password.
func (a *accountPort) ChangePassword(ctx context.Context, current, next string) (model.PasswordResult, error) {
	if a.svc == nil {
		return model.PasswordResult{}, unsupported("password")
	}

	return a.svc.ChangePassword(ctx, current, next)
}

// Coach is GET /me/coach-profile, kept.
func (a *accountPort) Coach(ctx context.Context) (model.CoachProfile, error) {
	if a.svc == nil {
		return model.CoachProfile{}, unsupported("coach")
	}
	gen := a.generation()
	c, err := a.svc.Coach(ctx)
	if err == nil {
		a.keep(gen, func(s *AccountState) { s.Coach, s.HasCoach = c, true })
	}

	return c, err
}

// UpdateCoach is PUT /me/coach-profile; the profile it answers replaces the kept
// one.
func (a *accountPort) UpdateCoach(ctx context.Context, r model.CoachUpdateRequest) (model.CoachProfile, error) {
	if a.svc == nil {
		return model.CoachProfile{}, unsupported("update coach")
	}
	gen := a.generation()
	c, err := a.svc.UpdateCoach(ctx, r)
	if err == nil {
		a.keep(gen, func(s *AccountState) { s.Coach, s.HasCoach = c, true })
	}

	return c, err
}

// UploadAvatar is PUT /me/avatar; the URL it answers is the kept user's.
func (a *accountPort) UploadAvatar(ctx context.Context, jpeg []byte) (model.Avatar, error) {
	if a.svc == nil {
		return model.Avatar{}, unsupported("avatar")
	}
	gen := a.generation()
	av, err := a.svc.UploadAvatar(ctx, jpeg)
	if err == nil {
		a.keep(gen, func(s *AccountState) { s.Me.User.AvatarURL = av.AvatarURL })
	}

	return av, err
}

// PickPhoto is the system's picker.
func (a *accountPort) PickPhoto(ctx context.Context) ([]byte, error) {
	if a.svc == nil {
		return nil, unsupported("pick photo")
	}

	return a.svc.PickPhoto(ctx)
}

// Devices is GET /me/devices, kept.
func (a *accountPort) Devices(ctx context.Context) (model.DeviceList, error) {
	if a.svc == nil {
		return model.DeviceList{}, unsupported("devices")
	}
	gen, writes := a.devicesMark()
	d, err := a.svc.Devices(ctx)
	// A read whose context ended (Connect cancels it before it confirms) or that
	// began before a write to the list answers what was true before the write:
	// the caller has the answer, the kept list does not.
	if err == nil && ctx.Err() == nil {
		a.keepDevices(gen, writes, func(s *AccountState) { s.Devices, s.HasDevices = d, true })
	}

	return d, err
}

// DeleteDevice removes the device; one the server no longer has (404) is gone all
// the same, which is what the user wanted, so it is success. Either way the kept
// list drops it.
func (a *accountPort) DeleteDevice(ctx context.Context, id string) error {
	if a.svc == nil {
		return unsupported("delete device")
	}
	gen := a.generation()
	err := a.svc.DeleteDevice(ctx, id)
	if err != nil && !isNotFound(err) {
		return err
	}
	a.keep(gen, func(s *AccountState) {
		a.devWrites++
		kept := make([]model.Device, 0, len(s.Devices.Items))
		for _, d := range s.Devices.Items {
			if d.ID != id {
				kept = append(kept, d)
			}
		}
		s.Devices.Items = kept
	})

	return nil
}

// PairConfirm confirms the TV code; the device it answers joins the kept list.
func (a *accountPort) PairConfirm(ctx context.Context, code string) (model.PairConfirm, error) {
	if a.svc == nil {
		return model.PairConfirm{}, unsupported("pair confirm")
	}
	gen := a.generation()
	res, err := a.svc.PairConfirm(ctx, code)
	if err != nil {
		return res, err
	}
	a.keep(gen, func(s *AccountState) {
		a.devWrites++
		for _, d := range s.Devices.Items {
			if d.ID == res.Device.ID {
				return
			}
		}
		items := make([]model.Device, 0, len(s.Devices.Items)+1)
		items = append(items, s.Devices.Items...)
		s.Devices.Items = append(items, model.Device{ID: res.Device.ID, Platform: res.Device.Platform, Name: res.Device.Name})
	})

	return res, nil
}

// Logout ends the session. The kept view is not touched here: the Shell calls
// ResetAccount on the UI thread before it starts, so that a screen built
// meanwhile (the login's next user) never draws this user.
func (a *accountPort) Logout(ctx context.Context) error {
	if a.svc == nil {
		return unsupported("logout")
	}

	return a.svc.Logout(ctx)
}

// BeginLogout marks the sign-out under way (AccountServices.BeginLogout); with no
// Services the returned function answers core.ErrUnsupported.
func (a *accountPort) BeginLogout() func(ctx context.Context) error {
	if a.svc == nil {
		return func(context.Context) error { return unsupported("logout") }
	}

	return a.svc.BeginLogout()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// FailureOf classifies the error of a call of the account (P6 section 5). A nil
// error is FailureUnknown. A 426 is not a kind: ask Env.Refused first.
func FailureOf(err error) Failure {
	var f Failure
	if err == nil {
		return f
	}
	var ae *api.Error
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, ErrPhotoCanceled):
		f.Kind = FailureCanceled
	case Offline(err):
		f.Kind = FailureOffline
	case errors.As(err, &ae):
		f = failureOfAPI(ae)
	case errors.Is(err, core.ErrInvalid):
		// pairing.Phone.Confirm refusing what is not a code
		f.Kind = FailureCodeInvalid
	default:
		f.Kind = FailureBusy
	}

	return f
}

// failureOfAPI classifies an answer of the API by its code, then its status.
func failureOfAPI(ae *api.Error) Failure {
	f := Failure{Kind: FailureBusy, Fields: ae.Body.Fields}
	switch ae.Code {
	case "validation_failed":
		f.Kind = FailureValidation
	case "wrong_password":
		f.Kind = FailureWrongPassword
	case "file_too_large":
		f.Kind = FailureFileTooLarge
	case "invalid_social":
		f.Kind = FailureInvalidSocial
	case "pair_code_invalid":
		f.Kind = FailureCodeInvalid
	case "pair_code_expired":
		f.Kind = FailureCodeExpired
	case "device_limit":
		f.Kind, f.Devices = FailureDeviceLimit, ae.Body.Devices
	default:
		switch ae.Status {
		case http.StatusRequestEntityTooLarge:
			f.Kind = FailureFileTooLarge
		case http.StatusGone:
			f.Kind = FailureCodeExpired
		default:
		}
	}

	return f
}

// ReplaceableDevices returns the devices of a device_limit answer that a sign-in
// on a phone can replace (K31): the phones, not the TVs (the server only revokes a
// device of the same kind), the most recently seen first; one the server gave no
// day for goes last, and the order of the answer holds among equals.
func ReplaceableDevices(list []model.Device) []model.Device {
	out := make([]model.Device, 0, len(list))
	for _, d := range list {
		if d.Platform != model.PlatformTVOS && d.ID != "" {
			out = append(out, d)
		}
	}
	slices.SortStableFunc(out, func(a, b model.Device) int { return b.LastSeenAt.Compare(a.LastSeenAt) })

	return out
}

// PhotoSilent tells whether err, from PickPhoto, shows nothing: the picker was
// left, the platform has none, one is already shown, or the context ended.
func PhotoSilent(err error) bool {
	return errors.Is(err, ErrPhotoCanceled) || errors.Is(err, context.Canceled) ||
		errors.Is(err, core.ErrUnsupported) || errors.Is(err, core.ErrDuplicate)
}

// isNotFound tells whether err is the server's 404.
func isNotFound(err error) bool {
	var ae *api.Error

	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// unsupported is the failure of a call to services that have no account.
func unsupported(what string) error {
	return fmt.Errorf("env: account %s: %w", what, core.ErrUnsupported)
}

// generation returns the count of resets, taken before a call.
func (a *accountPort) generation() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.gen
}

// keep applies fn to the kept view, unless the view was reset since gen was
// taken: the answer is the session before's.
func (a *accountPort) keep(gen uint64, fn func(*AccountState)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gen == gen {
		fn(&a.state)
	}
}

// devicesMark returns the generation and the count of writes to the device list,
// taken before a read of it.
func (a *accountPort) devicesMark() (gen, writes uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.gen, a.devWrites
}

// keepDevices applies fn to the kept view, unless it was reset since gen was
// taken or the device list was written since writes was.
func (a *accountPort) keepDevices(gen, writes uint64, fn func(*AccountState)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gen == gen && a.devWrites == writes {
		fn(&a.state)
	}
}

// clone copies the lists, so that what a screen holds is not changed under it.
func (s AccountState) clone() AccountState {
	s.Me.Badges = append([]model.Badge(nil), s.Me.Badges...)
	s.Me.Subjects = append([]string(nil), s.Me.Subjects...)
	s.Coach.Subjects = append([]string(nil), s.Coach.Subjects...)
	s.Coach.Socials = append([]model.Social(nil), s.Coach.Socials...)
	s.Coach.AllowedPlatforms = append([]string(nil), s.Coach.AllowedPlatforms...)
	s.Coach.Places = append([]string(nil), s.Coach.Places...)
	s.Devices.Items = append([]model.Device(nil), s.Devices.Items...)

	return s
}
