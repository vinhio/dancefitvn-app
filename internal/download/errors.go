package download

import (
	"errors"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of this package.
var (
	// ErrNoSpace is a device with less free space than the file needs
	// (Q13: "Không đủ bộ nhớ").
	ErrNoSpace = fmt.Errorf("download: not enough space: %w", core.ErrInvalid)
	// ErrChecksum is a finished file whose size or sha256 differs from the
	// server's (Q13: "Tải lỗi, thử lại"). The file is deleted.
	ErrChecksum = fmt.Errorf("download: checksum mismatch: %w", core.ErrInvalid)
	// ErrNoAudio is a lesson whose audio the server does not have: GET
	// /lessons/{id}/audio, or the file's own address, answered not_found. The
	// Manager remembers it for the session (Unavailable) and Start refuses the
	// lesson with it. It is a core.ErrNotFound too.
	ErrNoAudio = fmt.Errorf("download: lesson has no audio file: %w", core.ErrNotFound)
	// ErrWiFiOnly is what an Allowed func returns while the rule "Wi-Fi
	// only" holds and the network is not Wi-Fi.
	ErrWiFiOnly = fmt.Errorf("download: wi-fi only: %w", core.ErrUnsupported)
)

// isOffline reports whether err is a network loss (the job pauses).
func isOffline(err error) bool { return errors.Is(err, api.ErrOffline) }
