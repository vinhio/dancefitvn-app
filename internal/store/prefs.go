package store

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DefaultSpeed is the video speed when none was ever chosen (K41): 1x, with
// the mirror and the beat bar off.
const DefaultSpeed = 1.0

// PrefsKey is the Backend key of the preferences: <data>/prefs.json
// (docs/plan/05-data.md §6).
const PrefsKey = "prefs"

// Quality is the video quality preference (prototype quality cycle,
// reference/prototype/index.html:610 and :748).
type Quality int

// The qualities. QualityUnknown is any text the app does not know; it reads
// as QualityAuto.
const (
	QualityUnknown Quality = iota
	QualityAuto
	QualityP1080
	QualityP720
	QualityP480
)

// Prefs are the preferences of the user. The defaults are the prototype's
// (:285): Wi-Fi only, auto next, quality auto; WiFiOnly starts as the
// remote configuration says (AppConfig.Download.WiFiOnlyDefault).
type Prefs struct {
	WiFiOnly bool    `json:"wifi_only"`
	AutoNext bool    `json:"auto_next"`
	Quality  Quality `json:"quality"`
	// Speed, Mirror and Beat are the last video options the user chose
	// (K41): the speed (0.5 to 1.5), the mirror and the beat bar. They are
	// never the A–B loop or the audio repeat, which are specific to a
	// position or a track.
	Speed  float64 `json:"speed"`
	Mirror bool    `json:"mirror"`
	Beat   bool    `json:"beat"`
	// Scenarios is the mock scenario of each route (route id to name), the
	// dev menu's choice; empty in the live app.
	Scenarios map[string]string `json:"scenarios,omitzero"`
}

// PrefsStore keeps Prefs on a Backend. It is safe for concurrent use; the
// Backend may be slow on its first write, so call Update off the UI thread
// when that matters.
type PrefsStore struct {
	b Backend

	mu      sync.Mutex
	p       Prefs
	loadErr error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// DefaultPrefs returns the preferences of a first launch.
func DefaultPrefs(wifiOnlyDefault bool) Prefs {
	return Prefs{WiFiOnly: wifiOnlyDefault, AutoNext: true, Quality: QualityAuto, Speed: DefaultSpeed}
}

// OpenPrefs loads the preferences from b. Nothing stored, or a stored record
// that cannot be read, gives DefaultPrefs (LoadErr tells the second case);
// fields the record lacks keep their defaults.
func OpenPrefs(b Backend, wifiOnlyDefault bool) *PrefsStore {
	s := &PrefsStore{b: b, p: DefaultPrefs(wifiOnlyDefault)}
	raw, err := b.Load(PrefsKey)
	switch {
	case errors.Is(err, core.ErrNotFound):
	case err != nil:
		s.loadErr = fmt.Errorf("store: prefs: %w", err)
	default:
		p := DefaultPrefs(wifiOnlyDefault)
		if err := json.Unmarshal(raw, &p); err != nil {
			s.loadErr = fmt.Errorf("store: prefs: %w: %w", core.ErrInvalid, err)
		} else {
			s.p = p
		}
	}
	s.p.Quality = s.p.Quality.orAuto()
	s.p.Speed = validSpeed(s.p.Speed)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Get returns a copy of the preferences.
func (s *PrefsStore) Get() Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clonePrefs(s.p)
}

// Update applies fn to a copy of the preferences, keeps the result and saves
// it. When the save fails the new values stay in memory and the error comes
// back.
func (s *PrefsStore) Update(fn func(*Prefs)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := clonePrefs(s.p)
	fn(&p)
	p.Quality = p.Quality.orAuto()
	p.Speed = validSpeed(p.Speed)
	s.p = p
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("store: prefs: %w: %w", core.ErrInvalid, err)
	}
	if err := s.b.Save(PrefsKey, raw); err != nil {
		return fmt.Errorf("store: prefs: %w", err)
	}

	return nil
}

// LoadErr is why the stored preferences were not used at opening; nil when
// they were, or none were stored.
func (s *PrefsStore) LoadErr() error { return s.loadErr }

// NextMobile is the quality after q on a phone: Tự động, 1080p, 720p, 480p,
// and round (prototype :610).
func (q Quality) NextMobile() Quality {
	switch q.orAuto() {
	case QualityAuto:
		return QualityP1080
	case QualityP1080:
		return QualityP720
	case QualityP720:
		return QualityP480
	case QualityP480:
		return QualityAuto
	case QualityUnknown:
		return QualityP1080
	default:
		return QualityAuto
	}
}

// NextTV is the quality after q on the TV: Tự động, 1080p, 720p, and round
// (prototype :748). A stored 480p goes to Tự động.
func (q Quality) NextTV() Quality {
	switch q.orAuto() {
	case QualityAuto:
		return QualityP1080
	case QualityP1080:
		return QualityP720
	case QualityP720, QualityP480:
		return QualityAuto
	case QualityUnknown:
		return QualityP1080
	default:
		return QualityAuto
	}
}

// Height is the video height the quality asks for; 0 for automatic.
func (q Quality) Height() int {
	switch q {
	case QualityP1080:
		return 1080
	case QualityP720:
		return 720
	case QualityP480:
		return 480
	case QualityUnknown, QualityAuto:
		return 0
	default:
		return 0
	}
}

// String returns the stored text: "auto", "1080p", "720p", "480p"; "" for
// QualityUnknown. The text on screen ("Tự động") is the locale's.
func (q Quality) String() string {
	switch q {
	case QualityAuto:
		return "auto"
	case QualityP1080:
		return "1080p"
	case QualityP720:
		return "720p"
	case QualityP480:
		return "480p"
	case QualityUnknown:
		return ""
	default:
		return ""
	}
}

// MarshalText implements encoding.TextMarshaler; QualityUnknown is written
// as "auto".
func (q Quality) MarshalText() ([]byte, error) { return []byte(q.orAuto().String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler; a text the app does not
// know is QualityUnknown, not an error.
func (q *Quality) UnmarshalText(b []byte) error {
	switch string(b) {
	case "auto":
		*q = QualityAuto
	case "1080p":
		*q = QualityP1080
	case "720p":
		*q = QualityP720
	case "480p":
		*q = QualityP480
	default:
		*q = QualityUnknown
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// orAuto reads QualityUnknown as QualityAuto.
func (q Quality) orAuto() Quality {
	if q == QualityUnknown {
		return QualityAuto
	}

	return q
}

// validSpeed keeps a stored speed in the sheet's range 0.5 to 1.5; anything
// else (absent, NaN, out of range) is DefaultSpeed.
func validSpeed(v float64) float64 {
	if !(v >= 0.5 && v <= 1.5) {
		return DefaultSpeed
	}

	return v
}

func clonePrefs(p Prefs) Prefs {
	p.Scenarios = maps.Clone(p.Scenarios)

	return p
}
