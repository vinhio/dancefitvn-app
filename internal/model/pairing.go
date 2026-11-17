package model

import (
	"fmt"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// PairStatus is the state of a TV pairing code.
type PairStatus int

// The states of a pairing code. PairUnknown is any text the app does not
// know.
const (
	PairUnknown PairStatus = iota
	PairPending
	PairConfirmed
)

// PairStart is the answer of POST /tv/pair/start. It prints as
// "model.PairStart{***}" in every verb: the poll secret is not for logs.
type PairStart struct {
	Code            string    `json:"code"`
	DisplayCode     string    `json:"display_code"`
	PollSecret      string    `json:"poll_secret"`
	PollIntervalSec int       `json:"poll_interval_sec"`
	ExpiresIn       int       `json:"expires_in"`
	ExpiresAt       time.Time `json:"expires_at,omitzero"`
	QRURL           string    `json:"qr_url"`
}

// PairPoll is the answer of POST /tv/pair/poll: only Status and ExpiresIn
// (seconds the code has left) while pending; when confirmed the fields of a
// Session as well, and ExpiresIn is the access token's. It prints as
// "model.PairPoll{***}" in every verb.
type PairPoll struct {
	Status           PairStatus `json:"status,omitzero"`
	ExpiresIn        int        `json:"expires_in,omitzero"`
	AccessToken      string     `json:"access_token,omitzero"`
	TokenType        string     `json:"token_type,omitzero"`
	RefreshToken     string     `json:"refresh_token,omitzero"`
	RefreshExpiresIn int        `json:"refresh_expires_in,omitzero"`
	User             UserRef    `json:"user,omitzero"`
	Device           DeviceRef  `json:"device,omitzero"`
}

// PairConfirm is the answer of POST /tv/pair/confirm.
type PairConfirm struct {
	Status PairStatus `json:"status,omitzero"`
	Device DeviceRef  `json:"device"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Session returns the session of a confirmed poll.
func (p PairPoll) Session() Session {
	return Session{
		Tokens: Tokens{
			AccessToken:      p.AccessToken,
			TokenType:        p.TokenType,
			ExpiresIn:        p.ExpiresIn,
			RefreshToken:     p.RefreshToken,
			RefreshExpiresIn: p.RefreshExpiresIn,
		},
		User:   p.User,
		Device: p.Device,
	}
}

// String returns the JSON text of s ("pending", "confirmed"); "" for
// PairUnknown.
func (s PairStatus) String() string {
	switch s {
	case PairPending:
		return "pending"
	case PairConfirmed:
		return "confirmed"
	case PairUnknown:
		return ""
	default:
		return ""
	}
}

// MarshalText implements encoding.TextMarshaler.
func (s PairStatus) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler; a text the app does not
// know is PairUnknown, not an error.
func (s *PairStatus) UnmarshalText(b []byte) error {
	switch string(b) {
	case "pending":
		*s = PairPending
	case "confirmed":
		*s = PairConfirmed
	default:
		*s = PairUnknown
	}

	return nil
}

// String hides the poll secret.
func (PairStart) String() string { return redacted("PairStart") }

// GoString hides the poll secret.
func (PairStart) GoString() string { return redacted("PairStart") }

// Format hides the poll secret in every verb.
func (PairStart) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("PairStart")) }

// String hides the tokens.
func (PairPoll) String() string { return redacted("PairPoll") }

// GoString hides the tokens.
func (PairPoll) GoString() string { return redacted("PairPoll") }

// Format hides the tokens in every verb.
func (PairPoll) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("PairPoll")) }
