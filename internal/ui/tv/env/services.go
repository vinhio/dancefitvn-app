//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/env/{services,playback}.go
// (the pattern: interfaces the screens read the app through, which import none
// of the app's packages), cut to what the TV screens use (docs/plan/P7 section
// 1.1).

package env

import (
	"context"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Services is what the TV screens ask of the app. The methods that take a
// context block (the network, the vault): call them from a ui.Go work
// function, never from the UI thread. The others are cheap and for the UI
// thread, and the callbacks they take are delivered on it.
//
// Every error path runs auth.Manager.Handle(err) inside the implementation, never
// in a screen. A context that was canceled (the screen was left) ends with
// context.Canceled, which no screen shows.
type Services interface {
	// Restore reads the session kept in the vault: true when there is one.
	// Blocks.
	Restore(ctx context.Context) (signed bool, err error)
	// SessionUser is the signed-in user, false when signed out. Cheap.
	SessionUser() (model.User, bool)
	// OnSignedOut calls fn on the UI thread when the session ends without the
	// user asking (the refresh token was refused); reason wraps
	// api.ErrSignedOut.
	OnSignedOut(fn func(reason error)) (stop func())
	// Pair starts the pairing of this TV (POST /tv/pair/start, then a poll every
	// poll_interval_sec): the hooks are called on the UI thread. Stop the
	// returned Pairing when TV02 is left.
	Pair(h PairHooks) Pairing
	// LogoutTV is POST /tv/logout, then the local sign-out; it keeps nothing
	// else. Blocks.
	LogoutTV(ctx context.Context) error
	// Home is GET /me/home. Blocks.
	Home(ctx context.Context) (model.Home, error)
	// Courses is GET /me/courses with every page merged. Blocks.
	Courses(ctx context.Context) (model.CoursePage, error)
	// Course is GET /courses/{id}. Blocks.
	Course(ctx context.Context, id string) (model.Course, error)
	// Me is GET /me. Blocks.
	Me(ctx context.Context) (model.Me, error)
	// Config is GET /app/config (support.hotline). Blocks.
	Config(ctx context.Context) (model.AppConfig, error)
	// Avatar loads the picture at url. Blocks.
	Avatar(ctx context.Context, url string) (*scene.Image, error)
	// Cover loads the cover of a course at url (K14): the real picture in the live
	// mode, reduced to the size it is drawn at, kept on the disk; an error
	// (core.ErrUnsupported for the mock's) leaves the vector poster. Blocks.
	Cover(ctx context.Context, url string) (*scene.Image, error)
	// WatchedLocal tells whether the lesson was watched in this run, which a
	// course read earlier does not know yet. Cheap.
	WatchedLocal(lessonID string) bool
	// Online tells whether the app thinks it is online (03 section 2).
	Online() bool
	// WatchOnline calls fn on the UI thread when Online changes; stop ends it.
	WatchOnline(fn func(online bool)) (stop func())
	// Playback is the one player of the process; nil for services that have
	// none. Cheap.
	Playback() Playback
	// Prefs are the preferences of the user; nil gives the defaults kept in
	// memory (Env.Prefs). Cheap.
	Prefs() Prefs
	// Version is the app's version, "0.1.0" (the footer of TV12, K2).
	Version() string
	// Now is the time of the device's clock (pinned in a shot state).
	Now() time.Time
}

// PairHooks are the calls a Pairing makes, on the UI thread.
type PairHooks struct {
	// OnState is a new code to show (the first, or after an expiry), with the
	// QR address and the time it expires.
	OnState func(pairing.State)
	// OnSignedIn is the code being confirmed on the phone: the session is kept
	// and email is the account's.
	OnSignedIn func(email string)
	// OnError is the pairing failing in a way it does not retry by itself.
	OnError func(error)
}

// Pairing is the pairing under way: stop it when TV02 is left.
type Pairing interface {
	// Stop ends the polls.
	Stop()
	// Remaining is how long the code shown has left; 0 without one.
	Remaining() time.Duration
	// Current is the code shown, false before the first.
	Current() (pairing.State, bool)
}

// Playback is the player as the TV screens drive it: the subset of
// *player.Player they use, plus the plane the video is drawn on. Every method
// is for the UI thread. A screen reads the state when it is built and again on
// each Change of Changes.
type Playback interface {
	// PlayVideo and PlayAudio start the lesson of a course; at is where the
	// audio starts (the video's position, Q-P7-10).
	PlayVideo(l player.Lesson, list []player.Lesson)
	PlayAudio(l player.Lesson, list []player.Lesson, at float64)
	// Play, Pause and Toggle are the play button; Toggle is the Play/Pause key.
	Play()
	Pause()
	Toggle()
	// SetRate, SetMirror, SetLoop and SetBeat are the options of the panel;
	// SetLoop(true) seeks to A.
	SetRate(r float64)
	SetMirror(on bool)
	SetLoop(on bool)
	SetBeat(on bool)
	// Prev, Next and Leave: the lesson before and after, at 0; the player
	// being left.
	Prev()
	Next()
	Leave()

	// Lesson, Kind, State and Err: the lesson loaded, video or audio, where the
	// load stands, why it failed.
	Lesson() player.Lesson
	Kind() player.Kind
	State() player.State
	Err() error
	// Playing says the media should be playing (the button shows pause).
	Playing() bool
	// Pos and Dur are the position and length in seconds; Rate the speed.
	Pos() float64
	Dur() float64
	Rate() float64
	// Mirror, Loop and Beat are the options of the panel; A and B the loop's
	// ends in seconds; BeatIndex the lit cell of the beat bar.
	Mirror() bool
	Loop() bool
	Beat() bool
	A() float64
	B() float64
	BeatIndex() int
	// HasPrev and HasNext tell whether the course has a lesson before and after
	// the one loaded.
	HasPrev() bool
	HasNext() bool
	// Changes tells what changed, on the UI thread.
	Changes() *event.Topic[player.Change]
	// Place puts the video's plane at r in the window; an empty r hides it.
	Place(r geom.Rect)
	// PictureReady tells that the first frame of the video is on the plane: the
	// stage of K5 goes away.
	PictureReady() bool
}

// Prefs are the preferences the TV screens read (TV12).
type Prefs interface {
	// AutoNext is "Tự phát bài tiếp theo" (`S.prefs.auto`).
	AutoNext() bool
	// SetAutoNext turns it on or off.
	SetAutoNext(on bool)
	// Quality is "Chất lượng video" (`S.prefs.quality`).
	Quality() store.Quality
	// CycleQualityTV is `quality` of tset (index.html:743): Tự động, 1080p,
	// 720p, Tự động; a stored 480p goes to Tự động.
	CycleQualityTV()
}
