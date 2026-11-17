package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeCalls are the requests that change data on the server; without -write
// none of them may be sent.
var writeCalls = []string{
	"PATCH /me", "PUT /me/password", "PUT /me/avatar", "PUT /me/coach-profile", "POST /progress/batch",
	"POST /tv/pair/confirm", "POST /tv/logout", "POST /auth/forgot",
}

func TestWriteRoutesNeedFlag(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	base := serveMock(t, mockDir, func(_ http.ResponseWriter, r *http.Request) bool {
		rec.add(r)

		return false
	})
	code, out, errOut := exec(t, liveArgs(base)...)
	require.Equal(t, exitPass, code, out+errOut)

	calls := rec.all()
	for _, w := range writeCalls {
		assert.NotContains(t, calls, w)
	}
	for _, c := range calls {
		assert.False(t, strings.HasPrefix(c, "PUT /lessons/"), c)
		assert.False(t, strings.HasPrefix(c, "DELETE "), c)
	}
	// the login, the reads, the pairing start and poll and the logout ran
	for _, c := range []string{"POST /auth/login", "GET /me", "POST /tv/pair/start", "POST /tv/pair/poll", "POST /auth/refresh", "POST /auth/logout"} {
		assert.Contains(t, calls, c)
	}
	rows := tableRows(t, out)
	skipped := 0
	for _, v := range rows {
		if v == "SKIP" {
			skipped++
		}
	}
	assert.Equal(t, 10, skipped, out)
	assert.Contains(t, out, "needs -write")
	assert.Contains(t, out, "0 errors")
}

func TestWriteFlagRunsWriteRoutes(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	base := serveMock(t, mockDir, func(_ http.ResponseWriter, r *http.Request) bool {
		rec.add(r)

		return false
	})
	// -password-new and -device switch on the two routes that need more
	code, out, errOut := exec(t, liveArgs(base, "-write", "-password-new", "Another-Unique-Pw-1", "-device", "d_102")...)
	require.Equal(t, exitPass, code, out+errOut)
	calls := rec.all()
	for _, w := range append(writeCalls, "DELETE /me/devices/d_102") {
		assert.Contains(t, calls, w)
	}
	assert.Contains(t, out, "0 skipped")
	assert.NotContains(t, out, "Another-Unique-Pw-1")
}

func TestWriteRoutesSkippedWithoutExtras(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, nil)
	code, out, errOut := exec(t, liveArgs(base, "-write")...)
	require.Equal(t, exitPass, code, out+errOut)
	rows := tableRows(t, out)
	assert.Equal(t, "SKIP", rows["me.password/default"])
	assert.Equal(t, "SKIP", rows["devices.delete/default"])
	assert.Contains(t, out, "needs -password-new")
	assert.Contains(t, out, "needs -device")
}

func TestRouteFilter(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, nil)
	code, out, errOut := exec(t, liveArgs(base, "-route", "me.courses", "-route", "me.update,devices.list")...)
	require.Equal(t, exitPass, code, out+errOut)
	rows := tableRows(t, out)
	assert.Equal(t, map[string]string{
		"me.courses/default":   "PASS",
		"devices.list/default": "PASS",
		"me.update/default":    "SKIP",
	}, rows)
}

func TestUnknownRoute(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, nil)
	code, _, errOut := exec(t, liveArgs(base, "-route", "me.nothing")...)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, errOut, `unknown route "me.nothing"`)
}

func TestPasswordNeverPrinted(t *testing.T) {
	t.Parallel()

	const newPassword = "Another-Unique-Pw-2"
	tests := []struct {
		name  string
		extra []string
		wrap  func(w http.ResponseWriter, r *http.Request) bool
	}{
		{name: "pass", extra: []string{"-v", "-write", "-password-new", newPassword}},
		{name: "login refused", extra: []string{"-v"}, wrap: func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != apiPath+"/auth/login" {
				return false
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			// a server that echoes secrets back in its message
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_credentials","message":"bad ` + testPassword + ` for ` + testEmail + `"}}`))

			return true
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := serveMock(t, mockDir, tc.wrap)
			_, out, errOut := exec(t, liveArgs(base, tc.extra...)...)
			for _, s := range []string{testPassword, newPassword, testEmail, "tester@"} {
				assert.NotContains(t, out, s)
				assert.NotContains(t, errOut, s)
			}
		})
	}
}

func TestReportRedacts(t *testing.T) {
	t.Parallel()

	r := newReport(nil)
	r.addSecret("secret-value")
	r.addSecret("ab") // too short to mask
	r.add(result{Route: "x.y", Method: "GET", Path: "/x", Scenario: "default", Status: 200,
		Findings: []finding{{Path: "$", Sev: sevError, Msg: "got secret-value ab"}}})
	var b strings.Builder
	require.NoError(t, r.write(&b, "self"))
	assert.NotContains(t, b.String(), "secret-value")
	assert.Contains(t, b.String(), "got *** ab")
	assert.Contains(t, b.String(), "FAIL")
}

func TestWrongStatusIsError(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != apiPath+"/me/home" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"server_error","message":"x"}}`))

		return true
	})
	code, out, _ := exec(t, liveArgs(base)...)
	assert.Equal(t, exitFail, code)
	assert.Contains(t, out, "error   me.home [default] $: status 500, want 200, error code server_error")
	assert.Equal(t, "FAIL", tableRows(t, out)["me.home/default"])
}

func TestErrorEnvelopeShapeChecked(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != apiPath+"/me/home" {
			return false
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"server_error"}}`))

		return true
	})
	code, out, _ := exec(t, liveArgs(base)...)
	assert.Equal(t, exitFail, code)
	assert.Contains(t, out, "error   me.home [default] $.error.message: missing key")
}

func TestUnknownErrorCodeWarns(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != apiPath+"/me/home" {
			return false
		}
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"error":{"code":"brew_failed","message":"m"}}`))

		return true
	})
	_, out, _ := exec(t, liveArgs(base)...)
	assert.Contains(t, out, "warning me.home [default] $.error.code: error code brew_failed has no mock file")
}

func TestBodyNotJSONIsError(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != apiPath+"/app/config" {
			return false
		}
		_, _ = w.Write([]byte("<html>bad gateway</html>"))

		return true
	})
	code, out, _ := exec(t, liveArgs(base)...)
	assert.Equal(t, exitFail, code)
	assert.Contains(t, out, "error   app.config [default] $: body is not json")
	assert.Contains(t, out, "content type is not json")
}

func TestLoginRefusedSkipsTheRest(t *testing.T) {
	t.Parallel()

	base := serveMock(t, mockDir, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != apiPath+"/auth/login" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_credentials","message":"m"}}`))

		return true
	})
	code, out, _ := exec(t, liveArgs(base)...)
	assert.Equal(t, exitFail, code)
	rows := tableRows(t, out)
	assert.Equal(t, "FAIL", rows["auth.login/default"])
	assert.Equal(t, "PASS", rows["app.config/default"])
	assert.Equal(t, "SKIP", rows["me.get/default"])
	assert.Equal(t, "SKIP", rows["auth.logout/default"])
	assert.Contains(t, out, "no session (login failed)")
}

func TestUnreachableServerExits2(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL + apiPath
	srv.Close() // nothing listens any more
	code, _, errOut := exec(t, liveArgs(base)...)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, errOut, "connection failed")
	assert.NotContains(t, errOut, testPassword)
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		msg  string
	}{
		{"no base", []string{"-mock", mockDir}, "-base or -self is required"},
		{"no credentials", []string{"-mock", mockDir, "-base", "https://example.invalid/api/v1"}, "-email and -password"},
		{"self and base", []string{"-self", "-base", "https://example.invalid/api/v1"}, "exclude each other"},
		{"plain http to a remote host", []string{"-mock", mockDir, "-base", "http://example.invalid/api/v1", "-email", "a@b.invalid", "-password", "longenough"}, "must be https"},
		{"not a url", []string{"-mock", mockDir, "-base", "ftp://x", "-email", "a@b.invalid", "-password", "longenough"}, "http or https"},
		{"missing mock folder", []string{"-self", "-mock", "does-not-exist"}, "load mock"},
		{"stray argument", []string{"-self", "extra"}, "unexpected argument"},
		{"unknown flag", []string{"-nope"}, "flag provided but not defined"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, _, errOut := exec(t, tc.args...)
			assert.Equal(t, exitUsage, code)
			assert.Contains(t, errOut, tc.msg)
		})
	}
}

func TestHelpExits0(t *testing.T) {
	t.Parallel()

	code, _, errOut := exec(t, "-h")
	assert.Equal(t, exitPass, code)
	assert.Contains(t, errOut, "-self")
}

func TestEnvCredentials(t *testing.T) {
	t.Parallel()

	env := map[string]string{envEmail: testEmail, envPassword: testPassword}
	o, code := parseFlags([]string{"-base", "https://example.invalid/api/v1"}, &strings.Builder{}, func(k string) string { return env[k] })
	assert.Equal(t, -1, code)
	assert.Equal(t, testEmail, o.Email)
	assert.Equal(t, testPassword, o.Password)

	o, code = parseFlags([]string{"-base", "https://example.invalid/api/v1", "-email", "x@y.invalid"}, &strings.Builder{}, func(k string) string { return env[k] })
	assert.Equal(t, -1, code)
	assert.Equal(t, "x@y.invalid", o.Email)
}

func TestHostOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://host.invalid:8443", hostOf("https://user:pw@host.invalid:8443/api/v1?token=abc"))
	assert.Equal(t, "server", hostOf("::bad"))
}

// TestUsageNeverPrintsEnvSecrets is not parallel: t.Setenv.
func TestUsageNeverPrintsEnvSecrets(t *testing.T) {
	const pw, em = "HUNTER2-LEAKCHECK", "leak.check@example.invalid"
	t.Setenv(envPassword, pw)
	t.Setenv(envEmail, em)

	for _, args := range [][]string{{"-h"}, {"-nope"}} {
		var out, errOut strings.Builder
		run(context.Background(), args, &out, &errOut, os.Getenv)
		for _, got := range []string{out.String(), errOut.String()} {
			assert.NotContains(t, got, pw, "args %v", args)
			assert.NotContains(t, got, em, "args %v", args)
		}
	}
}
