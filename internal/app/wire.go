package app

import (
	"fmt"
	"time"

	"dancefitvn-app/internal/api"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// appVersion is gdesk.json's version, sent as X-App-Version
// (TestAppVersionIsGdeskJSONs keeps them equal) and shown in the footers
// of screen 15 and TV12 (K2).
const appVersion = "1.0.1"

// The files of the app's folders, on the phone and on the TV.
const (
	// deviceKey is the document of what the device remembers: the email
	// of the last sign-in (the phone's login prefill, Q-P3-10; the TV's
	// last pairing, overwritten at its sign-out).
	deviceKey = "device"
	// apiFolder, under the cache folder, is the API's ETag cache.
	apiFolder = "api"
)

// maxImageBytes is the largest picture Avatar reads: an avatar is far
// smaller.
const maxImageBytes = 8 << 20

// coursesPage is the page size asked of GET /me/courses.
const coursesPage = 50

// flushTimeout bounds one flush of the offline progress queue.
const flushTimeout = 30 * time.Second

// artTimeout bounds the reading of a cover for Now Playing.
const artTimeout = 15 * time.Second

// logoutBudget bounds what a sign-out the user asked for waits for before
// the session ends: the save of the position Leave started, then the
// sending of the queue (03 section 3: 5 s at most). A slow network does not
// hold the sign-out longer.
const logoutBudget = 5 * time.Second

// logoutServerTimeout bounds the telling of the server (auth.logout,
// tv.logout): the session ends here whatever it answers, and a Login waits
// for it.
const logoutServerTimeout = 10 * time.Second

// errProbeCached is a probe answered from the cache: no server reached.
var errProbeCached = fmt.Errorf("app: probe: answered from the cache: %w", api.ErrOffline)

// errWireClosed is a call after the window closed.
var errWireClosed = fmt.Errorf("app: services closed: %w", core.ErrClosed)

// errSaveSignedOut is a save of the position with no session to send it
// with.
var errSaveSignedOut = fmt.Errorf("app: progress: no session: %w", api.ErrSignedOut)

// errNoMock is a developer's state asking the mock in the process for a
// scenario (or its overlay) while the app talks to a server: DFV_API, a
// route of DFV_LIVE_ROUTES, or a live build, which has no mock.
var errNoMock = fmt.Errorf("app: no mock in the process (live API): %w", core.ErrUnsupported)

// watch is a callback of WatchOnline, OnSignedOut or WatchForeground.
type watch struct {
	online func(bool)
	out    func(error)
	front  func()
}

// deviceDoc is the document deviceKey.
type deviceDoc struct {
	LastEmail string `json:"last_email"`
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setScenario names the scenario of a route of m, the mock in the process;
// errNoMock without one (the app talks to a server) or for a route a
// hybrid sends live.
func setScenario(m inProcMock, route, name string) error {
	if m == nil {
		return errNoMock
	}
	if err := m.SetScenario(route, name); err != nil {
		return fmt.Errorf("app: scenario %s of %s: %w", name, route, err)
	}

	return nil
}
