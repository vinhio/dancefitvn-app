package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strings"
	"time"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// idToken is the template token of the mock files.
	idToken = "{{path.id}}"
	// defaultScenario is the scenario the plan checks.
	defaultScenario = "default"
	// avatarSize is the side of the generated avatar, in pixels.
	avatarSize = 512
	// jsonType is the content type of a JSON body.
	jsonType = "application/json"
)

// errConnection marks a failure to reach the server; the command exits 2.
var errConnection = errors.New("connection failed")

// options are the settings of one run.
type options struct {
	// Mock is the mock folder.
	Mock string
	// Base is the base URL of the server, "https://host/api/v1".
	Base string
	// Email and Password are the test account.
	Email, Password string
	// PasswordNew enables me.password (the new password).
	PasswordNew string
	// DeviceID is the device devices.delete removes.
	DeviceID string
	// Routes limits the report to these route ids; empty means all.
	Routes []string
	// Write allows the routes that change data.
	Write bool
	// Self serves the mock in-process and checks it against itself.
	Self bool
	// Verbose logs one line per call.
	Verbose bool
	// Platform is the X-Platform of the requests: ios, android or tvos.
	Platform string
	// AppVersion is the X-App-Version of the requests.
	AppVersion string
	// InstallID is the X-Install-Id of the requests.
	InstallID string
}

// tvRoutes are the routes the Apple TV app itself calls; they carry tvos
// whatever -platform says, as the app does (the phone calls the others).
var tvRoutes = map[string]bool{"tv.pair.start": true, "tv.pair.poll": true}

// platformOf returns the X-Platform of a route.
func (o options) platformOf(id string) string {
	if tvRoutes[id] {
		return "tvos"
	}

	return o.Platform
}

// routeDef is one row of routes.json.
type routeDef struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Auth      bool                   `json:"auth"`
	Responses map[string]scenarioDef `json:"responses"`
}

// scenarioDef is one answer of a route.
type scenarioDef struct {
	Status int     `json:"status"`
	File   *string `json:"file"`
}

// state is what later steps take from earlier answers.
type state struct {
	installID string
	access    string
	refresh   string
	deviceID  string
	user      map[string]any
	coach     map[string]any
	courseID  string
	lessons   []any
	pairCode  string
	pairPoll  string
	deleteID  string
}

// checker runs the plan.
type checker struct {
	o      options
	fsys   fs.FS
	routes map[string]*routeDef
	order  []string
	tr     api.Transport
	rep    *report
	st     state
	// scenario selects the scenario of a route on the server; set only
	// with -self, where the checker owns the mock.
	scenario func(routeID, name string) error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newChecker loads routes.json of the mock folder.
func newChecker(o options, fsys fs.FS, tr api.Transport, rep *report) (*checker, error) {
	data, err := fs.ReadFile(fsys, "routes.json")
	if err != nil {
		return nil, fmt.Errorf("read routes.json: %w", err)
	}
	var file struct {
		Routes []routeDef `json:"routes"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode routes.json: %w", err)
	}
	c := &checker{o: o, fsys: fsys, routes: map[string]*routeDef{}, tr: tr, rep: rep}
	for i := range file.Routes {
		r := &file.Routes[i]
		c.routes[r.ID] = r
		c.order = append(c.order, r.ID)
	}
	for _, id := range o.Routes {
		if c.routes[id] == nil {
			return nil, fmt.Errorf("unknown route %q", id)
		}
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("install id: %w", err)
	}
	c.st.installID = "contractcheck-" + hex.EncodeToString(buf)
	for _, s := range []string{o.Email, o.Password, o.PasswordNew} {
		rep.addSecret(s)
	}

	return c, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// run executes the plan (05 §5, P1.8). The error is a failure of the
// transport (errConnection), never a contract error: those are in the
// report.
func (c *checker) run(ctx context.Context) error {
	steps := []string{
		"app.config", "auth.login",
		"me.get", "me.home", "me.courses", "courses.show", "lessons.stream", "lessons.audio",
		"me.coach.get", "devices.list", "tv.pair.start", "tv.pair.poll", "auth.refresh",
		"me.update", "me.avatar", "me.coach.update", "lessons.progress", "progress.batch",
		"tv.pair.confirm", "devices.delete", "auth.forgot", "me.password", "tv.logout",
	}
	for _, id := range steps {
		if err := c.exec(ctx, id); err != nil {
			return err
		}
	}
	if c.scenario != nil {
		if err := c.sweep(ctx); err != nil {
			return err
		}
	}

	// The login is undone whatever the flags are: a run must not leave a
	// device behind.
	return c.exec(ctx, "auth.logout")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// isWrite reports whether a route changes data on the server and so needs
// -write.
func isWrite(id string) bool {
	switch id {
	case "me.update", "me.password", "me.avatar", "me.coach.update", "lessons.progress", "progress.batch",
		"devices.delete", "tv.pair.confirm", "tv.logout", "auth.forgot":
		return true
	default:
		return false
	}
}

// selected reports whether the route is in the report: all when -route is
// not given.
func (c *checker) selected(id string) bool {
	return len(c.o.Routes) == 0 || slices.Contains(c.o.Routes, id)
}

// skip records a skipped call, when the route is in the report.
func (c *checker) skip(id, reason string) {
	if !c.selected(id) {
		return
	}
	r := c.routes[id]
	c.rep.add(result{Route: id, Method: r.Method, Path: r.Path, Scenario: defaultScenario, Skip: reason})
}

// exec runs one route of the plan in its default scenario.
func (c *checker) exec(ctx context.Context, id string) error {
	if isWrite(id) {
		switch {
		case !c.selected(id):
			return nil
		case !c.o.Write:
			c.skip(id, "needs -write")

			return nil
		}
	}
	pathID, body, ctype, reason := c.build(id)
	if reason != "" {
		c.skip(id, reason)

		return nil
	}
	resp, parsed, err := c.call(ctx, id, defaultScenario, pathID, body, ctype)
	if err != nil {
		return err
	}
	if resp.Status == c.routes[id].Responses[defaultScenario].Status {
		c.extract(id, parsed)
	}

	return nil
}

// sweep checks every non-default scenario of every route against the mock
// itself (-self only).
func (c *checker) sweep(ctx context.Context) error {
	for _, id := range c.order {
		r := c.routes[id]
		names := make([]string, 0, len(r.Responses))
		for n := range r.Responses {
			if n != defaultScenario {
				names = append(names, n)
			}
		}
		slices.Sort(names)
		if len(names) == 0 {
			continue
		}
		pathID, body, ctype, reason := c.build(id)
		if reason != "" {
			for _, n := range names {
				c.rep.add(result{Route: id, Method: r.Method, Path: r.Path, Scenario: n, Skip: reason})
			}

			continue
		}
		for _, n := range names {
			if err := c.scenario(id, n); err != nil {
				return fmt.Errorf("select scenario: %w", err)
			}
			_, _, err := c.call(ctx, id, n, pathID, body, ctype)
			if rerr := c.scenario(id, defaultScenario); rerr != nil && err == nil {
				err = fmt.Errorf("select scenario: %w", rerr)
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// build returns the path value, the body and its content type for a route,
// or the reason the route cannot run now.
func (c *checker) build(id string) (pathID string, body []byte, ctype, reason string) {
	r := c.routes[id]
	if r.Auth && c.st.access == "" {
		return "", nil, "", "no session (login failed)"
	}
	switch id {
	case "auth.login":
		b, ok := c.fileBody(id)
		if !ok {
			return "", nil, "", "no request file"
		}
		b["email"] = c.o.Email
		b["password"] = c.o.Password
		if dev, ok := b["device"].(map[string]any); ok {
			dev["install_id"] = c.st.installID
			dev["name"] = "contractcheck"
		}

		return "", mustJSON(b), jsonType, ""
	case "auth.refresh":
		if c.st.refresh == "" {
			return "", nil, "", "no refresh token from an earlier answer"
		}

		return "", mustJSON(map[string]any{"refresh_token": c.st.refresh, "install_id": c.st.installID}), jsonType, ""
	case "auth.forgot":
		return "", mustJSON(map[string]any{"email": c.o.Email}), jsonType, ""
	case "courses.show":
		if c.st.courseID == "" {
			return "", nil, "", "no course id from me.courses"
		}

		return c.st.courseID, nil, "", ""
	case "lessons.stream":
		return c.lessonRoute("has_video", "no lesson with video")
	case "lessons.audio":
		return c.lessonRoute("has_audio", "no lesson with audio")
	case "lessons.progress":
		l := c.lesson("has_video")
		if l == "" {
			return "", nil, "", "no lesson with video"
		}
		b, _ := c.fileBody(id)
		if b == nil {
			b = map[string]any{}
		}
		pos, dur := c.lessonTimes(l)
		b["position_sec"], b["duration_sec"] = pos, dur
		b["client_time"] = time.Now().Format(time.RFC3339)
		if c.st.deviceID != "" {
			b["device_id"] = c.st.deviceID
		}

		return l, mustJSON(b), jsonType, ""
	case "progress.batch":
		l := c.lesson("has_video")
		if l == "" {
			return "", nil, "", "no lesson with video"
		}
		pos, dur := c.lessonTimes(l)
		item := map[string]any{"lesson_id": l, "position_sec": pos, "duration_sec": dur, "client_time": time.Now().Format(time.RFC3339)}

		return "", mustJSON(map[string]any{"items": []any{item}}), jsonType, ""
	case "me.update":
		b, _ := c.fileBody(id)
		out := map[string]any{}
		for k := range b {
			if v := c.st.user[k]; v != nil {
				out[k] = v
			}
		}

		return "", mustJSON(out), jsonType, ""
	case "me.password":
		if c.o.PasswordNew == "" {
			return "", nil, "", "needs -password-new"
		}

		return "", mustJSON(map[string]any{"current_password": c.o.Password, "new_password": c.o.PasswordNew}), jsonType, ""
	case "me.avatar":
		return avatarBody()
	case "me.coach.update":
		if c.st.coach == nil {
			return "", nil, "", "no coach profile from me.coach.get"
		}
		socials := []any{}
		list, _ := c.st.coach["socials"].([]any)
		for _, s := range list {
			if m, ok := s.(map[string]any); ok {
				socials = append(socials, map[string]any{"url": m["url"]})
			}
		}
		out := map[string]any{"bio": c.st.coach["bio"], "places": c.st.coach["places"], "socials": socials}

		return "", mustJSON(out), jsonType, ""
	case "tv.pair.start":
		b, ok := c.fileBody(id)
		if !ok {
			return "", nil, "", "no request file"
		}
		if dev, ok := b["device"].(map[string]any); ok {
			dev["install_id"] = c.st.installID + "-tv"
			dev["name"] = "contractcheck tv"
		}

		return "", mustJSON(b), jsonType, ""
	case "tv.pair.poll":
		if c.st.pairCode == "" {
			return "", nil, "", "no pairing code from tv.pair.start"
		}

		return "", mustJSON(map[string]any{"code": c.st.pairCode, "poll_secret": c.st.pairPoll}), jsonType, ""
	case "tv.pair.confirm":
		if c.st.pairCode == "" {
			return "", nil, "", "no pairing code from tv.pair.start"
		}

		return "", mustJSON(map[string]any{"code": c.st.pairCode}), jsonType, ""
	case "devices.delete":
		if c.st.deleteID == "" {
			return "", nil, "", "needs -device"
		}

		return c.st.deleteID, nil, "", ""
	default:
		return "", nil, "", ""
	}
}

// lessonRoute builds a call on the first lesson with the flag.
func (c *checker) lessonRoute(flag, none string) (string, []byte, string, string) {
	l := c.lesson(flag)
	if l == "" {
		return "", nil, "", none
	}

	return l, nil, "", ""
}

// avatarBody returns a generated grey JPEG as a multipart body.
func avatarBody() (string, []byte, string, string) {
	const failed = "cannot build the avatar body"
	img := image.NewGray(image.Rect(0, 0, avatarSize, avatarSize))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		return "", nil, "", failed
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="avatar.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(h)
	if err != nil {
		return "", nil, "", failed
	}
	if _, err := part.Write(jpg.Bytes()); err != nil {
		return "", nil, "", failed
	}
	if err := mw.Close(); err != nil {
		return "", nil, "", failed
	}

	return "", buf.Bytes(), mw.FormDataContentType(), ""
}

// fileBody reads requests/<route>.json as an object.
func (c *checker) fileBody(id string) (map[string]any, bool) {
	data, err := fs.ReadFile(c.fsys, "requests/"+id+".json")
	if err != nil {
		return nil, false
	}
	v, err := decode(data)
	if err != nil {
		return nil, false
	}
	m, ok := v.(map[string]any)

	return m, ok
}

// lesson returns the id of the first lesson of the course whose flag is
// true, "" when none.
func (c *checker) lesson(flag string) string {
	for _, l := range c.st.lessons {
		m, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if b, _ := m[flag].(bool); b {
			s, _ := m["id"].(string)

			return s
		}
	}

	return ""
}

// lessonTimes returns the saved position and the duration of a lesson, so
// that a progress write leaves the account as it is.
func (c *checker) lessonTimes(id string) (pos, dur float64) {
	for _, l := range c.st.lessons {
		m, ok := l.(map[string]any)
		if !ok || m["id"] != id {
			continue
		}
		dur, _ = m["duration_sec"].(float64)
		if p, ok := m["progress"].(map[string]any); ok {
			pos, _ = p["position_sec"].(float64)
		}
	}

	return pos, dur
}

// extract takes from the default answer of a route what later steps need.
func (c *checker) extract(id string, v any) {
	switch id {
	case "auth.login":
		c.st.access, _ = dig(v, "access_token").(string)
		c.st.refresh, _ = dig(v, "refresh_token").(string)
		c.st.deviceID, _ = dig(v, "device", "id").(string)
		c.rep.addSecret(c.st.access)
		c.rep.addSecret(c.st.refresh)
	case "auth.refresh":
		if s, _ := dig(v, "access_token").(string); s != "" {
			c.st.access = s
			c.rep.addSecret(s)
		}
		if s, _ := dig(v, "refresh_token").(string); s != "" {
			c.st.refresh = s
			c.rep.addSecret(s)
		}
	case "me.get":
		c.st.user, _ = dig(v, "user").(map[string]any)
		if e, ok := c.st.user["email"].(string); ok {
			c.rep.addSecret(e)
		}
	case "me.courses":
		items, _ := dig(v, "items").([]any)
		if len(items) > 0 {
			c.st.courseID, _ = dig(items[0], "id").(string)
		}
	case "courses.show":
		c.st.lessons, _ = dig(v, "lessons").([]any)
	case "me.coach.get":
		c.st.coach, _ = v.(map[string]any)
	case "devices.list":
		c.pickDelete(v)
	case "tv.pair.start":
		c.st.pairCode, _ = dig(v, "code").(string)
		c.st.pairPoll, _ = dig(v, "poll_secret").(string)
		c.rep.addSecret(c.st.pairPoll)
	default:
	}
}

// pickDelete chooses the device devices.delete removes: the one of -device,
// or with -self the first that is not the current one.
func (c *checker) pickDelete(list any) {
	if c.o.DeviceID != "" {
		c.st.deleteID = c.o.DeviceID

		return
	}
	if !c.o.Self {
		return
	}
	items, _ := dig(list, "items").([]any)
	for _, it := range items {
		if cur, _ := dig(it, "current").(bool); !cur {
			c.st.deleteID, _ = dig(it, "id").(string)

			return
		}
	}
}

// dig walks object keys of a decoded JSON value; nil when a step is missing.
func dig(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}

	return v
}

// mustJSON encodes a value built in this file; it cannot fail for maps of
// strings, numbers and slices.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}

	return b
}

// call sends one request, records the result and returns the answer and its
// decoded body (nil when it is not JSON).
func (c *checker) call(ctx context.Context, id, scenario, pathID string, body []byte, ctype string) (*api.Response, any, error) {
	r := c.routes[id]
	path := r.Path
	if strings.Contains(path, "{id}") {
		path = strings.Replace(path, "{id}", url.PathEscape(pathID), 1)
	}
	req := &api.Request{RouteID: id, Method: r.Method, Path: path, Header: api.ClientHeader(c.o.AppVersion, c.o.platformOf(id), c.o.InstallID), Body: body}
	req.Header.Set("Accept", jsonType)
	if body != nil {
		req.Header.Set("Content-Type", ctype)
	}
	if r.Auth && c.st.access != "" {
		req.Header.Set("Authorization", "Bearer "+c.st.access)
	}
	resp, err := c.tr.Do(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w: %s", id, errConnection, c.rep.redact(err.Error()))
	}
	res := result{Route: id, Method: r.Method, Path: r.Path, Scenario: scenario, Status: resp.Status, Bytes: len(resp.Body)}
	var parsed any
	if len(resp.Body) > 0 {
		parsed, _ = decode(resp.Body)
	}
	res.Findings = c.evaluate(r, scenario, pathID, resp, parsed)
	// A call of a route that is not in the report appears only when it fails:
	// it is a prerequisite and its failure explains the skips after it.
	if c.selected(id) || res.verdict() != "PASS" {
		c.rep.add(res)
	}

	return resp, parsed, nil
}

// evaluate compares an answer with the mock scenario.
func (c *checker) evaluate(r *routeDef, scenario, pathID string, resp *api.Response, parsed any) []finding {
	var out []finding
	want := r.Responses[scenario]
	if resp.Status != want.Status {
		msg := fmt.Sprintf("status %d, want %d", resp.Status, want.Status)
		if code, ok := dig(parsed, "error", "code").(string); ok {
			msg += ", error code " + code
		}
		out = append(out, finding{Path: "$", Sev: sevError, Msg: msg})
		if resp.Status >= http.StatusBadRequest {
			out = append(out, c.errorShape(parsed, resp.Body)...)
		}

		return out
	}
	if want.File == nil {
		if len(resp.Body) > 0 {
			out = append(out, finding{Path: "$", Sev: sevWarning, Msg: "body on a status without one"})
		}

		return out
	}
	if len(resp.Body) == 0 {
		return append(out, finding{Path: "$", Sev: sevError, Msg: "empty body"})
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		out = append(out, finding{Path: "$", Sev: sevWarning, Msg: "content type is not json"})
	}
	file := strings.ReplaceAll(*want.File, idToken, pathID)
	data, err := fs.ReadFile(c.fsys, file)
	if err != nil && strings.Contains(*want.File, idToken) {
		data, err = c.fallbackFile(r)
	}
	if err != nil {
		return append(out, finding{Path: "$", Sev: sevError, Msg: "mock file unreadable: " + file})
	}

	out = append(out, compareBodies(data, resp.Body)...)
	if r.ID == "tv.pair.poll" && scenario == defaultScenario && dig(parsed, "status") != "pending" {
		out = append(out, finding{Path: "$.status", Sev: sevWarning, Msg: "a fresh pairing code should be pending"})
	}

	return out
}

// errorShape compares an error answer with errors/<code>.json.
func (c *checker) errorShape(parsed any, body []byte) []finding {
	code, _ := dig(parsed, "error", "code").(string)
	if code == "" {
		return []finding{{Path: "$.error.code", Sev: sevError, Msg: "error answer without error.code"}}
	}
	data, err := fs.ReadFile(c.fsys, "errors/"+code+".json")
	if err != nil {
		return []finding{{Path: "$.error.code", Sev: sevWarning, Msg: "error code " + code + " has no mock file"}}
	}

	return compareBodies(data, body)
}

// fallbackFile reads the mock file of the first course of the mock's own
// me.courses, for a course id the mock has no file of.
func (c *checker) fallbackFile(r *routeDef) ([]byte, error) {
	list := c.routes["me.courses"]
	if list == nil || list.Responses[defaultScenario].File == nil {
		return nil, fs.ErrNotExist
	}
	data, err := fs.ReadFile(c.fsys, *list.Responses[defaultScenario].File)
	if err != nil {
		return nil, fmt.Errorf("read mock courses: %w", err)
	}
	v, err := decode(data)
	if err != nil {
		return nil, err
	}
	items, _ := dig(v, "items").([]any)
	if len(items) == 0 {
		return nil, fs.ErrNotExist
	}
	first, _ := dig(items[0], "id").(string)

	return fs.ReadFile(c.fsys, strings.ReplaceAll(*r.Responses[defaultScenario].File, idToken, first))
}
