package model

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Platform is the operating system of a device.
type Platform int

// The platforms of the API. PlatformUnknown is any text the app does not
// know.
const (
	PlatformUnknown Platform = iota
	PlatformIOS
	PlatformAndroid
	PlatformTVOS
)

// Device is a device of the user (me/devices.json). The list inside a
// device_limit error carries only id, platform, name and last_seen_at.
type Device struct {
	ID         string    `json:"id"`
	Platform   Platform  `json:"platform,omitzero"`
	Name       string    `json:"name"`
	AppVersion string    `json:"app_version,omitzero"`
	LastSeenAt time.Time `json:"last_seen_at,omitzero"`
	Current    bool      `json:"current,omitzero"`
}

// DeviceList is the answer of GET /me/devices.
type DeviceList struct {
	Items []Device    `json:"items"`
	Limit DeviceLimit `json:"limit"`
}

// DeviceLimit is how many devices of each kind an account may have.
type DeviceLimit struct {
	Phone int `json:"phone"`
	TV    int `json:"tv"`
}

// DeviceRef is the short form of a device inside a Session.
type DeviceRef struct {
	ID       string   `json:"id"`
	Platform Platform `json:"platform,omitzero"`
	Name     string   `json:"name"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String returns the JSON text of p ("ios", "android", "tvos"); "" for
// PlatformUnknown.
func (p Platform) String() string {
	switch p {
	case PlatformIOS:
		return "ios"
	case PlatformAndroid:
		return "android"
	case PlatformTVOS:
		return "tvos"
	case PlatformUnknown:
		return ""
	default:
		return ""
	}
}

// MarshalText implements encoding.TextMarshaler.
func (p Platform) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler; a text the app does not
// know is PlatformUnknown, not an error.
func (p *Platform) UnmarshalText(b []byte) error {
	switch string(b) {
	case "ios":
		*p = PlatformIOS
	case "android":
		*p = PlatformAndroid
	case "tvos":
		*p = PlatformTVOS
	default:
		*p = PlatformUnknown
	}

	return nil
}
