// Command contractcheck checks a running DanceFITVN API against the mock
// API of mock/dancefitvn-mock-api (docs/plan/05-data.md §5): it logs in,
// calls the routes with the bodies of requests/ and compares the structure of
// each answer (keys and kinds, never values) with the file of the default
// scenario. With -self it serves the mock in-process and checks it against
// itself, which proves the tool; the tests run it.
//
// Exit codes: 0 pass, 1 contract errors, 2 usage or connection failure.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Exit codes.
const (
	exitPass  = 0
	exitFail  = 1
	exitUsage = 2
)

const (
	envEmail    = "DFV_TEST_EMAIL"
	envPassword = "DFV_TEST_PASSWORD"
	// selfEmail and selfPassword are the made-up account of -self; the mock
	// accepts any.
	selfEmail    = "contractcheck@example.invalid"
	selfPassword = "self-check-password"
	selfNew      = "self-check-new-password"
	apiPath      = "/api/v1"
	// defaultAppVersion is the version the app sends (internal/app appVersion).
	defaultAppVersion = "1.0.0"
)

// routeList is a repeatable -route flag.
type routeList []string

// String implements flag.Value.
func (r *routeList) String() string { return strings.Join(*r, ",") }

// Set implements flag.Value; a value may hold several ids separated by
// commas.
func (r *routeList) Set(v string) error {
	for _, id := range strings.Split(v, ",") {
		if id = strings.TrimSpace(id); id != "" {
			*r = append(*r, id)
		}
	}

	return nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv)
	stop()
	os.Exit(code)
}

// run is main without the process: it returns the exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	o, code := parseFlags(args, stderr, getenv)
	if code >= 0 {
		return code
	}

	rep := newReport(nil)
	if o.Verbose {
		rep = newReport(func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) })
	}
	var (
		tr       api.Transport
		scenario func(string, string) error
		mode     string
		cleanup  = func() {}
	)
	if o.Self {
		var err error
		if tr, scenario, cleanup, err = serveSelf(o); err != nil {
			fmt.Fprintln(stderr, "contractcheck:", err)

			return exitUsage
		}
		mode = "self"
	} else {
		var err error
		if tr, cleanup, err = dialLive(o); err != nil {
			fmt.Fprintln(stderr, "contractcheck:", err)

			return exitUsage
		}
		mode = hostOf(o.Base)
	}
	defer cleanup()

	c, err := newChecker(o, os.DirFS(o.Mock), tr, rep)
	if err != nil {
		fmt.Fprintln(stderr, "contractcheck:", rep.redact(err.Error()))

		return exitUsage
	}
	c.scenario = scenario
	if err := c.run(ctx); err != nil {
		_ = rep.write(stdout, mode)
		fmt.Fprintln(stderr, "contractcheck:", rep.redact(err.Error()))

		return exitUsage
	}
	if err := rep.write(stdout, mode); err != nil {
		fmt.Fprintln(stderr, "contractcheck:", rep.redact(err.Error()))

		return exitUsage
	}
	if !rep.passed() {
		return exitFail
	}

	return exitPass
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// parseFlags reads the command line; the code is -1 to go on, else the exit
// code.
func parseFlags(args []string, stderr io.Writer, getenv func(string) string) (options, int) {
	var o options
	var routes routeList
	fs := flag.NewFlagSet("contractcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.Mock, "mock", "mock/dancefitvn-mock-api", "mock API folder")
	fs.StringVar(&o.Base, "base", "", "base URL of the server to check, https://host/api/v1")
	fs.StringVar(&o.Email, "email", "", "test account email (default $"+envEmail+")")
	fs.StringVar(&o.Password, "password", "", "test account password (default $"+envPassword+"; never printed)")
	fs.StringVar(&o.PasswordNew, "password-new", "", "new password: enables me.password with -write (changes the password)")
	fs.StringVar(&o.DeviceID, "device", "", "device id devices.delete removes with -write")
	fs.Var(&routes, "route", "route id to report, e.g. me.courses (repeatable; reads still run as prerequisites)")
	fs.BoolVar(&o.Write, "write", false, "also run the routes that change data (test account only)")
	fs.BoolVar(&o.Self, "self", false, "serve the mock in-process and check it against itself")
	fs.StringVar(&o.Platform, "platform", "ios", "X-Platform to send: ios, android or tvos (tv.pair.start and tv.pair.poll always send tvos)")
	fs.StringVar(&o.AppVersion, "app-version", defaultAppVersion, "X-App-Version to send")
	fs.StringVar(&o.InstallID, "install-id", "", "X-Install-Id to send (default: a random UUID for this run)")
	fs.BoolVar(&o.Verbose, "v", false, "log one line per call to stderr")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, exitPass
		}

		return o, exitUsage
	}
	o.Routes = routes
	switch o.Platform {
	case "ios", "android", "tvos":
	default:
		fmt.Fprintln(stderr, "contractcheck: -platform must be ios, android or tvos")

		return o, exitUsage
	}
	if o.InstallID == "" {
		o.InstallID = newInstallID()
	}
	// The env values fill in after Parse so that usage never prints them.
	if o.Email == "" {
		o.Email = getenv(envEmail)
	}
	if o.Password == "" {
		o.Password = getenv(envPassword)
	}
	switch {
	case fs.NArg() > 0:
		fmt.Fprintln(stderr, "contractcheck: unexpected argument")

		return o, exitUsage
	case o.Self:
		if o.Base != "" {
			fmt.Fprintln(stderr, "contractcheck: -self and -base exclude each other")

			return o, exitUsage
		}
		o.Email, o.Password, o.PasswordNew, o.Write = selfEmail, selfPassword, selfNew, true
	case o.Base == "":
		fmt.Fprintln(stderr, "contractcheck: -base or -self is required")

		return o, exitUsage
	case o.Email == "" || o.Password == "":
		fmt.Fprintln(stderr, "contractcheck: -email and -password (or $"+envEmail+" and $"+envPassword+") are required")

		return o, exitUsage
	}

	return o, -1
}

// newInstallID returns a random version 4 UUID.
func newInstallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// newHTTPClient returns a client with a transport of its own, so that
// closing it leaves no connection goroutine behind.
func newHTTPClient() (*http.Client, func()) {
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}

	return &http.Client{Transport: tr, Timeout: 15 * time.Second}, tr.CloseIdleConnections
}

// serveSelf serves the mock of o.Mock over HTTP on a loopback port and
// returns a Live transport to it, the scenario selector and the cleanup.
func serveSelf(o options) (api.Transport, func(string, string) error, func(), error) {
	mt, err := mock.New(os.DirFS(o.Mock), mock.Options{
		Wait:            func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		KeepAudioDigest: true,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load mock %s: %w", o.Mock, err)
	}
	srv := httptest.NewServer(mt.Handler())
	hc, closeIdle := newHTTPClient()
	live, err := api.NewLive(api.LiveOptions{Base: srv.URL + apiPath, Client: hc})
	if err != nil {
		srv.Close()
		_ = mt.Close()

		return nil, nil, nil, fmt.Errorf("start self server: %w", err)
	}
	cleanup := func() {
		closeIdle()
		srv.Close()
		_ = mt.Close()
	}

	return live, mt.SetScenario, cleanup, nil
}

// dialLive returns a Live transport to o.Base. A password does not go over
// plain http to a host that is not on this machine.
func dialLive(o options) (api.Transport, func(), error) {
	u, err := url.Parse(o.Base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, nil, errors.New("-base must be an http or https URL")
	}
	if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return nil, nil, errors.New("-base must be https unless the host is on this machine")
	}
	hc, closeIdle := newHTTPClient()
	live, err := api.NewLive(api.LiveOptions{Base: o.Base, Client: hc})
	if err != nil {
		return nil, nil, fmt.Errorf("-base: %w", err)
	}

	return live, closeIdle, nil
}

// isLoopback reports whether host is localhost or a loopback address.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

// hostOf returns "scheme://host" of a URL, without user, path or query.
func hostOf(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return "server"
	}

	return u.Scheme + "://" + u.Host
}
