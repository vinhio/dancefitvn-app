// Adapted from jplayer-tv/internal/media/avf/errors_test.go at 781bee5.

package avftv

import (
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/media"
)

func TestStreamErrorClass(t *testing.T) {
	t.Parallel()

	const (
		url  = "NSURLErrorDomain"
		cf   = "kCFErrorDomainCFNetwork"
		cm   = "CoreMediaErrorDomain"
		avfd = "AVFoundationErrorDomain"
	)
	cases := []struct {
		name      string
		domain    string
		code      int
		under     string
		underCode int
		class     error
		legacy    error
	}{
		{"timed out", url, -1001, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"host not found", url, -1003, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"refused", url, -1004, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"connection lost", url, -1005, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"DNS lookup failed", url, -1006, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"no connection", url, -1009, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"no data allowed", url, -1020, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"CFNetwork timed out", cf, -1001, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"CFNetwork no connection", cf, -1009, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"CFNetwork host not found", cf, 1, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"CFNetwork host unknown", cf, 2, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"CFNetwork HTTP connection lost", cf, 302, "", 0, media.ErrNetwork, core.ErrNotFound},
		{"refused under AVFoundation", avfd, -11800, url, -1004, media.ErrNetwork, core.ErrNotFound},
		{"timeout under AVFoundation", avfd, -11800, url, -1001, media.ErrNetwork, core.ErrNotFound},
		{"network over CoreMedia", url, -1009, cm, -12645, media.ErrNetwork, core.ErrNotFound},
		{"HTTP 404, as measured", url, -1100, cm, -12938, media.ErrSource, core.ErrNotFound},
		{"HTTP 404 under AVFoundation", avfd, -11800, cm, -12938, media.ErrSource, core.ErrNotFound},
		{"HTTP 403 from CoreMedia", avfd, -11800, cm, -12660, media.ErrSource, core.ErrUnsupported},
		{"not a playlist, as measured", url, -1002, cm, -1002, media.ErrSource, core.ErrNotFound},
		{"bad URL", url, -1000, "", 0, media.ErrSource, core.ErrNotFound},
		{"ATS refused, as measured", url, -1022, "", 0, media.ErrSource, core.ErrNotFound},
		{"bad server response", url, -1011, "", 0, media.ErrSource, core.ErrNotFound},
		{"CFNetwork HTTP parse failure", cf, 303, "", 0, media.ErrSource, core.ErrNotFound},
		{"CoreMedia decode", avfd, -11800, cm, -12642, media.ErrSource, core.ErrUnsupported},
		{"decoder", avfd, -11821, "", 0, media.ErrSource, core.ErrUnsupported},
		{"no error read", "unknown", 0, "", 0, media.ErrSource, core.ErrUnsupported},
		{"empty domain", "", 0, "", 0, media.ErrSource, core.ErrUnsupported},
		{"a URL code in another domain", "evil://host.example/x", -1009, "", 0, media.ErrSource, core.ErrUnsupported},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			err := streamError(c.domain, c.code, c.under, c.underCode)
			require.ErrorIs(t, err, c.class)
			require.ErrorIs(t, err, c.legacy, "the sentinel from before T13 stays in the chain")
			other := media.ErrSource
			if c.class == media.ErrSource {
				other = media.ErrNetwork
			}
			assert.NotErrorIs(t, err, other, "exactly one class")
			assert.NotContains(t, err.Error(), "://")
		})
	}
}

func TestStreamErrorTextNamesTheClassOnce(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		text string
	}{
		{"network", streamError(domainURL, -1009, "", 0),
			"avf: stream failed: NSURLErrorDomain -1009: media: network error: gdesk: not found"},
		{"source, not found", streamError("AVFoundationErrorDomain", -11800, domainCoreMedia, -12938),
			"avf: stream failed: AVFoundationErrorDomain -11800 (CoreMediaErrorDomain -12938): " +
				"media: cannot open the source: gdesk: invalid argument: gdesk: not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.text, c.err.Error())
			assert.Equal(t, 1, strings.Count(c.err.Error(), "media: "))
		})
	}
}

func TestMediaErrorClasses(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, media.ErrNetwork, core.ErrNotFound)
	require.ErrorIs(t, media.ErrSource, core.ErrInvalid)
	assert.NotErrorIs(t, media.ErrNetwork, media.ErrSource)
	assert.NotErrorIs(t, media.ErrSource, media.ErrNetwork)
	assert.NotErrorIs(t, media.ErrNetwork, core.ErrUnsupported)
	assert.NotErrorIs(t, media.ErrSource, core.ErrUnsupported)
}
