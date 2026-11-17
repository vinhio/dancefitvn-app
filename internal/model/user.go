package model

import (
	"fmt"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// User is the profile of the signed-in user (me/me.json .user).
type User struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AvatarURL   string `json:"avatar_url"`
	MemberSince Date   `json:"member_since,omitzero"`
	Role        string `json:"role"`
}

// Date is a calendar day as the API writes it, "2024-03-12". The standard
// library's JSON has no `format` tag option, so the layout lives here.
type Date time.Time

// Me is the answer of GET /me.
type Me struct {
	User     User        `json:"user"`
	Badges   []Badge     `json:"badges"`
	Subjects []string    `json:"subjects"`
	Devices  DeviceCount `json:"devices"`
}

// DeviceCount is the device usage of the user.
type DeviceCount struct {
	Active int         `json:"active"`
	Limit  DeviceLimit `json:"limit"`
}

// MeUpdate is the answer of PATCH /me; EmailChangePending is nil when the
// e-mail did not change.
type MeUpdate struct {
	User               User    `json:"user"`
	EmailChangePending *string `json:"email_change_pending"`
}

// Badge is a badge of the user.
type Badge struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}

// CoachProfile is the public profile of a coach (me/coach-profile.json).
type CoachProfile struct {
	Bio              string    `json:"bio"`
	Subjects         []string  `json:"subjects"`
	SubjectsEditable bool      `json:"subjects_editable"`
	Socials          []Social  `json:"socials"`
	AllowedPlatforms []string  `json:"allowed_platforms"`
	MaxSocials       int       `json:"max_socials"`
	Places           []string  `json:"places"`
	MaxPlaces        int       `json:"max_places"`
	PublicURL        string    `json:"public_url"`
	UpdatedAt        time.Time `json:"updated_at,omitzero"`
}

// Social is one social link of a coach.
type Social struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// PasswordResult is the answer of PUT /me/password.
type PasswordResult struct {
	OK             bool   `json:"ok"`
	Message        string `json:"message"`
	RevokedDevices int    `json:"revoked_devices"`
}

// OKMessage is the answer of POST /auth/forgot.
type OKMessage struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Avatar is the answer of PUT /me/avatar (me.avatar).
type Avatar struct {
	AvatarURL string `json:"avatar_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Time returns d as a time.Time (midnight UTC of the day).
func (d Date) Time() time.Time { return time.Time(d) }

// IsZero reports whether d is the zero Date; omitzero uses it.
func (d Date) IsZero() bool { return time.Time(d).IsZero() }

// String returns the day as "2024-03-12"; "" for the zero Date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}

	return time.Time(d).Format(time.DateOnly)
}

// MarshalText implements encoding.TextMarshaler.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler; "" is the zero Date.
func (d *Date) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = Date{}

		return nil
	}
	t, err := time.Parse(time.DateOnly, string(b))
	if err != nil {
		return fmt.Errorf("model: date: %w", core.ErrInvalid)
	}
	*d = Date(t)

	return nil
}
