package main

import (
	"bufio"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

// listenLine is the line cmd/mockserve prints once it listens.
var listenLine = regexp.MustCompile(`API at (http://127\.0\.0\.1:\d+/api/v1) `)

// startMockserve builds cmd/mockserve, starts it on a free loopback port with
// the real mock folder and returns its base URL. The server stops with the
// test.
func startMockserve(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and starts cmd/mockserve")
	}
	goBin, err := osexec.LookPath("go")
	if err != nil {
		t.Skip("go tool not found")
	}
	bin := filepath.Join(t.TempDir(), "mockserve")
	build := osexec.Command(goBin, "build", "-o", bin, "../mockserve")
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	ctx, cancel := context.WithCancel(context.Background())
	cmd := osexec.CommandContext(ctx, bin, "-addr", "127.0.0.1:0", "-mock", mockDir)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	lines := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		if sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
		for sc.Scan() { // drain so the child never blocks
		}
	}()
	select {
	case line := <-lines:
		m := listenLine.FindStringSubmatch(line)
		require.NotNil(t, m, "unexpected first line: %s", line)

		return m[1]
	case <-time.After(30 * time.Second):
		require.FailNow(t, "mockserve did not start")

		return ""
	}
}

// TestAgainstMockserveOverHTTP is the offline proof of the tool: the whole
// plan, with -write, against cmd/mockserve (overlay and pair relay in front
// of the mock) through net/http on the loopback address.
func TestAgainstMockserveOverHTTP(t *testing.T) {
	t.Parallel()

	base := startMockserve(t)
	args := []string{"-base", base, "-mock", mockDir, "-email", testEmail, "-password", testPassword,
		"-write", "-password-new", testPassword + "-new", "-device", "d_102"}

	code, out, errOut := exec(t, args...)
	require.Equal(t, exitPass, code, "stdout:\n%s\nstderr:\n%s", out, errOut)

	rows := tableRows(t, out)
	for _, r := range api.Routes() {
		verdict, ok := rows[r.ID+"/default"]
		assert.True(t, ok, "route %s missing from the report", r.ID)
		assert.Equal(t, "PASS", verdict, "route %s", r.ID)
	}
	assert.Contains(t, out, "PASS, 24 routes")
	assert.Contains(t, out, "0 errors")
	assert.Contains(t, out, "0 skipped")
	assert.Less(t, strings.Index(out, "tv.logout"), strings.Index(out, "auth.logout"), "auth.logout runs last")
	for _, secret := range []string{testPassword, testEmail} {
		assert.NotContains(t, out+errOut, secret)
	}
}

// TestRouteFilterOverHTTP runs one route against the server: reads run as
// prerequisites but only the chosen route is reported.
func TestRouteFilterOverHTTP(t *testing.T) {
	t.Parallel()

	base := startMockserve(t)
	code, out, errOut := exec(t, "-base", base, "-mock", mockDir, "-email", testEmail, "-password", testPassword,
		"-route", "me.courses")
	require.Equal(t, exitPass, code, "stdout:\n%s\nstderr:\n%s", out, errOut)
	rows := tableRows(t, out)
	assert.Equal(t, "PASS", rows["me.courses/default"])
	assert.Len(t, rows, 1)
}
