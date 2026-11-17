package model

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockDir = "../../mock/dancefitvn-mock-api"

// fileTypes maps every JSON file of the mock API (except routes.json and the
// media folder) to the type that must decode it.
var fileTypes = map[string]any{
	"app/config.json":                AppConfig{},
	"auth/login.json":                Session{},
	"auth/refresh.json":              Tokens{},
	"auth/forgot.json":               OKMessage{},
	"lessons/audio.json":             Audio{},
	"lessons/progress.json":          ProgressResult{},
	"lessons/progress.watched.json":  ProgressResult{},
	"lessons/stream.json":            Stream{},
	"me/avatar.json":                 Avatar{},
	"me/coach-profile.json":          CoachProfile{},
	"me/coach-profile.put.json":      CoachProfile{},
	"me/courses.cat-special.json":    CoursePage{},
	"me/courses.json":                CoursePage{},
	"me/courses.page1.json":          CoursePage{},
	"me/courses.search-empty.json":   CoursePage{},
	"me/devices.json":                DeviceList{},
	"me/home.json":                   Home{},
	"me/home.new_user.json":          Home{},
	"me/me.json":                     Me{},
	"me/me.patch.json":               MeUpdate{},
	"me/password.json":               PasswordResult{},
	"progress/batch.json":            BatchResult{},
	"tv/pair-confirm.json":           PairConfirm{},
	"tv/pair-poll.confirmed.json":    PairPoll{},
	"tv/pair-poll.pending.json":      PairPoll{},
	"tv/pair-start.json":             PairStart{},
	"requests/auth.forgot.json":      ForgotRequest{},
	"requests/auth.login.json":       LoginRequest{},
	"requests/auth.refresh.json":     RefreshRequest{},
	"requests/lessons.progress.json": ProgressRequest{},
	"requests/me.coach.update.json":  CoachUpdateRequest{},
	"requests/me.password.json":      PasswordRequest{},
	"requests/me.update.json":        MeUpdateRequest{},
	"requests/progress.batch.json":   BatchRequest{},
	"requests/tv.pair.confirm.json":  PairConfirmRequest{},
	"requests/tv.pair.poll.json":     PairPollRequest{},
	"requests/tv.pair.start.json":    PairStartRequest{},
}

func init() {
	// courses/*.json are Courses, errors/*.json are ErrorBodies.
	for _, c := range []string{
		"hot-extra-2", "hot-extra", "intro", "set-24", "set-25", "set-26", "set-27", "set-28", "set-29",
		"set-30", "set-31", "set-32", "set-33", "set-34", "set-35", "set-36",
	} {
		fileTypes["courses/"+c+".json"] = Course{}
	}
	for _, e := range []string{
		"account_locked", "device_limit", "file_too_large", "invalid_credentials", "invalid_social",
		"not_enrolled", "not_found", "pair_code_expired", "pair_code_invalid", "rate_limited",
		"refresh_invalid", "server_error", "token_expired", "upgrade_required", "validation_failed",
		"wrong_password",
	} {
		fileTypes["errors/"+e+".json"] = ErrorBody{}
	}
}

// readMock reads a mock file with the template token replaced.
func readMock(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(mockDir + "/" + name)
	require.NoError(t, err)

	return []byte(strings.ReplaceAll(string(b), "{{path.id}}", "l_3503"))
}

func TestEveryMockFileIsInTheTable(t *testing.T) {
	t.Parallel()

	seen := 0
	err := fs.WalkDir(os.DirFS(mockDir), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == "media" {
				return fs.SkipDir
			}

			return nil
		}
		if !strings.HasSuffix(p, ".json") || p == "routes.json" {
			return nil
		}
		seen++
		assert.Contains(t, fileTypes, p, "mock file has no model type in the test table")

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 69, seen)
	assert.Len(t, fileTypes, 69)
}

func TestEveryMockFileDecodesStrictly(t *testing.T) {
	t.Parallel()

	for name, proto := range fileTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v := reflect.New(reflect.TypeOf(proto)).Interface()
			err := json.Unmarshal(readMock(t, name), v, json.RejectUnknownMembers(true))
			require.NoError(t, err)
			assert.False(t, reflect.ValueOf(v).Elem().IsZero() && !strings.Contains(name, "pending"), "decoded to the zero value")
		})
	}
}

// TestRoundTripKeepsKeys decodes every file, encodes the model and compares
// with the source: every non-zero value of the source survives (a null may
// become the zero value), and a key the source lacks is at most a zero.
func TestRoundTripKeepsKeys(t *testing.T) {
	t.Parallel()

	for name, proto := range fileTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := readMock(t, name)
			v := reflect.New(reflect.TypeOf(proto)).Interface()
			require.NoError(t, json.Unmarshal(src, v))
			out, err := json.Marshal(v)
			require.NoError(t, err)

			var a, b any
			require.NoError(t, json.Unmarshal(src, &a))
			require.NoError(t, json.Unmarshal(out, &b))
			assertSame(t, "$", a, b)
		})
	}
}

func assertSame(t *testing.T, path string, src, out any) {
	t.Helper()
	switch s := src.(type) {
	case nil:
		return
	case map[string]any:
		o, ok := out.(map[string]any)
		if !assert.True(t, ok, "%s: want object, got %T", path, out) {
			return
		}
		for k, sv := range s {
			ov, found := o[k]
			if !found {
				// omitzero may drop a zero value; a typo in a tag cannot
				// hide here, strict decoding catches it.
				assert.True(t, deepZero(sv), "%s.%s lost: %v", path, k, sv)

				continue
			}
			assertSame(t, path+"."+k, sv, ov)
		}
		for k, ov := range o {
			if _, found := s[k]; !found {
				assert.True(t, zeroish(ov), "%s.%s appeared as %v", path, k, ov)
			}
		}
	case []any:
		o, ok := out.([]any)
		if !assert.True(t, ok, "%s: want array, got %T", path, out) || !assert.Len(t, o, len(s), path) {
			return
		}
		for i := range s {
			assertSame(t, path+"[]", s[i], o[i])
		}
	default:
		assert.Equal(t, src, out, path)
	}
}

// deepZero reports whether every leaf of v is a zero value.
func deepZero(v any) bool {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if !deepZero(e) {
				return false
			}
		}

		return true
	case map[string]any:
		for _, e := range x {
			if !deepZero(e) {
				return false
			}
		}

		return true
	default:
		return zeroish(v)
	}
}

func zeroish(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case float64:
		return x == 0
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	default:
		return false
	}
}

func TestNullsBecomeZeroValues(t *testing.T) {
	t.Parallel()

	var c Course
	require.NoError(t, json.Unmarshal(readMock(t, "courses/set-36.json"), &c))
	assert.Zero(t, c.TotalDurationSec)
	assert.True(t, c.Lessons[1].Locked)
	assert.Empty(t, c.Lessons[1].ID)
	assert.True(t, c.Lessons[0].Preview)

	var intro Course
	require.NoError(t, json.Unmarshal(readMock(t, "courses/intro.json"), &intro))
	assert.Nil(t, intro.Resume)

	var h Home
	require.NoError(t, json.Unmarshal(readMock(t, "me/home.new_user.json"), &h))
	assert.Nil(t, h.Continue)

	var p CoursePage
	require.NoError(t, json.Unmarshal(readMock(t, "me/courses.page1.json"), &p))
	require.NotNil(t, p.Page.NextCursor)
	assert.Equal(t, "c_6", *p.Page.NextCursor)

	var mu MeUpdate
	require.NoError(t, json.Unmarshal(readMock(t, "me/me.patch.json"), &mu))
	assert.Nil(t, mu.EmailChangePending)
}

func TestDecodedValues(t *testing.T) {
	t.Parallel()

	var me Me
	require.NoError(t, json.Unmarshal(readMock(t, "me/me.json"), &me))
	assert.Equal(t, time.Date(2024, 3, 12, 0, 0, 0, 0, time.UTC), me.User.MemberSince.Time())
	assert.Equal(t, 2, me.Devices.Limit.Phone)

	var h Home
	require.NoError(t, json.Unmarshal(readMock(t, "me/home.json"), &h))
	require.NotNil(t, h.Continue)
	assert.Equal(t, "l_3503", h.Continue.Lesson.ID)
	assert.Equal(t, CategoryRegular, h.MyCourses[0].Category)

	var poll PairPoll
	require.NoError(t, json.Unmarshal(readMock(t, "tv/pair-poll.confirmed.json"), &poll))
	assert.Equal(t, PairConfirmed, poll.Status)
	assert.Equal(t, PlatformTVOS, poll.Session().Device.Platform)
	assert.Equal(t, 900, poll.Session().ExpiresIn)

	var pending PairPoll
	require.NoError(t, json.Unmarshal(readMock(t, "tv/pair-poll.pending.json"), &pending))
	assert.Equal(t, PairPending, pending.Status)
	assert.Equal(t, 272, pending.ExpiresIn)

	var eb ErrorBody
	require.NoError(t, json.Unmarshal(readMock(t, "errors/upgrade_required.json"), &eb))
	assert.Equal(t, "https://apps.apple.com/", eb.Error.StoreURL["ios"])
	require.NoError(t, json.Unmarshal(readMock(t, "errors/device_limit.json"), &eb))
	assert.Len(t, eb.Error.Devices, 3)

	var b BatchResult
	require.NoError(t, json.Unmarshal([]byte(`{"accepted":1,"rejected":[{"lesson_id":"x"}]}`), &b))
	assert.Equal(t, jsontext.Value(`{"lesson_id":"x"}`), b.Rejected[0])
}

func TestUnknownMembersAreIgnoredByDefault(t *testing.T) {
	t.Parallel()

	var c Course
	require.NoError(t, json.Unmarshal([]byte(`{"id":"x","brand_new_field":1}`), &c))
	assert.Equal(t, "x", c.ID)
	assert.Error(t, json.Unmarshal([]byte(`{"id":"x","brand_new_field":1}`), &c, json.RejectUnknownMembers(true)))
}

func TestEnumText(t *testing.T) {
	t.Parallel()

	t.Run("category", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			text string
			want Category
		}{{"regular", CategoryRegular}, {"special", CategorySpecial}, {"intro", CategoryIntro}, {"new", CategoryUnknown}, {"", CategoryUnknown}} {
			var c Category
			require.NoError(t, c.UnmarshalText([]byte(tc.text)))
			assert.Equal(t, tc.want, c, tc.text)
			assert.Equal(t, tc.text == "new" || tc.text == "", c == CategoryUnknown)
			if c != CategoryUnknown {
				assert.Equal(t, tc.text, c.String())
			}
		}
		assert.Equal(t, "", Category(99).String())
	})
	t.Run("platform", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			text string
			want Platform
		}{{"ios", PlatformIOS}, {"android", PlatformAndroid}, {"tvos", PlatformTVOS}, {"watchos", PlatformUnknown}} {
			var p Platform
			require.NoError(t, p.UnmarshalText([]byte(tc.text)))
			assert.Equal(t, tc.want, p, tc.text)
			b, err := p.MarshalText()
			require.NoError(t, err)
			if p != PlatformUnknown {
				assert.Equal(t, tc.text, string(b))
			}
		}
		assert.Equal(t, "", Platform(99).String())
	})
	t.Run("pair status", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			text string
			want PairStatus
		}{{"pending", PairPending}, {"confirmed", PairConfirmed}, {"expired", PairUnknown}} {
			var s PairStatus
			require.NoError(t, s.UnmarshalText([]byte(tc.text)))
			assert.Equal(t, tc.want, s, tc.text)
			b, err := s.MarshalText()
			require.NoError(t, err)
			if s != PairUnknown {
				assert.Equal(t, tc.text, string(b))
			}
		}
		assert.Equal(t, "", PairStatus(99).String())
	})
	t.Run("unknown text inside a document", func(t *testing.T) {
		t.Parallel()
		var c Course
		require.NoError(t, json.Unmarshal([]byte(`{"category":"vip"}`), &c))
		assert.Equal(t, CategoryUnknown, c.Category)
	})
}

func TestDateText(t *testing.T) {
	t.Parallel()

	var d Date
	require.NoError(t, d.UnmarshalText([]byte("2024-03-12")))
	assert.Equal(t, "2024-03-12", d.String())
	b, err := d.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "2024-03-12", string(b))

	require.NoError(t, d.UnmarshalText(nil))
	assert.True(t, d.IsZero())
	assert.Empty(t, d.String())

	for _, bad := range []string{"12/03/2024", "2024-13-01", "x"} {
		err := d.UnmarshalText([]byte(bad))
		assert.ErrorIs(t, err, core.ErrInvalid, bad)
	}
}
