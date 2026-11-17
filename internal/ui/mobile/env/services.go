//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/port.go at 49de3c2 (the
// pattern: interfaces the screens read the app through, which import none of the
// app's packages, every method for the UI thread unless it says it blocks).

package env

import (
	"context"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"

	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Services is what the phone screens ask of the app. The methods that take a
// context block (the network, the vault): call them from a ui.Go work function,
// never from the UI thread. The others are cheap and for the UI thread, and the
// callbacks they take are delivered on it.
//
// Every error path runs auth.Manager.Handle(err) inside the implementation (a
// second 401 after a refresh signs out), never in a screen. A context that was
// canceled (the screen was left) ends with context.Canceled, which no screen
// shows (P1.11).
type Services interface {
	// Restore reads the session kept in the vault (auth.Manager.Restore): true
	// when there is one. Blocks.
	Restore(ctx context.Context) (signed bool, err error)
	// Config is GET /app/config. A 426 is an *api.Error with Code
	// "upgrade_required" and Body.StoreURL. Blocks.
	Config(ctx context.Context) (model.AppConfig, error)
	// Login is auth.login: it keeps the session, remembers the email and keeps
	// the biometric slot. The errors are the server's *api.Error (Code
	// invalid_credentials, account_locked, device_limit ...) or wrap
	// api.ErrOffline. Blocks.
	Login(ctx context.Context, email, password string) error
	// LoginReplacing is Login again after a device_limit (K31): replaceID is one
	// of the devices of that error, which the server revokes once the password is
	// verified before it completes the login. A 400 validation_failed with
	// Body.Fields["replace_device_id"] is the id not fitting. Blocks.
	LoginReplacing(ctx context.Context, email, password, replaceID string) error
	// BioReady is the biometry the sign-in offers (K13): the device has one
	// enrolled AND a slot of this app holds a refresh token; BiometryNone
	// otherwise. Cheap.
	BioReady() sys.BiometryKind
	// BioLogin asks the user to prove who they are with the system's sheet,
	// opens the slot, refreshes with its token and adopts the session. The
	// errors are those of sys.Authenticate (ErrBioCanceled, ErrBioFailed,
	// ErrBioLockout ...) or the refresh's. Blocks.
	BioLogin(ctx context.Context, p sys.BioPrompt) error
	// Home is GET /me/home. Blocks.
	Home(ctx context.Context) (model.Home, error)
	// Courses is GET /me/courses with every page merged. Blocks.
	Courses(ctx context.Context) (model.CoursePage, error)
	// Course is GET /courses/{id}. Blocks.
	Course(ctx context.Context, id string) (model.Course, error)
	// Downloads is the audio files of the device and the downloads under way,
	// as the screens read and drive them; nil for services that have none (Env
	// then gives a manager with nothing in it). Cheap.
	Downloads() DownloadManager
	// Online tells whether the app thinks it is online (03 section 2).
	Online() bool
	// WatchOnline calls fn on the UI thread when Online changes; stop ends it.
	WatchOnline(fn func(online bool)) (stop func())
	// OnSignedOut calls fn on the UI thread when the session ends without the
	// user asking (the refresh token was refused); reason wraps api.ErrSignedOut.
	OnSignedOut(fn func(reason error)) (stop func())
	// OpenURL opens raw in the system's browser; a failure is logged and
	// ignored.
	OpenURL(raw string)
	// LastEmail is the email last used to sign in on this device, "" for none.
	LastEmail() string
	// Avatar loads the picture at url. Blocks.
	Avatar(ctx context.Context, url string) (*scene.Image, error)
	// Cover loads the cover of a course at url (K14): the real picture in the live
	// mode, reduced to the size it is drawn at, kept on the disk; an error
	// (core.ErrUnsupported for the mock's) leaves the vector poster. Blocks.
	Cover(ctx context.Context, url string) (*scene.Image, error)
	// Now is the time of the device's clock (pinned in a shot state).
	Now() time.Time
	// Playback is the one player of the process; nil for services that have
	// none, which leaves the actions of playback doing nothing (as jPlayer's
	// Editor does). Cheap.
	Playback() Playback
	// WatchedLocal tells whether the lesson was watched in this run (played to
	// the share that makes it watched, or saved as watched by the server) which
	// a course read earlier does not know yet. False for services that have no
	// player. Cheap.
	WatchedLocal(lessonID string) bool
	// Prefs are the preferences of the user; nil gives the defaults kept in
	// memory (Env.Prefs). Cheap.
	Prefs() Prefs
	// AccountServices are the account's: the profile, the coach profile, the
	// avatar, the password, the devices, the TV code and the sign-out. The
	// screens and the Shell reach them through Env.Account, which keeps what they
	// answer (Env.AccountView), never through the Services' own.
	AccountServices
}

// Foregrounder is the optional part of Services that tells whether the app is
// in front of the user (active, not in the background). The Shell asks it before
// a toast that would only be stale when the user comes back ("Đã tải xong bài
// NN", Q-P5-4); services that do not implement it count as in front (Env.Foreground).
// The app's wire reports its lifecycle (host.App.OnLifecycle: Background is
// false, Active is true); the method is for the UI thread and cheap.
type Foregrounder interface {
	Foreground() bool
}

// ForegroundWatcher is the optional part of Services that tells when the app comes
// back to the front after it was in the background (host.App.OnLifecycle: Active
// after Background; the first Active of a launch is not a return). fn is called on
// the UI thread; stop ends the watch. The Shell tells the screen showing
// (ForegroundAware), so that a read that the system cut while the app was away is
// made again.
type ForegroundWatcher interface {
	WatchForeground(fn func()) (stop func())
}
