// Package mock is the mock API of DanceFITVN: a Transport that answers from mock/dancefitvn-mock-api by the six rules of docs/plan/05-data.md §4, and the loopback server of its media.
package mock

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Files that rules 5 and 1 answer with.
const (
	tokenExpiredFile = "errors/token_expired.json"
	notFoundFile     = "errors/not_found.json"
	idToken          = "{{path.id}}"
	// deviceLimitScenario is the scenario of auth.login that a login with
	// replace_device_id answers with the default instead.
	deviceLimitScenario = "device_limit"
)

// Options configure a Transport. The zero value is the behaviour of the app.
type Options struct {
	// Wait waits for d, or until ctx ends, and reports the end of ctx. Nil
	// is a real timer. Tests pass an instant one (rule 4).
	Wait func(ctx context.Context, d time.Duration) error
	// KeepAudioDigest leaves size_bytes and sha256 of lessons.audio as the
	// mock file has them. By default they are the values of the file the
	// media server serves, so that the download check passes.
	KeepAudioDigest bool
	// NoETag turns off ETag and 304.
	NoETag bool
	// Logf logs a line; nil is silent. It never gets tokens or bodies.
	Logf func(format string, args ...any)
}

// Transport is an api.Transport that answers from the mock folder. It keeps
// no state of the API (a PUT does not change what a later GET reads), only
// the scenarios chosen. Safe for concurrent use.
type Transport struct {
	fsys  fs.FS
	table *table
	opts  Options
	media *Server

	mu        sync.RWMutex
	scenarios map[string]string

	digestMu sync.Mutex
	digests  map[string]digest
}

var _ api.Transport = (*Transport)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns a Transport over fsys, the root of the mock folder
// (routes.json, errors/, media/ ...): os.DirFS in tests, the embedded copy
// in the app.
func New(fsys fs.FS, opts Options) (*Transport, error) {
	tab, err := loadTable(fsys)
	if err != nil {
		return nil, err
	}
	files, err := fs.Sub(fsys, "media")
	if err != nil {
		return nil, fmt.Errorf("mock: media folder: %w", err)
	}
	t := &Transport{
		fsys:      fsys,
		table:     tab,
		opts:      opts,
		media:     NewServer(files, opts.Logf),
		scenarios: map[string]string{},
		digests:   map[string]digest{},
	}
	if t.opts.Wait == nil {
		t.opts.Wait = timerWait
	}

	return t, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do answers a request by the six rules: it waits the delay (4), finds the
// route (1), refuses a missing token (5), picks the scenario (2), reads its
// file with the id put in (3) and points the media at the loopback server
// (6). The error is only for a canceled ctx, a dead media server or an
// unreadable mock folder.
func (t *Transport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d := time.Duration(t.table.DefaultDelayMs) * time.Millisecond
	if err := t.opts.Wait(ctx, d); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	status, file, id, routeID := t.pick(req)
	if file == "" {
		return &api.Response{Status: status, Header: http.Header{}}, nil
	}
	body, err := fs.ReadFile(t.fsys, strings.ReplaceAll(file, idToken, id))
	if errors.Is(err, fs.ErrNotExist) {
		status = http.StatusNotFound
		body, err = fs.ReadFile(t.fsys, notFoundFile)
		routeID = ""
	}
	if err != nil {
		return nil, fmt.Errorf("mock: read %s: %w", file, err)
	}
	body = bytes.ReplaceAll(body, []byte(idToken), []byte(id))
	if routeID == "lessons.audio" && status == http.StatusOK && !t.opts.KeepAudioDigest {
		if body, err = t.patchAudio(body); err != nil {
			return nil, err
		}
	}
	if body, err = t.rewriteMedia(body); err != nil {
		return nil, err
	}

	return t.finish(req, status, body), nil
}

// Close stops the media server. Safe to call twice.
func (t *Transport) Close() error { return t.media.Close() }

// MediaServer returns the loopback server of the media.
func (t *Transport) MediaServer() *Server { return t.media }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pick applies rules 1, 5 and 2: the status, the file name of the body ("" for
// none), the {id} and the route id ("" when no route matched).
func (t *Transport) pick(req *api.Request) (status int, file, id, routeID string) {
	r, id, ok := t.table.match(req.Method, req.Path)
	if !ok {
		return http.StatusNotFound, notFoundFile, "", ""
	}
	if r.Auth && !hasBearer(req.Header) {
		return http.StatusUnauthorized, tokenExpiredFile, id, ""
	}
	name := t.scenarioOf(r.ID)
	if r.ID == "auth.login" && name == deviceLimitScenario && replacesDevice(req.Body) {
		name = defaultScenario // the device is revoked and the login completes (K31)
	}
	sc := r.Responses[name]
	if sc.File == nil {
		return sc.Status, "", id, r.ID
	}

	return sc.Status, *sc.File, id, r.ID
}

// replacesDevice reports whether the body of an auth.login carries a non-empty
// replace_device_id: the server revokes that device after the password and
// completes the login, so the mock's device_limit scenario gives way to the
// default answer. The password is not checked, as the mock never does.
func replacesDevice(body []byte) bool {
	var in struct {
		ReplaceDeviceID string `json:"replace_device_id"`
	}

	return json.Unmarshal(body, &in) == nil && in.ReplaceDeviceID != ""
}

// hasBearer reports whether h carries "Authorization: Bearer <token>" with a
// token.
func hasBearer(h http.Header) bool {
	v := h.Get("Authorization")
	tok, ok := strings.CutPrefix(v, "Bearer ")

	return ok && strings.TrimSpace(tok) != ""
}

// finish makes the response: JSON headers, ETag and 304.
func (t *Transport) finish(req *api.Request, status int, body []byte) *api.Response {
	h := http.Header{}
	h.Set("Content-Type", "application/json; charset=utf-8")
	if !t.opts.NoETag && req.Method == http.MethodGet && status == http.StatusOK {
		tag := etagOf(body)
		h.Set("ETag", tag)
		if etagMatches(req.Header.Get("If-None-Match"), tag) {
			delete(h, "Content-Type")

			return &api.Response{Status: http.StatusNotModified, Header: h}
		}
	}

	return &api.Response{Status: status, Header: h, Body: body}
}

// timerWait is the default Wait: a real timer that ctx can cut short.
func timerWait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	tm := time.NewTimer(d)
	defer tm.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tm.C:
		return nil
	}
}

// errClosed is the error of a closed media server.
var errClosed = fmt.Errorf("mock: media server: %w", core.ErrClosed)
