// Adapted from jplayer-mobile/internal/media/enginekit/errors.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/media"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Phase is how far a load has come, which decides what a failure means: as
// in a browser, whose media element reports every failure before it has the
// metadata as MEDIA_ERR_SRC_NOT_SUPPORTED (player.js:29).
type Phase uint8

// The phases of a load.
const (
	PhaseUnknown Phase = iota
	// PhaseOpening is before the media is ready (AVPlayerItemStatusReadyToPlay,
	// MediaPlayer's onPrepared): every failure is a source that cannot be
	// opened.
	PhaseOpening
	// PhasePlaying is after: a failure of the network or of decoding is told
	// from the rest.
	PhasePlaying
)

// The domains of the errors AVFoundation reports.
const (
	NSURLErrorDomain        = "NSURLErrorDomain"
	AVFoundationErrorDomain = "AVFoundationErrorDomain"
	NSOSStatusErrorDomain   = "NSOSStatusErrorDomain"
	NSCocoaErrorDomain      = "NSCocoaErrorDomain"
	CoreMediaErrorDomain    = "CoreMediaErrorDomain"
)

// NSURLErrorAppTransportSecurityRequiresSecureConnection is the code of a
// link the app may not open for want of https (ATS).
const NSURLErrorAppTransportSecurityRequiresSecureConnection = -1022

// The codes of AVFoundationErrorDomain the engines tell apart.
const (
	AVErrorUnknown                 = -11800
	AVErrorDecodeFailed            = -11821
	AVErrorFileFormatNotRecognized = -11828
	AVErrorFailedToParse           = -11853
)

// The codes of android.media.MediaPlayer's onError: what, and extra.
const (
	MediaErrorUnknown     = 1
	MediaErrorServerDied  = 100
	MediaErrorIO          = -1004
	MediaErrorMalformed   = -1007
	MediaErrorUnsupported = -1010
	MediaErrorTimedOut    = -110
	// MediaErrorStream is the extra of MEDIA_ERROR_UNKNOWN that an HLS
	// stream reports when a playlist or a segment fails while it plays
	// (LiveSession: "Received error -1 from PlaylistFetcher"), as a link
	// that expired or a 403 does (local/youtube/spikeS1.md (h)). Not a
	// constant of the SDK: what the framework passes on of its own error.
	MediaErrorStream = -1
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AppleError turns an error of AVFoundation into one of the media sentinels
// (D12): in PhaseOpening it is always media.ErrSource; in PhasePlaying a
// network error (NSURLErrorDomain, or an AVFoundation one with a network
// error under it) is media.ErrNetwork and AVErrorDecodeFailed is
// media.ErrDecode. Anything else, and PhaseUnknown, has no sentinel, which
// the player shows as "cannot play". underDomain and underCode are those of
// the error under NSUnderlyingErrorKey, "" for none. The text tells the
// domain and code, for the log.
func AppleError(p Phase, domain string, code int, underDomain string, underCode int) error {
	detail := fmt.Sprintf("avf: %s %d", domain, code)
	if underDomain != "" {
		detail = fmt.Sprintf("%s (%s %d)", detail, underDomain, underCode)
	}
	switch p {
	case PhaseOpening:
		return fmt.Errorf("%s: %w", detail, media.ErrSource)
	case PhasePlaying:
		switch {
		case domain == AVFoundationErrorDomain && code == AVErrorDecodeFailed:
			return fmt.Errorf("%s: %w", detail, media.ErrDecode)
		case domain == NSURLErrorDomain || underDomain == NSURLErrorDomain:
			return fmt.Errorf("%s: %w", detail, media.ErrNetwork)
		}
	case PhaseUnknown:
	default:
	}

	return errors.New(detail)
}

// AndroidError turns the what and extra of MediaPlayer's onError into one of
// the media sentinels (D12): in PhaseOpening it is always media.ErrSource; in
// PhasePlaying extra MediaErrorIO or MediaErrorTimedOut, and what
// MediaErrorUnknown with extra MediaErrorStream (a playlist or a segment of
// HLS refused), is media.ErrNetwork, so that the player gets the address
// again; MediaErrorMalformed or MediaErrorUnsupported is media.ErrDecode,
// and the rest (MediaErrorServerDied included), like PhaseUnknown, has no
// sentinel.
func AndroidError(p Phase, what, extra int32) error {
	detail := fmt.Sprintf("amp: error what %d extra %d", what, extra)
	switch p {
	case PhaseOpening:
		return fmt.Errorf("%s: %w", detail, media.ErrSource)
	case PhasePlaying:
		switch {
		case extra == MediaErrorIO, extra == MediaErrorTimedOut,
			what == MediaErrorUnknown && extra == MediaErrorStream:
			return fmt.Errorf("%s: %w", detail, media.ErrNetwork)
		case extra == MediaErrorMalformed, extra == MediaErrorUnsupported:
			return fmt.Errorf("%s: %w", detail, media.ErrDecode)
		}
	case PhaseUnknown:
	default:
	}

	return errors.New(detail)
}

// OpenError tells why a load cannot go on: a local file (local) that is no
// longer at path is media.ErrMissingFile, whatever the phase and cause.
// Otherwise it is cause as the phase tells (media.ErrSource around it while
// opening, itself later), and nil when there is no cause: an engine calls it
// with a nil cause before it loads a file, to find a missing one. A
// content:// document (IsContent: a linked file on Android) is local but
// has no path to stat; a cause that tells already that a linked file is
// gone or out of reach (media.ErrMissingFile, media.ErrUnreachable, as the
// engine found by checking the link) is itself, whatever the phase, so
// that the player tells the user which.
func OpenError(p Phase, local bool, path string, cause error) error {
	if local && !IsContent(path) {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: %w", path, media.ErrMissingFile)
		}
	}
	if cause == nil {
		return nil
	}
	if errors.Is(cause, media.ErrMissingFile) || errors.Is(cause, media.ErrUnreachable) {
		return cause
	}
	if p == PhaseOpening {
		return fmt.Errorf("%w: %w", media.ErrSource, cause)
	}

	return cause
}

// IsContent tells whether url is a document of an Android content
// provider (content://, any case of the scheme): a linked file (LINKED
// §3.4), which the engine plays and probes through
// setDataSource(Context, Uri), as a local file, without a Wi-Fi lock.
func IsContent(url string) bool {
	const scheme = "content://"

	return len(url) > len(scheme) && strings.EqualFold(url[:len(scheme)], scheme)
}
