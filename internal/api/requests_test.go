package api

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/model"
)

// TestRequestBodiesMatchMockRequests: for each of the 11 requests/*.json the
// client's body has the same key paths (nested) as the file.
func TestRequestBodiesMatchMockRequests(t *testing.T) {
	t.Parallel()

	load := func(t *testing.T, file string, v any) []byte {
		t.Helper()

		b := mockFile(t, "requests/"+file+".json")
		require.NoError(t, json.Unmarshal(b, v))

		return b
	}

	for _, tc := range []struct {
		file string
		call func(t *testing.T, c *Client) error
	}{
		{"auth.login", func(t *testing.T, c *Client) error {
			var r model.LoginRequest
			load(t, "auth.login", &r)

			return ignore(c.Login(context.Background(), r.Email, r.Password, r.Device))
		}},
		{"auth.refresh", func(t *testing.T, c *Client) error {
			var r model.RefreshRequest
			load(t, "auth.refresh", &r)

			return ignore(c.Refresh(context.Background(), r.RefreshToken))
		}},
		{"auth.forgot", func(t *testing.T, c *Client) error {
			var r model.ForgotRequest
			load(t, "auth.forgot", &r)

			return ignore(c.Forgot(context.Background(), r.Email))
		}},
		{"me.update", func(t *testing.T, c *Client) error {
			var r model.MeUpdateRequest
			load(t, "me.update", &r)

			return ignore(c.UpdateMe(context.Background(), r))
		}},
		{"me.password", func(t *testing.T, c *Client) error {
			var r model.PasswordRequest
			load(t, "me.password", &r)

			return ignore(c.ChangePassword(context.Background(), r))
		}},
		{"me.coach.update", func(t *testing.T, c *Client) error {
			var r model.CoachUpdateRequest
			load(t, "me.coach.update", &r)

			return ignore(c.UpdateCoachProfile(context.Background(), r))
		}},
		{"lessons.progress", func(t *testing.T, c *Client) error {
			var r model.ProgressRequest
			load(t, "lessons.progress", &r)

			return ignore(c.SaveProgress(context.Background(), "l_3503", r))
		}},
		{"progress.batch", func(t *testing.T, c *Client) error {
			var r model.BatchRequest
			load(t, "progress.batch", &r)

			return ignore(c.SendProgressBatch(context.Background(), r))
		}},
		{"tv.pair.start", func(t *testing.T, c *Client) error {
			var r model.PairStartRequest
			load(t, "tv.pair.start", &r)

			return ignore(c.StartPairing(context.Background(), r.Device))
		}},
		{"tv.pair.poll", func(t *testing.T, c *Client) error {
			var r model.PairPollRequest
			load(t, "tv.pair.poll", &r)

			return ignore(c.PollPairing(context.Background(), r.Code, r.PollSecret))
		}},
		{"tv.pair.confirm", func(t *testing.T, c *Client) error {
			var r model.PairConfirmRequest
			load(t, "tv.pair.confirm", &r)

			return ignore(c.ConfirmPairing(context.Background(), r.Code))
		}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			tr := always(200, "{}")
			c := newTestClient(t, tr)
			require.NoError(t, tc.call(t, c))
			reqs := tr.seen()
			require.Len(t, reqs, 1)

			want := keyPaths(t, mockFile(t, "requests/"+tc.file+".json"))
			got := keyPaths(t, reqs[0].Body)
			assert.Equal(t, want, got)
		})
	}
}

func TestEmptySlicesAreNotNull(t *testing.T) {
	t.Parallel()

	tr := always(200, "{}")
	c := newTestClient(t, tr)
	require.NoError(t, ignore(c.UpdateCoachProfile(context.Background(), model.CoachUpdateRequest{Bio: "b"})))
	assert.JSONEq(t, `{"bio":"b","socials":[],"places":[]}`, string(tr.seen()[0].Body))
}

func TestMultipartAvatar(t *testing.T) {
	t.Parallel()

	jpeg := append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{7}, 100)...)
	tr := always(200, `{"avatar_url":"https://x.test/a.jpg","width":512,"height":512}`)
	c := newTestClient(t, tr)

	av, err := c.UploadAvatar(context.Background(), jpeg, `my "photo".jpg`)
	require.NoError(t, err)
	assert.Equal(t, 512, av.Width)

	req := tr.seen()[0]
	assert.Equal(t, "PUT", req.Method)
	assert.Equal(t, "/me/avatar", req.Path)
	mt, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mt)

	r := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
	part, err := r.NextPart()
	require.NoError(t, err)
	assert.Equal(t, "file", part.FormName())
	assert.Equal(t, `my "photo".jpg`, part.FileName())
	assert.Equal(t, "image/jpeg", part.Header.Get("Content-Type"))
	data, err := io.ReadAll(part)
	require.NoError(t, err)
	assert.Equal(t, jpeg, data)
	_, err = r.NextPart()
	assert.ErrorIs(t, err, io.EOF)

	// Empty image and default name.
	assert.Error(t, ignore(c.UploadAvatar(context.Background(), nil, "x.jpg")))
	require.NoError(t, ignore(c.UploadAvatar(context.Background(), jpeg, "")))
	assert.Contains(t, string(tr.seen()[1].Body), `filename="avatar.jpg"`)
}

func TestLoginRequestBodyNotLogged(t *testing.T) {
	t.Parallel()

	// The model prints no password; the client adds no way to print it.
	r := model.LoginRequest{Email: "a@example.test", Password: "hunter2"}
	assert.NotContains(t, strings.Join([]string{r.String()}, ""), "hunter2")
}

// TestLoginReplacingBody checks the optional field of K31: the body has replace_device_id only
// when one is given, and is otherwise the body of Login with the same keys as the mock's
// requests/auth.login.json.
func TestLoginReplacingBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"none", "", false},
		{"a device of the error", "d_102", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := always(200, "{}")
			c := newTestClient(t, tr)
			require.NoError(t, ignore(c.LoginReplacing(context.Background(), "a@example.test", "pw", model.DeviceInfo{}, tc.id)))
			reqs := tr.seen()
			require.Len(t, reqs, 1)
			assert.Equal(t, "auth.login", reqs[0].RouteID)
			assert.Equal(t, "POST", reqs[0].Method)
			var got model.LoginRequest
			require.NoError(t, json.Unmarshal(reqs[0].Body, &got))
			assert.Equal(t, tc.id, got.ReplaceDeviceID)
			assert.Equal(t, "a@example.test", got.Email)
			assert.Equal(t, "pw", got.Password)
			assert.Equal(t, tc.want, strings.Contains(string(reqs[0].Body), "replace_device_id"))
			if !tc.want {
				assert.Equal(t, keyPaths(t, mockFile(t, "requests/auth.login.json")), keyPaths(t, reqs[0].Body))
			}
		})
	}
}

// TestLoginReplacingThroughHybrid checks that the field reaches the live side of a hybrid
// transport, which keeps auth.login and auth.refresh together: the body the server gets is the
// client's, whole; a 400 on the field comes back as the server's error with its fields.
func TestLoginReplacingThroughHybrid(t *testing.T) {
	t.Parallel()

	live := &scriptTransport{fn: func(_ int, r *Request) (*Response, error) {
		if strings.Contains(string(r.Body), `"replace_device_id":"d_nope"`) {
			return &Response{Status: 400, Body: []byte(`{"error":{"code":"validation_failed","message":"m","fields":{"replace_device_id":"x"}}}`)}, nil
		}

		return &Response{Status: 200, Body: mockFile(t, "auth/login.json")}, nil
	}}
	mock := always(200, "{}")
	c := newTestClient(t, NewHybrid(live, mock, []string{"auth.login", "auth.refresh"}))

	s, err := c.LoginReplacing(context.Background(), "a@example.test", "pw", model.DeviceInfo{}, "d_102")
	require.NoError(t, err)
	assert.NotEmpty(t, s.AccessToken)
	require.Len(t, live.seen(), 1)
	assert.Empty(t, mock.seen(), "auth.login is live")
	assert.Contains(t, string(live.seen()[0].Body), `"replace_device_id":"d_102"`)

	_, err = c.LoginReplacing(context.Background(), "a@example.test", "pw", model.DeviceInfo{}, "d_nope")
	var ae *Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
	assert.Equal(t, "validation_failed", ae.Code)
	assert.Contains(t, ae.Body.Fields, "replace_device_id")
}
