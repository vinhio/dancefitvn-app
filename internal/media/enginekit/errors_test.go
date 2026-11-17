// Adapted from jplayer-mobile/internal/media/enginekit/errors_test.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/media"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentinels are the media errors an error may wrap.
var sentinels = []error{media.ErrSource, media.ErrNetwork, media.ErrDecode, media.ErrMissingFile}

// assertWraps checks err wraps want and no other sentinel; a nil want means none.
func assertWraps(t *testing.T, err, want error) {
	t.Helper()
	require.Error(t, err)
	for _, s := range sentinels {
		if errors.Is(s, want) {
			assert.ErrorIs(t, err, s)
		} else {
			assert.NotErrorIs(t, err, s)
		}
	}
}

func TestAppleError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		phase     Phase
		domain    string
		code      int
		under     string
		underCode int
		want      error
		text      string
	}{
		{"unknown host before ready", PhaseOpening, NSURLErrorDomain, -1003, "", 0, media.ErrSource, "avf: NSURLErrorDomain -1003"},
		{"404 before ready", PhaseOpening, NSURLErrorDomain, -1100, "", 0, media.ErrSource, ""},
		{"403 before ready", PhaseOpening, NSURLErrorDomain, -1102, "", 0, media.ErrSource, ""},
		{"ATS before ready", PhaseOpening, NSURLErrorDomain, NSURLErrorAppTransportSecurityRequiresSecureConnection, "", 0, media.ErrSource, ""},
		{"general failure with a network cause before ready", PhaseOpening, AVFoundationErrorDomain, AVErrorUnknown, NSURLErrorDomain, -1003, media.ErrSource, "(NSURLErrorDomain -1003)"},
		{"decode failure before ready is still a source", PhaseOpening, AVFoundationErrorDomain, AVErrorDecodeFailed, "", 0, media.ErrSource, ""},
		{"format not recognized before ready", PhaseOpening, AVFoundationErrorDomain, AVErrorFileFormatNotRecognized, "", 0, media.ErrSource, ""},
		{"connection lost while playing", PhasePlaying, NSURLErrorDomain, -1005, "", 0, media.ErrNetwork, ""},
		{"timed out while playing", PhasePlaying, NSURLErrorDomain, -1001, "", 0, media.ErrNetwork, ""},
		{"general failure with a network cause while playing", PhasePlaying, AVFoundationErrorDomain, AVErrorUnknown, NSURLErrorDomain, -1005, media.ErrNetwork, ""},
		{"decode failed while playing", PhasePlaying, AVFoundationErrorDomain, AVErrorDecodeFailed, "", 0, media.ErrDecode, ""},
		{"decode failed wins over a network cause", PhasePlaying, AVFoundationErrorDomain, AVErrorDecodeFailed, NSURLErrorDomain, -1005, media.ErrDecode, ""},
		{"unrelated code while playing", PhasePlaying, AVFoundationErrorDomain, -11819, "", 0, nil, "avf: AVFoundationErrorDomain -11819"},
		{"general failure with an OSStatus cause", PhasePlaying, AVFoundationErrorDomain, AVErrorUnknown, NSOSStatusErrorDomain, -12345, nil, ""},
		{"same code in another domain is no decode failure", PhasePlaying, NSOSStatusErrorDomain, AVErrorDecodeFailed, "", 0, nil, ""},
		{"unknown phase", PhaseUnknown, NSURLErrorDomain, -1005, "", 0, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := AppleError(tt.phase, tt.domain, tt.code, tt.under, tt.underCode)
			assertWraps(t, err, tt.want)
			if tt.text != "" {
				assert.Contains(t, err.Error(), tt.text)
			}
		})
	}
}

func TestAndroidError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phase       Phase
		what, extra int32
		want        error
	}{
		{"io before prepared", PhaseOpening, MediaErrorUnknown, MediaErrorIO, media.ErrSource},
		{"unsupported before prepared", PhaseOpening, MediaErrorUnknown, MediaErrorUnsupported, media.ErrSource},
		{"server died before prepared", PhaseOpening, MediaErrorServerDied, 0, media.ErrSource},
		{"io after", PhasePlaying, MediaErrorUnknown, MediaErrorIO, media.ErrNetwork},
		{"timed out after", PhasePlaying, MediaErrorUnknown, MediaErrorTimedOut, media.ErrNetwork},
		{"malformed after", PhasePlaying, MediaErrorUnknown, MediaErrorMalformed, media.ErrDecode},
		{"unsupported after", PhasePlaying, MediaErrorUnknown, MediaErrorUnsupported, media.ErrDecode},
		{"server died after", PhasePlaying, MediaErrorServerDied, 0, nil},
		{"unknown after", PhasePlaying, MediaErrorUnknown, 0, nil},
		{"hls segment refused after", PhasePlaying, MediaErrorUnknown, MediaErrorStream, media.ErrNetwork},
		{"hls segment refused before prepared", PhaseOpening, MediaErrorUnknown, MediaErrorStream, media.ErrSource},
		{"extra -1 of server died after", PhasePlaying, MediaErrorServerDied, MediaErrorStream, nil},
		{"extra -1 of another what after", PhasePlaying, -38, MediaErrorStream, nil},
		{"hls segment refused, unknown phase", PhaseUnknown, MediaErrorUnknown, MediaErrorStream, nil},
		{"state error -38 after", PhasePlaying, -38, 0, nil},
		{"unknown phase", PhaseUnknown, MediaErrorUnknown, MediaErrorIO, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := AndroidError(tt.phase, tt.what, tt.extra)
			assertWraps(t, err, tt.want)
			assert.Contains(t, err.Error(), "amp: error what")
		})
	}
}

func TestOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	present := filepath.Join(dir, "clip.mp4")
	require.NoError(t, os.WriteFile(present, []byte("x"), 0o600))
	gone := filepath.Join(dir, "gone.mp4")
	cause := errors.New("boom")

	tests := []struct {
		name  string
		phase Phase
		local bool
		path  string
		cause error
		want  error // nil: the error must be nil or wrap only cause
		isNil bool
	}{
		{"missing local file", PhaseOpening, true, gone, nil, media.ErrMissingFile, false},
		{"missing local file with a cause", PhaseOpening, true, gone, cause, media.ErrMissingFile, false},
		{"missing local file while playing", PhasePlaying, true, gone, cause, media.ErrMissingFile, false},
		{"present file, nothing wrong", PhaseOpening, true, present, nil, nil, true},
		{"present file with a cause, opening", PhaseOpening, true, present, cause, media.ErrSource, false},
		{"link, nothing wrong", PhaseOpening, false, "https://x.test/a.mp4", nil, nil, true},
		{"link never stats", PhaseOpening, false, gone, nil, nil, true},
		{"link with a cause, opening", PhaseOpening, false, "https://x.test/a.mp4", cause, media.ErrSource, false},
		{"cause while playing is itself", PhasePlaying, false, "https://x.test/a.mp4", cause, nil, false},
		{"content document never stats", PhaseOpening, true, "content://com.android.externalstorage.documents/document/primary%3Agone.mp3", nil, nil, true},
		{"content document with a cause, opening", PhaseOpening, true, "content://com.android.providers.media.documents/document/audio%3A1", cause, media.ErrSource, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := OpenError(tt.phase, tt.local, tt.path, tt.cause)
			if tt.isNil {
				assert.NoError(t, err)

				return
			}
			require.Error(t, err)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
			}
			if tt.cause != nil && !errors.Is(tt.want, media.ErrMissingFile) {
				assert.ErrorIs(t, err, tt.cause)
			}
			if !errors.Is(tt.want, media.ErrMissingFile) {
				assert.NotErrorIs(t, err, media.ErrMissingFile)
			}
		})
	}
}

func TestOpenErrorMissingFile(t *testing.T) {
	t.Parallel()

	err := OpenError(PhaseOpening, true, filepath.Join(t.TempDir(), "nope.mp3"), nil)
	assert.ErrorIs(t, err, media.ErrMissingFile)
	assert.NotErrorIs(t, err, media.ErrSource)
	assert.Contains(t, err.Error(), "nope.mp3")
}

// TestOpenErrorLinked checks that a linked file found gone or out of reach
// stays so, whatever the phase: never ErrSource, which the player would
// tell first.
func TestOpenErrorLinked(t *testing.T) {
	t.Parallel()

	url := "content://com.android.providers.media.documents/document/audio%3A1"
	for _, sentinel := range []error{media.ErrMissingFile, media.ErrUnreachable} {
		for _, p := range []Phase{PhaseOpening, PhasePlaying, PhaseUnknown} {
			cause := fmt.Errorf("amp: %s: %w", url, sentinel)
			err := OpenError(p, true, url, cause)
			require.ErrorIs(t, err, sentinel)
			assert.NotErrorIs(t, err, media.ErrSource)
		}
	}
}

// TestIsContent checks the URLs that are Android documents.
func TestIsContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want bool
	}{
		{"content://com.android.externalstorage.documents/document/primary%3Aa.mp3", true},
		{"CONTENT://x/y", true},
		{"content://", false},
		{"/storage/emulated/0/Music/a.mp3", false},
		{"/data/user/0/app/files/media/content://x", false},
		{"https://example.com/content://a.mp3", false},
		{"file:///sdcard/a.mp3", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsContent(tt.url), tt.url)
	}
}
