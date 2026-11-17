package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"unicode"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorMapping checks the 16 codes of 05 §3 against errors/*.json.
func TestErrorMapping(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		code   string
		status int
		is     []error
		isNot  []error
		check  func(t *testing.T, e *Error)
	}{
		{"invalid_credentials", 401, []error{ErrUnauthorized}, []error{ErrSignedOut}, func(t *testing.T, e *Error) { assert.Equal(t, 3, e.Body.AttemptsLeft) }},
		{"account_locked", 423, []error{ErrLocked}, nil, func(t *testing.T, e *Error) { assert.Equal(t, 900, e.Body.RetryAfter) }},
		{"device_limit", 409, []error{ErrConflict}, nil, func(t *testing.T, e *Error) {
			require.Len(t, e.Body.Devices, 3)
			assert.Equal(t, "d_101", e.Body.Devices[0].ID)
		}},
		{"refresh_invalid", 401, []error{ErrUnauthorized}, nil, nil},
		{"token_expired", 401, []error{ErrUnauthorized}, []error{ErrSignedOut}, nil},
		{"rate_limited", 429, []error{ErrRateLimited}, nil, func(t *testing.T, e *Error) { assert.Equal(t, 60, e.Body.RetryAfter) }},
		{"validation_failed", 400, []error{core.ErrInvalid}, nil, func(t *testing.T, e *Error) {
			assert.Equal(t, map[string]string{"first_name": "Tên là bắt buộc", "email": "Email không hợp lệ"}, e.Body.Fields)
		}},
		{"wrong_password", 400, []error{core.ErrInvalid}, nil, func(t *testing.T, e *Error) { assert.Contains(t, e.Body.Fields, "current_password") }},
		{"file_too_large", 413, []error{core.ErrInvalid}, nil, func(t *testing.T, e *Error) { assert.EqualValues(t, 2097152, e.Body.MaxBytes) }},
		{"invalid_social", 400, []error{core.ErrInvalid}, nil, func(t *testing.T, e *Error) { assert.Contains(t, e.Body.Fields, "socials[1].url") }},
		{"not_found", 404, []error{core.ErrNotFound}, nil, nil},
		{"not_enrolled", 403, []error{ErrForbidden}, nil, func(t *testing.T, e *Error) { assert.Contains(t, e.Body.EnrollURL, "set-36") }},
		{"pair_code_expired", 410, []error{ErrGone}, nil, nil},
		{"pair_code_invalid", 404, []error{core.ErrNotFound}, nil, nil},
		{"server_error", 500, []error{ErrServer}, []error{ErrOffline}, nil},
		{"upgrade_required", 426, []error{core.ErrUnsupported, errors.ErrUnsupported}, nil, func(t *testing.T, e *Error) {
			assert.Contains(t, e.Body.StoreURL, "ios")
		}},
	} {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()

			body := mockFile(t, "errors/"+tc.code+".json")
			c := newTestClient(t, always(tc.status, string(body)))
			err := ignore(c.Config(context.Background()))

			var e *Error
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tc.code, e.Code)
			assert.Equal(t, tc.status, e.Status)
			assert.NotEmpty(t, e.Message)
			for _, s := range tc.is {
				assert.ErrorIs(t, err, s)
			}
			for _, s := range tc.isNot {
				assert.NotErrorIs(t, err, s)
			}
			if tc.check != nil {
				tc.check(t, e)
			}
		})
	}
}

func TestErrorsWithoutBody(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		body   string
		code   string
		is     error
	}{
		{502, "<html>bad gateway</html>", "server_error", ErrServer},
		{500, "", "server_error", ErrServer},
		{503, `{"error":{}}`, "server_error", ErrServer},
		{404, "gone", "http_404", core.ErrNotFound},
		{400, "{", "http_400", core.ErrInvalid},
		{418, "teapot", "http_418", nil},
		{429, "", "http_429", ErrRateLimited},
		{401, "", "http_401", ErrUnauthorized},
	} {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t, always(tc.status, tc.body))
			err := ignore(c.Config(context.Background()))
			var e *Error
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tc.code, e.Code)
			if tc.is != nil {
				assert.ErrorIs(t, err, tc.is)
			}
		})
	}
}

func TestOfflineMapping(t *testing.T) {
	t.Parallel()

	timeout := &url.Error{Op: "Get", URL: "https://x.test/", Err: context.DeadlineExceeded}
	for _, tc := range []struct {
		name    string
		err     error
		offline bool
	}{
		{"url error", &url.Error{Op: "Get", URL: "https://x.test/", Err: errors.New("connection refused")}, true},
		{"dns", &net.DNSError{Err: "no such host", Name: "x.test"}, true},
		{"op error", &net.OpError{Op: "dial", Err: errors.New("down")}, true},
		{"deadline", timeout, true},
		{"bare deadline", context.DeadlineExceeded, true},
		{"cut body", fmt.Errorf("read: %w", io.ErrUnexpectedEOF), true},
		{"canceled", context.Canceled, false},
		{"url canceled", &url.Error{Op: "Get", URL: "https://x.test/", Err: context.Canceled}, false},
		{"other", errors.New("boom"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := &scriptTransport{fn: func(int, *Request) (*Response, error) { return nil, tc.err }}
			c := newTestClient(t, tr)
			err := ignore(c.Config(context.Background()))
			require.Error(t, err)
			assert.Equal(t, tc.offline, errors.Is(err, ErrOffline))
			assert.Equal(t, tc.offline, errors.Is(err, core.ErrClosed))
			assert.ErrorIs(t, err, tc.err, "the cause stays reachable")
		})
	}
}

func TestOfflineNeverContainsSecrets(t *testing.T) {
	t.Parallel()

	tr := &scriptTransport{fn: func(int, *Request) (*Response, error) { return nil, errors.New("down") }}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = &tokenSource{token: "secret-token"} })
	err := ignore(c.Me(context.Background()))
	assert.NotContains(t, err.Error(), "secret-token")
}

// TestErrorsAreLowercase: messages follow the code conventions.
func TestErrorsAreLowercase(t *testing.T) {
	t.Parallel()

	for _, err := range []error{ErrOffline, ErrUnauthorized, ErrSignedOut, ErrForbidden, ErrConflict, ErrGone, ErrLocked, ErrRateLimited, ErrServer,
		&Error{Status: 404, Code: "not_found", Message: "Không tìm thấy"}} {
		s := err.Error()
		assert.True(t, unicode.IsLower(rune(s[0])), s)
		assert.NotEqual(t, '.', rune(s[len(s)-1]), s)
		assert.NotContains(t, s, "Không", "the server message is not part of Error()")
	}
}

func TestErrorIsHTTPStatusOnly(t *testing.T) {
	t.Parallel()

	e := &Error{Status: http.StatusTeapot, Code: "http_418"}
	assert.NoError(t, e.Unwrap())
	assert.False(t, errors.Is(e, ErrServer))
}

// TestSentinelIdentity checks that the status sentinels are distinct and
// that none of them matches a core sentinel by accident.
func TestSentinelIdentity(t *testing.T) {
	t.Parallel()

	all := []error{ErrUnauthorized, ErrForbidden, ErrConflict, ErrGone, ErrLocked, ErrRateLimited, ErrServer}
	cores := []error{core.ErrClosed, core.ErrNotFound, core.ErrInvalid, core.ErrUnsupported, core.ErrDuplicate}
	for i, e := range all {
		t.Run(e.Error(), func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, fmt.Errorf("wrap: %w", e), e)
			for j, o := range all {
				if i != j {
					assert.NotErrorIs(t, e, o)
				}
			}
			for _, c := range cores {
				assert.NotErrorIs(t, e, c)
			}
		})
	}
	assert.ErrorIs(t, ErrOffline, core.ErrClosed)
	assert.ErrorIs(t, ErrSignedOut, ErrUnauthorized)
}
