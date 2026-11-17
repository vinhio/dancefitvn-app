//go:build !tvos

package env

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/mobile/nav"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Env is what a screen is built with: the services of the app, the texts, the
// actions and the navigator. The Shell makes one and passes it to the factory of
// every route.
type Env struct {
	Services
	// Act is the Shell's actions.
	Act Actions
	// Nav is the navigator, for the screens that read it (the toast, the sheet).
	Nav *nav.Nav
	// CoursesView is what the Courses screen keeps when it is left and built again:
	// the prototype's `S.m.filter` and `S.m.q` are state of the app, not of the
	// screen, so that a card that opens a course and Back return to the list as it
	// was left.
	CoursesView CoursesState

	texts  map[string]string
	cfg    model.AppConfig
	hasCfg bool
	prefs  Prefs // the in-memory preferences of a Services that has none

	nopDl   *nopManager // the DownloadManager of services whose Downloads is nil
	wifiOff bool        // "Chỉ tải qua Wi‑Fi" turned off, when the Prefs keep no such value

	mu      sync.Mutex
	courses map[string]model.Course // the courses Course answered, by id

	acctOnce sync.Once
	acct     *accountPort // Account's port, made at its first call
}

// CoursesState is the filter and the search text of the Courses screen: Filter is
// the index of the button on (0, all, is the zero value) and Query the text of the
// field, as typed.
type CoursesState struct {
	Filter int
	Query  string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns an Env over the services and the texts of locales/vi.json.
func New(s Services, texts map[string]string) *Env {
	return &Env{Services: s, texts: texts}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// T returns the text of key; a key the locale lacks shows as the key itself, where
// a missing string cannot be overlooked.
func (e *Env) T(key string) string {
	if s, ok := e.texts[key]; ok {
		return s
	}

	return key
}

// Fill returns the template of key with each {name} replaced by the value that
// follows it in kv (name, value, name, value ...).
func (e *Env) Fill(key string, kv ...string) string { return Fill(e.T(key), kv...) }

// SetConfig keeps the app config the boot fetched.
func (e *Env) SetConfig(c model.AppConfig) { e.cfg, e.hasCfg = c, true }

// Cfg returns the app config the boot fetched, false while there is none (the
// network was down at launch).
func (e *Env) Cfg() (model.AppConfig, bool) { return e.cfg, e.hasCfg }

// Course is GET /courses/{id} and remembers the answer: the shell builds the
// lessons of the player from the course a screen showed (CachedCourse) without
// asking the server again. The screens call it, not the Services' own, which it
// hides.
func (e *Env) Course(ctx context.Context, id string) (model.Course, error) {
	c, err := e.Services.Course(ctx, id)
	if err == nil {
		e.mu.Lock()
		if e.courses == nil {
			e.courses = map[string]model.Course{}
		}
		e.courses[id] = c
		e.mu.Unlock()
	}

	return c, err
}

// CachedCourse returns the course id as Course answered last, false when it
// has not been read in this run.
func (e *Env) CachedCourse(id string) (model.Course, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.courses[id]

	return c, ok
}

// Refused is the one place a screen's error path asks first (03 section 9): when
// err is the server refusing this version (HTTP 426, code "upgrade_required") it
// replaces the whole stack with the blocking upgrade screen, the store pages of
// the error's body in its params, and returns true: the screen shows nothing
// of its own. A canceled read or any other error returns false. The sign-out of a
// refused session is not here: services run auth.Manager.Handle(err) already.
func (e *Env) Refused(err error) bool {
	p, ok := UpgradeOf(err)
	if !ok {
		return false
	}
	if e.Nav != nil && e.Nav.Current().Screen == RouteUpgrade {
		return true
	}
	if e.Act != nil {
		e.Act.Go(nav.Route{Screen: RouteUpgrade, Params: p}, true)
	}

	return true
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// UpgradeOf tells whether err is a 426 upgrade_required answer, wrapped or not,
// and returns the params of the upgrade route made from its body.
func UpgradeOf(err error) (UpgradeParams, bool) {
	var ae *api.Error
	if !errors.As(err, &ae) || (ae.Code != "upgrade_required" && ae.Status != http.StatusUpgradeRequired) {
		return UpgradeParams{}, false
	}

	return UpgradeParams{StoreURL: ae.Body.StoreURL}, true
}

// Fill replaces each "{name}" of tpl by the value after name in kv. A value that is
// empty takes its " · " part of the line with it (format.DropBlank).
func Fill(tpl string, kv ...string) string {
	if len(kv) < 2 || !strings.Contains(tpl, "{") {
		return tpl
	}
	pairs := make([]string, 0, len(kv))
	empty := false
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, "{"+kv[i]+"}", kv[i+1])
		empty = empty || kv[i+1] == ""
	}
	out := strings.NewReplacer(pairs...).Replace(tpl)
	if empty { // a lesson with no tag: no stray " · " where its part would be
		out = format.DropBlank(out)
	}

	return out
}
