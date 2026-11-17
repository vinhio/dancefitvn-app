package mock

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
)

// row is one expected scenario: the contract written out by hand, so that a
// silent change of routes.json fails the test.
type row struct {
	route, scenario string
	status          int
	file            string // "" = no body
}

var expected = []row{
	{"auth.login", "default", 200, "auth/login.json"},
	{"auth.login", "invalid_credentials", 401, "errors/invalid_credentials.json"},
	{"auth.login", "account_locked", 423, "errors/account_locked.json"},
	{"auth.login", "device_limit", 409, "errors/device_limit.json"},
	{"auth.refresh", "default", 200, "auth/refresh.json"},
	{"auth.refresh", "refresh_invalid", 401, "errors/refresh_invalid.json"},
	{"auth.logout", "default", 204, ""},
	{"auth.forgot", "default", 200, "auth/forgot.json"},
	{"auth.forgot", "rate_limited", 429, "errors/rate_limited.json"},
	{"me.get", "default", 200, "me/me.json"},
	{"me.get", "token_expired", 401, "errors/token_expired.json"},
	{"me.update", "default", 200, "me/me.patch.json"},
	{"me.update", "validation_failed", 400, "errors/validation_failed.json"},
	{"me.password", "default", 200, "me/password.json"},
	{"me.password", "wrong_password", 400, "errors/wrong_password.json"},
	{"me.avatar", "default", 200, "me/avatar.json"},
	{"me.avatar", "file_too_large", 413, "errors/file_too_large.json"},
	{"me.coach.get", "default", 200, "me/coach-profile.json"},
	{"me.coach.update", "default", 200, "me/coach-profile.put.json"},
	{"me.coach.update", "invalid_social", 400, "errors/invalid_social.json"},
	{"me.home", "default", 200, "me/home.json"},
	{"me.home", "new_user", 200, "me/home.new_user.json"},
	{"me.home", "server_error", 500, "errors/server_error.json"},
	{"me.courses", "default", 200, "me/courses.json"},
	{"me.courses", "cat_special", 200, "me/courses.cat-special.json"},
	{"me.courses", "search_empty", 200, "me/courses.search-empty.json"},
	{"me.courses", "paged", 200, "me/courses.page1.json"},
	{"courses.show", "default", 200, "courses/{{path.id}}.json"},
	{"courses.show", "not_found", 404, "errors/not_found.json"},
	{"lessons.stream", "default", 200, "lessons/stream.json"},
	{"lessons.stream", "not_enrolled", 403, "errors/not_enrolled.json"},
	{"lessons.audio", "default", 200, "lessons/audio.json"},
	{"lessons.audio", "not_enrolled", 403, "errors/not_enrolled.json"},
	{"lessons.progress", "default", 200, "lessons/progress.json"},
	{"lessons.progress", "watched", 200, "lessons/progress.watched.json"},
	{"progress.batch", "default", 200, "progress/batch.json"},
	{"devices.list", "default", 200, "me/devices.json"},
	{"devices.delete", "default", 204, ""},
	{"devices.delete", "not_found", 404, "errors/not_found.json"},
	{"tv.pair.start", "default", 200, "tv/pair-start.json"},
	{"tv.pair.start", "rate_limited", 429, "errors/rate_limited.json"},
	{"tv.pair.poll", "default", 200, "tv/pair-poll.pending.json"},
	{"tv.pair.poll", "confirmed", 200, "tv/pair-poll.confirmed.json"},
	{"tv.pair.poll", "expired", 410, "errors/pair_code_expired.json"},
	{"tv.pair.confirm", "default", 200, "tv/pair-confirm.json"},
	{"tv.pair.confirm", "invalid_code", 404, "errors/pair_code_invalid.json"},
	{"tv.pair.confirm", "device_limit", 409, "errors/device_limit.json"},
	{"tv.logout", "default", 204, ""},
	{"app.config", "default", 200, "app/config.json"},
	{"app.config", "upgrade_required", 426, "errors/upgrade_required.json"},
}

// concretePath fills {id} of a route path with the id the tests use.
func concretePath(p string) string {
	switch {
	case strings.HasPrefix(p, "/courses/"):
		return strings.ReplaceAll(p, "{id}", "set-35")
	case strings.HasPrefix(p, "/lessons/"):
		return strings.ReplaceAll(p, "{id}", "l_3503")
	default:
		return strings.ReplaceAll(p, "{id}", "d_102")
	}
}

func idOf(p string) string {
	switch {
	case strings.HasPrefix(p, "/courses/"):
		return "set-35"
	case strings.HasPrefix(p, "/lessons/"):
		return "l_3503"
	case strings.Contains(p, "{id}"):
		return "d_102"
	}

	return ""
}

func TestExpectedTableMatchesRoutesJSON(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	got := map[string]bool{}
	routes := tr.Routes()
	assert.Len(t, routes, 24)
	for _, r := range routes {
		for _, n := range tr.ScenarioNames(r.ID) {
			got[r.ID+"/"+n] = true
		}
	}
	want := map[string]bool{}
	for _, e := range expected {
		want[e.route+"/"+e.scenario] = true
	}
	assert.Len(t, want, 50)
	assert.Equal(t, want, got)
}

func TestEveryRouteAndScenario(t *testing.T) {
	t.Parallel()
	for _, e := range expected {
		t.Run(e.route+"/"+e.scenario, func(t *testing.T) {
			t.Parallel()
			tr := newTransport(t)
			var route Route
			for _, r := range tr.Routes() {
				if r.ID == e.route {
					route = r
				}
			}
			require.NotEmpty(t, route.ID)
			require.NoError(t, tr.SetScenario(e.route, e.scenario))
			req := &api.Request{Method: route.Method, Path: concretePath(route.Path), Header: bearer()}
			resp, err := tr.Do(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, e.status, resp.Status)
			if e.file == "" {
				assert.Empty(t, resp.Body)

				return
			}
			want, err := os.ReadFile(mockDir + "/" + strings.ReplaceAll(e.file, "{{path.id}}", idOf(route.Path)))
			require.NoError(t, err)
			want = []byte(strings.ReplaceAll(string(want), "{{path.id}}", idOf(route.Path)))
			assert.NotContains(t, string(resp.Body), "mock-media://")
			assert.NotContains(t, string(resp.Body), "{{")
			assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
			if strings.Contains(string(want), "mock-media://") {
				base := tr.MediaServer().BaseURL()
				require.NotEmpty(t, base)
				want = []byte(strings.ReplaceAll(string(want), "mock-media://", base+"/"))
			}
			if e.route == "lessons.audio" && e.status == 200 {
				// size_bytes and sha256 are patched: all else equals the file.
				var g, w map[string]any
				require.NoError(t, json.Unmarshal(resp.Body, &g))
				require.NoError(t, json.Unmarshal(want, &w))
				for _, k := range []string{"size_bytes", "sha256"} {
					assert.NotEqual(t, w[k], g[k])
					delete(g, k)
					delete(w, k)
				}
				assert.Equal(t, w, g)

				return
			}
			assert.JSONEq(t, string(want), string(resp.Body))
		})
	}
}

func TestAuthRoutesWithoutToken(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	want, err := os.ReadFile(mockDir + "/errors/token_expired.json")
	require.NoError(t, err)
	n := 0
	for _, r := range tr.Routes() {
		if !r.Auth {
			continue
		}
		n++
		for name, h := range map[string]http.Header{
			"none":     nil,
			"empty":    {"Authorization": {"Bearer "}},
			"basic":    {"Authorization": {"Basic abc"}},
			"no space": {"Authorization": {"Bearer"}},
		} {
			resp, err := tr.Do(context.Background(), &api.Request{Method: r.Method, Path: concretePath(r.Path), Header: h})
			require.NoError(t, err, "%s %s", r.ID, name)
			assert.Equal(t, 401, resp.Status, "%s %s", r.ID, name)
			assert.JSONEq(t, string(want), string(resp.Body), "%s %s", r.ID, name)
		}
	}
	assert.Equal(t, 18, n)
}

func TestPublicRoutesNeedNoToken(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, r := range tr.Routes() {
		if r.Auth {
			continue
		}
		resp, err := tr.Do(context.Background(), &api.Request{Method: r.Method, Path: r.Path})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.Status, r.ID)
	}
}

func TestTemplateID(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	ids := []string{"intro", "hot-extra", "hot-extra-2"}
	for n := 24; n <= 36; n++ {
		ids = append(ids, "set-"+itoa(n))
	}
	assert.Len(t, ids, 16)
	for _, id := range ids {
		resp, err := tr.Do(context.Background(), get("/api/v1/courses/"+id))
		require.NoError(t, err)
		assert.Equal(t, 200, resp.Status, id)
		assert.Contains(t, string(resp.Body), `"`+id+`"`)
	}
	for _, bad := range []string{"/courses/set-99", "/courses/..", "/courses/a.b", "/courses/a%2Fb", "/courses/a/b", "/courses/", "/courses/" + strings.Repeat("a", 65)} {
		resp, err := tr.Do(context.Background(), get(bad))
		require.NoError(t, err)
		assert.Equal(t, 404, resp.Status, bad)
		assert.Contains(t, string(resp.Body), `"not_found"`, bad)
	}

	resp, err := tr.Do(context.Background(), get("/lessons/l_3507/stream"))
	require.NoError(t, err)
	assert.Contains(t, string(resp.Body), "/hls/l_3507/master.m3u8")
	resp, err = tr.Do(context.Background(), &api.Request{Method: "PUT", Path: "/lessons/l_3507/progress", Header: bearer()})
	require.NoError(t, err)
	assert.Contains(t, string(resp.Body), "l_3507")
}

func itoa(n int) string {
	b, _ := json.Marshal(n)

	return string(b)
}

func TestUnknownRouteIs404(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, c := range []struct{ method, path string }{
		{"GET", "/nope"},
		{"GET", ""},
		{"GET", "me"},
		{"DELETE", "/me"},
		{"GET", "/me/home/extra"},
		{"POST", "/me"},
	} {
		resp, err := tr.Do(context.Background(), &api.Request{Method: c.method, Path: c.path, Header: bearer()})
		require.NoError(t, err)
		assert.Equal(t, 404, resp.Status, "%s %q", c.method, c.path)
		assert.Contains(t, string(resp.Body), `"not_found"`)
	}
}

func TestBasePathAndQuery(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, p := range []string{"/me/courses", "/api/v1/me/courses"} {
		req := get(p)
		req.Query = map[string][]string{"cursor": {"c_6"}, "limit": {"6"}}
		resp, err := tr.Do(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.Status, p)
	}
	// the query is not read: a paged scenario answers page 1 for any cursor.
	require.NoError(t, tr.SetScenario("me.courses", "paged"))
	var first, again struct {
		Page struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"page"`
	}
	for i, dst := range []any{&first, &again} {
		req := get("/me/courses")
		if i == 1 {
			req.Query = map[string][]string{"cursor": {"c_6"}}
		}
		resp, err := tr.Do(context.Background(), req)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(resp.Body, dst))
	}
	require.NotNil(t, first.Page.NextCursor)
	assert.Equal(t, "c_6", *first.Page.NextCursor)
	assert.Equal(t, first, again)
}

func TestDelay(t *testing.T) {
	t.Parallel()
	rec := &waitRecorder{}
	tr := newTransport(t, func(o *Options) { o.Wait = rec.wait })
	for _, p := range []string{"/app/config", "/nope", "/me"} { // ok, 404, 401
		_, err := tr.Do(context.Background(), &api.Request{Method: "GET", Path: p})
		require.NoError(t, err)
	}
	assert.Equal(t, []time.Duration{250 * time.Millisecond, 250 * time.Millisecond, 250 * time.Millisecond}, rec.delays())
}

func TestDelayCanceled(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	tr := newTransport(t, func(o *Options) {
		o.Wait = func(ctx context.Context, _ time.Duration) error {
			close(entered)
			<-ctx.Done()

			return ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := tr.Do(ctx, get("/me"))
		errc <- err
	}()
	<-entered
	cancel()
	assert.ErrorIs(t, <-errc, context.Canceled)

	// a context canceled before the call never waits.
	_, err := tr.Do(ctx, get("/me"))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRealTimerWaitHonoursContext(t *testing.T) {
	t.Parallel()
	tr, err := New(os.DirFS(mockDir), Options{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = tr.Do(ctx, get("/me"))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	resp, err := tr.Do(context.Background(), get("/me"))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)
}

func TestNoBodyIsNil(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	resp, err := tr.Do(context.Background(), &api.Request{Method: "POST", Path: "/tv/logout", Header: bearer()})
	require.NoError(t, err)
	assert.Equal(t, 204, resp.Status)
	assert.Nil(t, resp.Body)
}

// TestLoginReplacingDeviceAnswersTheDefault checks K31: at the scenario device_limit a login with
// a non-empty replace_device_id gets the normal session (the device is revoked, the login
// completes) and one without it, or with an empty one, still gets the 409; the other scenarios
// are not touched by the field.
func TestLoginReplacingDeviceAnswersTheDefault(t *testing.T) {
	t.Parallel()

	const device = `"device":{"platform":"ios","name":"n","app_version":"1.0.0","install_id":"i"}`
	tests := []struct {
		name, scenario, body string
		status               int
		file                 string
	}{
		{"limit, no field", "device_limit", `{"email":"a@b.c","password":"x",` + device + `}`, 409, "errors/device_limit.json"},
		{"limit, empty field", "device_limit", `{"email":"a@b.c","password":"x",` + device + `,"replace_device_id":""}`, 409, "errors/device_limit.json"},
		{"limit, no body", "device_limit", ``, 409, "errors/device_limit.json"},
		{"limit, not JSON", "device_limit", `replace_device_id`, 409, "errors/device_limit.json"},
		{"limit, a device", "device_limit", `{"email":"a@b.c","password":"x",` + device + `,"replace_device_id":"d_102"}`, 200, "auth/login.json"},
		{"default, a device", "default", `{"email":"a@b.c","password":"x","replace_device_id":"d_102"}`, 200, "auth/login.json"},
		{"locked, a device", "account_locked", `{"email":"a@b.c","password":"x","replace_device_id":"d_102"}`, 423, "errors/account_locked.json"},
		{"wrong password, a device", "invalid_credentials", `{"email":"a@b.c","password":"x","replace_device_id":"d_102"}`, 401, "errors/invalid_credentials.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := newTransport(t)
			require.NoError(t, tr.SetScenario("auth.login", tc.scenario))
			var body []byte
			if tc.body != "" {
				body = []byte(tc.body)
			}
			resp, err := tr.Do(t.Context(), &api.Request{RouteID: "auth.login", Method: "POST", Path: "/api/v1/auth/login", Body: body})
			require.NoError(t, err)
			assert.Equal(t, tc.status, resp.Status)
			want, err := os.ReadFile(mockDir + "/" + tc.file)
			require.NoError(t, err)
			if tc.status != 200 {
				assert.JSONEq(t, string(want), string(resp.Body))

				return
			}
			// the session of auth/login.json, with its media pointed at the loopback server
			var got, file map[string]any
			require.NoError(t, json.Unmarshal(resp.Body, &got))
			require.NoError(t, json.Unmarshal(want, &file))
			assert.Equal(t, file["access_token"], got["access_token"])
			assert.Equal(t, file["refresh_token"], got["refresh_token"])
			assert.Equal(t, file["device"], got["device"])
		})
	}
}

// TestLoginReplacingOverHTTP checks the same through the loopback handler, the way cmd/mockserve
// serves the live and hybrid modes: an api.Client over api.Live over the handler is told 409 with
// the devices, sends the login again with replace_device_id and gets the session.
func TestLoginReplacingOverHTTP(t *testing.T) {
	t.Parallel()

	tr := newTransport(t)
	require.NoError(t, tr.SetScenario("auth.login", "device_limit"))
	srv := httptest.NewServer(tr.Handler())
	t.Cleanup(srv.Close)
	live, err := api.NewLive(api.LiveOptions{Base: srv.URL + "/api/v1", Client: noKeepAliveClient()})
	require.NoError(t, err)
	c, err := api.New(api.Options{Transport: live, Version: "1.0.0", Platform: "ios", InstallID: "i-1"})
	require.NoError(t, err)

	_, err = c.Login(t.Context(), "a@b.c", "x", model.DeviceInfo{})
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "device_limit", ae.Code)
	require.Len(t, ae.Body.Devices, 3)

	s, err := c.LoginReplacing(t.Context(), "a@b.c", "x", model.DeviceInfo{}, ae.Body.Devices[1].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, s.AccessToken)
	assert.NotEmpty(t, s.RefreshToken)
}
