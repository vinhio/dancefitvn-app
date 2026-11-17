package api

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/model"
)

func ignore[T any](_ T, err error) error { return err }

// invokers calls the client method of every route.
var invokers = map[string]func(ctx context.Context, c *Client) error{
	"auth.login": func(ctx context.Context, c *Client) error {
		return ignore(c.Login(ctx, "a@example.test", "pw", model.DeviceInfo{}))
	},
	"auth.refresh": func(ctx context.Context, c *Client) error { return ignore(c.Refresh(ctx, "r")) },
	"auth.logout":  func(ctx context.Context, c *Client) error { return c.Logout(ctx) },
	"auth.forgot":  func(ctx context.Context, c *Client) error { return ignore(c.Forgot(ctx, "a@example.test")) },
	"me.get":       func(ctx context.Context, c *Client) error { return ignore(c.Me(ctx)) },
	"me.update": func(ctx context.Context, c *Client) error {
		return ignore(c.UpdateMe(ctx, model.MeUpdateRequest{}))
	},
	"me.password": func(ctx context.Context, c *Client) error {
		return ignore(c.ChangePassword(ctx, model.PasswordRequest{}))
	},
	"me.avatar": func(ctx context.Context, c *Client) error {
		return ignore(c.UploadAvatar(ctx, []byte("\xff\xd8\xff\xe0jpeg"), "a.jpg"))
	},
	"me.coach.get": func(ctx context.Context, c *Client) error { return ignore(c.CoachProfile(ctx)) },
	"me.coach.update": func(ctx context.Context, c *Client) error {
		return ignore(c.UpdateCoachProfile(ctx, model.CoachUpdateRequest{}))
	},
	"me.home":    func(ctx context.Context, c *Client) error { return ignore(c.Home(ctx)) },
	"me.courses": func(ctx context.Context, c *Client) error { return ignore(c.MyCourses(ctx, "", 0)) },
	"courses.show": func(ctx context.Context, c *Client) error {
		return ignore(c.Course(ctx, "set-35"))
	},
	"lessons.stream": func(ctx context.Context, c *Client) error { return ignore(c.Stream(ctx, "l_3503")) },
	"lessons.audio":  func(ctx context.Context, c *Client) error { return ignore(c.Audio(ctx, "l_3503")) },
	"lessons.progress": func(ctx context.Context, c *Client) error {
		return ignore(c.SaveProgress(ctx, "l_3503", model.ProgressRequest{}))
	},
	"progress.batch": func(ctx context.Context, c *Client) error {
		return ignore(c.SendProgressBatch(ctx, model.BatchRequest{}))
	},
	"devices.list":   func(ctx context.Context, c *Client) error { return ignore(c.Devices(ctx)) },
	"devices.delete": func(ctx context.Context, c *Client) error { return c.DeleteDevice(ctx, "d_101") },
	"tv.pair.start": func(ctx context.Context, c *Client) error {
		return ignore(c.StartPairing(ctx, model.DeviceInfo{}))
	},
	"tv.pair.poll": func(ctx context.Context, c *Client) error { return ignore(c.PollPairing(ctx, "c", "s")) },
	"tv.pair.confirm": func(ctx context.Context, c *Client) error {
		return ignore(c.ConfirmPairing(ctx, "c"))
	},
	"tv.logout":  func(ctx context.Context, c *Client) error { return c.LogoutTV(ctx) },
	"app.config": func(ctx context.Context, c *Client) error { return ignore(c.Config(ctx)) },
}

type routesJSON struct {
	Routes []struct {
		ID        string `json:"id"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		Auth      bool   `json:"auth"`
		Responses map[string]struct {
			Status int     `json:"status"`
			File   *string `json:"file"`
		} `json:"responses"`
	} `json:"routes"`
}

func loadRoutesJSON(t testing.TB) routesJSON {
	t.Helper()

	var r routesJSON
	require.NoError(t, json.Unmarshal(mockFile(t, "routes.json"), &r))

	return r
}

func TestRouteTableMatchesRoutesJSON(t *testing.T) {
	t.Parallel()

	r := loadRoutesJSON(t)
	require.Len(t, r.Routes, 24)
	require.Len(t, Routes(), 24)

	for _, want := range r.Routes {
		t.Run(want.ID, func(t *testing.T) {
			t.Parallel()

			got, ok := RouteByID(want.ID)
			require.True(t, ok)
			assert.Equal(t, want.Method, got.Method)
			assert.Equal(t, want.Path, got.Path)
			assert.Equal(t, want.Auth, got.Auth)
			assert.Equal(t, want.Method == "GET" && want.ID != "lessons.stream" && want.ID != "lessons.audio", got.Cacheable)
			assert.Contains(t, invokers, want.ID)
		})
	}
	_, ok := RouteByID("nope")
	assert.False(t, ok)
}

func TestNew(t *testing.T) {
	t.Parallel()

	ok := scriptTransport{}
	for _, tc := range []struct {
		name string
		o    Options
		fail bool
	}{
		{"ok", Options{Transport: &ok, Version: "1", Platform: "ios"}, false},
		{"android", Options{Transport: &ok, Version: "1", Platform: "android"}, false},
		{"tvos", Options{Transport: &ok, Version: "1", Platform: "tvos"}, false},
		{"no transport", Options{Version: "1", Platform: "ios"}, true},
		{"no version", Options{Transport: &ok, Platform: "ios"}, true},
		{"no platform", Options{Transport: &ok, Version: "1"}, true},
		{"bad platform", Options{Transport: &ok, Version: "1", Platform: "web"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := New(tc.o)
			if tc.fail {
				assert.ErrorIs(t, err, core.ErrInvalid)
				assert.Nil(t, c)

				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

// TestEveryRouteEveryScenario answers each scenario of routes.json with its
// file: 2xx decodes, an error becomes *Error with the code of the file.
func TestEveryRouteEveryScenario(t *testing.T) {
	t.Parallel()

	n := 0
	for _, route := range loadRoutesJSON(t).Routes {
		for name, sc := range route.Responses {
			n++
			t.Run(route.ID+"/"+name, func(t *testing.T) {
				t.Parallel()

				var body []byte
				if sc.File != nil {
					body = []byte(strings.ReplaceAll(string(mockFile(t, strings.ReplaceAll(*sc.File, "{{path.id}}", "set-35"))), "{{path.id}}", "set-35"))
				}
				tr := &scriptTransport{fn: func(int, *Request) (*Response, error) {
					return &Response{Status: sc.Status, Body: body}, nil
				}}
				c := newTestClient(t, tr)

				err := invokers[route.ID](context.Background(), c)
				if sc.Status < 300 {
					assert.NoError(t, err)

					return
				}
				var ae *Error
				require.ErrorAs(t, err, &ae)
				var want model.ErrorBody
				require.NoError(t, json.Unmarshal(body, &want))
				assert.Equal(t, sc.Status, ae.Status)
				assert.Equal(t, want.Error.Code, ae.Code)
				assert.Equal(t, want.Error.Message, ae.Message)
				assert.Len(t, tr.seen(), 1, "no retry, no refresh without a token source")
			})
		}
	}
	assert.Equal(t, 50, n, "scenarios in routes.json")
}

func TestHeaders(t *testing.T) {
	t.Parallel()

	for _, route := range Routes() {
		for _, withToken := range []bool{true, false} {
			t.Run(route.ID, func(t *testing.T) {
				t.Parallel()

				// 200 with "{}" decodes into every type; 204 routes ignore it.
				tr := always(200, "{}")
				ts := &tokenSource{}
				if withToken {
					ts.token = "tok-1"
				}
				c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

				require.NoError(t, invokers[route.ID](context.Background(), c))
				reqs := tr.seen()
				require.Len(t, reqs, 1)
				h := reqs[0].Header

				assert.Equal(t, route.ID, reqs[0].RouteID)
				assert.Equal(t, route.Method, reqs[0].Method)
				assert.Equal(t, "1.2.3", h.Get("X-App-Version"))
				assert.Equal(t, "ios", h.Get("X-Platform"))
				assert.Equal(t, "11111111-2222-4333-8444-555555555555", h.Get("X-Install-Id"))
				assert.Equal(t, "vi", h.Get("Accept-Language"))
				assert.Equal(t, "application/json", h.Get("Accept"))
				if route.Auth && withToken {
					assert.Equal(t, "Bearer tok-1", h.Get("Authorization"))
				} else {
					assert.Empty(t, h.Get("Authorization"))
				}
				switch {
				case len(reqs[0].Body) == 0:
					assert.Empty(t, h.Get("Content-Type"))
				case route.ID == "me.avatar":
					assert.True(t, strings.HasPrefix(h.Get("Content-Type"), "multipart/form-data; boundary="))
				default:
					assert.Equal(t, "application/json", h.Get("Content-Type"))
				}
			})
		}
	}
}

func TestHeadersPerPlatform(t *testing.T) {
	t.Parallel()

	for _, p := range []string{"ios", "android", "tvos"} {
		t.Run(p, func(t *testing.T) {
			t.Parallel()

			tr := always(200, "{}")
			c := newTestClient(t, tr, func(o *Options) { o.Platform = p })
			require.NoError(t, ignore(c.Config(context.Background())))
			assert.Equal(t, p, tr.seen()[0].Header.Get("X-Platform"))
		})
	}
}

func TestMyCoursesQuery(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		cursor string
		limit  int
		want   string
	}{
		{"none", "", 0, ""},
		{"limit", "", 20, "limit=20"},
		{"cursor", "abc", 0, "cursor=abc"},
		{"both", "a b", 50, "cursor=a+b&limit=50"},
		{"too big", "", 51, ""},
		{"negative", "", -1, ""},
		{"one", "", 1, "limit=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := always(200, "{}")
			c := newTestClient(t, tr)
			require.NoError(t, ignore(c.MyCourses(context.Background(), tc.cursor, tc.limit)))
			assert.Equal(t, tc.want, tr.seen()[0].Query.Encode())
			assert.Equal(t, "/me/courses", tr.seen()[0].Path)
		})
	}
}

func TestPathID(t *testing.T) {
	t.Parallel()

	tr := always(200, "{}")
	c := newTestClient(t, tr)
	ctx := context.Background()

	require.NoError(t, ignore(c.Course(ctx, "a/b c")))
	assert.Equal(t, "/courses/a%2Fb%20c", tr.seen()[0].Path)

	err := ignore(c.Course(ctx, ""))
	assert.ErrorIs(t, err, core.ErrInvalid)
	assert.Len(t, tr.seen(), 1, "an empty id is refused before the transport")
}

func TestDeviceDefaults(t *testing.T) {
	t.Parallel()

	tr := always(200, "{}")
	c := newTestClient(t, tr)
	ctx := context.Background()

	require.NoError(t, ignore(c.Login(ctx, "a@example.test", "pw", model.DeviceInfo{Name: "iPhone"})))
	var lr model.LoginRequest
	require.NoError(t, json.Unmarshal(tr.seen()[0].Body, &lr))
	assert.Equal(t, model.DeviceInfo{Platform: "ios", Name: "iPhone", AppVersion: "1.2.3", InstallID: "11111111-2222-4333-8444-555555555555"}, lr.Device)
}

func TestDecodeFailure(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"not json", "[1]", `{"user": 5}`} {
		c := newTestClient(t, always(200, body))
		err := ignore(c.Me(context.Background()))
		assert.ErrorIs(t, err, ErrServer, body)
	}
}

func TestNonErrorStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []int{301, 304} {
		c := newTestClient(t, always(status, ""))
		err := ignore(c.Config(context.Background()))
		assert.Error(t, err, status)
	}
}

// TestDecodeComposes checks that the API's text comes out in NFC whatever
// form the server stored it in: decomposed (NFD), raw or as \u escapes in
// the JSON text, in nested values, pointers and anys, and in error bodies; and
// that nulls, wrong kinds and named string types keep their default
// decoding.
func TestDecodeComposes(t *testing.T) {
	t.Parallel()

	const (
		nfd     = "Bu\u0323ng Na\u0306\u0301ng"     // NFD bytes in the JSON text
		escaped = `Thu\u031bo\u031bng \ud83e\udd20` // JSON escapes: NFD, an emoji too
		nfc     = "B\u1ee5ng N\u1eafng"             // what nfd decodes to
		nfcEsc  = "Th\u01b0\u01a1ng \U0001F920"     // what escaped decodes to
	)
	body := `{"id":"set-1","name":"` + nfd + `","tagline":null,` +
		`"lessons":[{"id":"l_1","title":"` + escaped + `","tag":"` + nfd + `","duration_sec":60}]}`
	c := newTestClient(t, always(200, body))
	got, err := c.Course(context.Background(), "set-1")
	require.NoError(t, err)
	assert.Equal(t, nfc, got.Name)
	assert.Empty(t, got.Tagline, "null stays the zero string")
	require.Len(t, got.Lessons, 1)
	assert.Equal(t, nfcEsc, got.Lessons[0].Title)
	assert.Equal(t, nfc, got.Lessons[0].Tag)

	var v struct {
		Category model.Category `json:"category"`
		Ptr      *string        `json:"ptr"`
		Any      map[string]any `json:"any"`
	}
	require.NoError(t, Decode([]byte(`{"category":"regular","ptr":"`+nfd+`","any":{"k":"`+nfd+`"}}`), &v))
	assert.Equal(t, model.CategoryRegular, v.Category, "a named type decodes as before")
	require.NotNil(t, v.Ptr)
	assert.Equal(t, nfc, *v.Ptr)
	assert.Equal(t, nfc, v.Any["k"], "a string decoded into any")

	assert.Error(t, Decode([]byte(`{"name":5}`), &got), "a number for a string still fails")

	e := newTestClient(t, always(403, `{"error":{"code":"forbidden","message":"`+nfd+`"}}`))
	_, err = e.Course(context.Background(), "set-1")
	var apiErr *Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, nfc, apiErr.Message)
}

func TestDecodeComposedTextDoesNotChange(t *testing.T) {
	t.Parallel()

	var got model.Lesson
	require.NoError(t, Decode([]byte("{\"title\":\"B\u1ee5ng \U0001F920\",\"tag\":\"Set 35\"}"), &got))
	assert.Equal(t, "B\u1ee5ng \U0001F920", got.Title)
	assert.Equal(t, "Set 35", got.Tag)
}
