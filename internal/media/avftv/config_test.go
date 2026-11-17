// Adapted from jplayer-tv/internal/media/avf/config_test.go at 781bee5.

package avftv

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/clock"
)

func TestLimitsOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		q       Quality
		w, h, b float64
	}{
		{QualityUnknown, 0, 0, 0},
		{QualityAuto, 0, 0, 0},
		{Quality1080, 1920, 1080, 0},
		{Quality720, 1280, 720, 0},
		{Quality480, 854, 480, 0},
		{Quality(99), 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.q.String(), func(t *testing.T) {
			t.Parallel()

			w, h, b := limitsOf(c.q)
			assert.InDelta(t, c.w, w, 0)
			assert.InDelta(t, c.h, h, 0)
			assert.InDelta(t, c.b, b, 0)
			if c.q.MaxHeight() > 0 {
				assert.InDelta(t, float64(c.q.MaxHeight()), h, 0, "the height is the quality's")
			}
		})
	}
}

func TestStreamErrorHasNoURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		domain    string
		code      int
		under     string
		underCode int
		want      error
		text      string
	}{
		{"host not found", "NSURLErrorDomain", -1003, "", 0, core.ErrNotFound, "avf: stream failed: NSURLErrorDomain -1003"},
		{"refused under AVFoundation", "AVFoundationErrorDomain", -11800, "NSURLErrorDomain", -1004, core.ErrNotFound,
			"avf: stream failed: AVFoundationErrorDomain -11800 (NSURLErrorDomain -1004)"},
		{"HTTP 404 from CoreMedia", "AVFoundationErrorDomain", -11800, "CoreMediaErrorDomain", -12938, core.ErrNotFound,
			"avf: stream failed: AVFoundationErrorDomain -11800 (CoreMediaErrorDomain -12938)"},
		{"CFNetwork", "kCFErrorDomainCFNetwork", 2, "", 0, core.ErrNotFound, "avf: stream failed: kCFErrorDomainCFNetwork 2"},
		{"not a playlist, as measured", "NSURLErrorDomain", -1002, "CoreMediaErrorDomain", -1002, core.ErrNotFound,
			"avf: stream failed: NSURLErrorDomain -1002 (CoreMediaErrorDomain -1002)"},
		{"another CoreMedia error", "AVFoundationErrorDomain", -11800, "CoreMediaErrorDomain", -12642, core.ErrUnsupported,
			"avf: stream failed: AVFoundationErrorDomain -11800 (CoreMediaErrorDomain -12642)"},
		{"decoder", "AVFoundationErrorDomain", -11821, "", 0, core.ErrUnsupported, "avf: stream failed: AVFoundationErrorDomain -11821"},
		{"a domain carrying a URL", "evil://host.example/path?q=1", 1, "", 0, core.ErrUnsupported, "avf: stream failed: evilhost.examplepathq1 1"},
		{"empty domain", "", 0, "", 0, core.ErrUnsupported, "avf: stream failed: unknown 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			err := streamError(c.domain, c.code, c.under, c.underCode)
			require.Error(t, err)
			require.ErrorIs(t, err, c.want)
			assert.True(t, strings.HasPrefix(err.Error(), c.text), err.Error())
			assert.NotContains(t, err.Error(), "://")
			other := core.ErrUnsupported
			if errors.Is(c.want, core.ErrUnsupported) {
				other = core.ErrNotFound
			}
			assert.NotErrorIs(t, err, other)
		})
	}
}

func TestCleanDomainIsBounded(t *testing.T) {
	t.Parallel()

	assert.Len(t, cleanDomain(strings.Repeat("a", 500)), maxDomain)
}

func TestConfigNormalize(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(time.Unix(0, 0))
	post := func(func()) bool { return true }
	under := func(uintptr) (Plane, error) { return nil, core.ErrUnsupported }
	cases := []struct {
		name string
		cfg  Config
		err  error
	}{
		{"no clock", Config{Post: post, Underlay: under}, core.ErrInvalid},
		{"no post", Config{Clock: clk, Underlay: under}, core.ErrInvalid},
		{"no underlay", Config{Clock: clk, Post: post}, core.ErrInvalid},
		{"complete", Config{Clock: clk, Post: post, Underlay: under}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := c.cfg.normalize()
			if c.err != nil {
				require.ErrorIs(t, err, c.err)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got.Log, "a default logger")
		})
	}
}

func TestQualityString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		q    Quality
		want string
		h    int
	}{
		{QualityUnknown, "unknown", 0}, {QualityAuto, "auto", 0}, {Quality1080, "1080", 1080},
		{Quality720, "720", 720}, {Quality480, "480", 480}, {Quality(42), "unknown", 0},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, c.q.String())
			assert.Equal(t, c.h, c.q.MaxHeight())
		})
	}
}
