package model

import (
	"fmt"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DeviceInfo describes this device in a login or a pairing start.
type DeviceInfo struct {
	Platform   string `json:"platform"`
	Name       string `json:"name"`
	AppVersion string `json:"app_version"`
	InstallID  string `json:"install_id"`
}

// LoginRequest is the body of POST /auth/login. It prints as
// "model.LoginRequest{***}" in every verb.
type LoginRequest struct {
	Email    string     `json:"email"`
	Password string     `json:"password"`
	Device   DeviceInfo `json:"device"`
	// ReplaceDeviceID is optional (omitted when empty): the id of one of the
	// devices of a device_limit answer ("d_..."). The server acts on it only
	// after the password is verified, revokes that device (an active one of
	// the same user and the same kind) and completes the login; an id that
	// does not fit is 400 validation_failed with fields.replace_device_id.
	ReplaceDeviceID string `json:"replace_device_id,omitzero"`
}

// RefreshRequest is the body of POST /auth/refresh. It prints as
// "model.RefreshRequest{***}" in every verb.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	InstallID    string `json:"install_id"`
}

// ForgotRequest is the body of POST /auth/forgot. It prints as
// "model.ForgotRequest{***}" in every verb (an e-mail is not for logs).
type ForgotRequest struct {
	Email string `json:"email"`
}

// ProgressRequest is the body of PUT /lessons/{id}/progress.
type ProgressRequest struct {
	PositionSec int       `json:"position_sec"`
	DurationSec int       `json:"duration_sec"`
	ClientTime  time.Time `json:"client_time"`
	DeviceID    string    `json:"device_id,omitzero"`
}

// BatchItem is one lesson of a BatchRequest.
type BatchItem struct {
	LessonID    string    `json:"lesson_id"`
	PositionSec int       `json:"position_sec"`
	DurationSec int       `json:"duration_sec"`
	ClientTime  time.Time `json:"client_time"`
}

// BatchRequest is the body of POST /progress/batch.
type BatchRequest struct {
	Items []BatchItem `json:"items"`
}

// PairStartRequest is the body of POST /tv/pair/start.
type PairStartRequest struct {
	Device DeviceInfo `json:"device"`
}

// PairPollRequest is the body of POST /tv/pair/poll. It prints as
// "model.PairPollRequest{***}" in every verb.
type PairPollRequest struct {
	Code       string `json:"code"`
	PollSecret string `json:"poll_secret"`
}

// PairConfirmRequest is the body of POST /tv/pair/confirm.
type PairConfirmRequest struct {
	Code string `json:"code"`
}

// MeUpdateRequest is the body of PATCH /me.
type MeUpdateRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// PasswordRequest is the body of PUT /me/password. It prints as
// "model.PasswordRequest{***}" in every verb.
type PasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// SocialURL is one link in a CoachUpdateRequest.
type SocialURL struct {
	URL string `json:"url"`
}

// CoachUpdateRequest is the body of PUT /me/coach-profile.
type CoachUpdateRequest struct {
	Bio     string      `json:"bio"`
	Socials []SocialURL `json:"socials"`
	Places  []string    `json:"places"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String hides the e-mail and the password.
func (LoginRequest) String() string { return redacted("LoginRequest") }

// GoString hides the e-mail and the password.
func (LoginRequest) GoString() string { return redacted("LoginRequest") }

// Format hides the e-mail and the password in every verb.
func (LoginRequest) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("LoginRequest")) }

// String hides the refresh token.
func (RefreshRequest) String() string { return redacted("RefreshRequest") }

// GoString hides the refresh token.
func (RefreshRequest) GoString() string { return redacted("RefreshRequest") }

// Format hides the refresh token in every verb.
func (RefreshRequest) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("RefreshRequest")) }

// String hides the e-mail.
func (ForgotRequest) String() string { return redacted("ForgotRequest") }

// GoString hides the e-mail.
func (ForgotRequest) GoString() string { return redacted("ForgotRequest") }

// Format hides the e-mail in every verb.
func (ForgotRequest) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("ForgotRequest")) }

// String hides the poll secret.
func (PairPollRequest) String() string { return redacted("PairPollRequest") }

// GoString hides the poll secret.
func (PairPollRequest) GoString() string { return redacted("PairPollRequest") }

// Format hides the poll secret in every verb.
func (PairPollRequest) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("PairPollRequest")) }

// String hides the passwords.
func (PasswordRequest) String() string { return redacted("PasswordRequest") }

// GoString hides the passwords.
func (PasswordRequest) GoString() string { return redacted("PasswordRequest") }

// Format hides the passwords in every verb.
func (PasswordRequest) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("PasswordRequest")) }
