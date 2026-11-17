package app

import (
	"fmt"
	"strings"

	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// prefsEnv, in a developer's build, sets preferences at launch as the
// settings screen does: "quality=720p,autonext=off" (scripts/
// tvos-store-check.sh app, local/plans/P7.md P7.5c). Every launch of such a
// build then logs the preferences it reads ("dfv: prefs ...").
const prefsEnv = "DFV_PREFS"

// devPrefSet is what DFV_PREFS asks for.
type devPrefSet struct {
	quality     store.Quality // QualityUnknown: left as it is
	autoNext    bool
	setAutoNext bool
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// parseDevPrefs reads DFV_PREFS: comma-separated key=value, keys quality
// (auto, 1080p, 720p) and autonext (on, off). Anything else is
// core.ErrInvalid.
func parseDevPrefs(spec string) (devPrefSet, error) {
	var s devPrefSet
	for part := range strings.SplitSeq(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return devPrefSet{}, fmt.Errorf("app: %s: %q has no value: %w", prefsEnv, part, core.ErrInvalid)
		}
		switch k {
		case "quality":
			var q store.Quality
			_ = q.UnmarshalText([]byte(v))
			if q == store.QualityUnknown || q == store.QualityP480 {
				return devPrefSet{}, fmt.Errorf("app: %s: quality %q: %w", prefsEnv, v, core.ErrInvalid)
			}
			s.quality = q
		case "autonext":
			switch v {
			case "on":
				s.autoNext = true
			case "off":
				s.autoNext = false
			default:
				return devPrefSet{}, fmt.Errorf("app: %s: autonext %q: %w", prefsEnv, v, core.ErrInvalid)
			}
			s.setAutoNext = true
		default:
			return devPrefSet{}, fmt.Errorf("app: %s: key %q: %w", prefsEnv, k, core.ErrInvalid)
		}
	}

	return s, nil
}
