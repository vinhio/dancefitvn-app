package model

import "fmt"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Tokens is the token pair of a refresh (auth/refresh.json). It prints as
// "model.Tokens{***}" in every verb: tokens never reach a log.
type Tokens struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// Session is the answer of a login or of a confirmed TV pairing. It prints
// as "model.Session{***}" in every verb.
type Session struct {
	Tokens `json:",inline"`
	User   UserRef   `json:"user"`
	Device DeviceRef `json:"device"`
}

// UserRef is the short form of a user inside a Session.
type UserRef struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String hides the tokens.
func (Tokens) String() string { return redacted("Tokens") }

// GoString hides the tokens.
func (Tokens) GoString() string { return redacted("Tokens") }

// Format hides the tokens in every verb.
func (Tokens) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("Tokens")) }

// String hides the tokens.
func (Session) String() string { return redacted("Session") }

// GoString hides the tokens.
func (Session) GoString() string { return redacted("Session") }

// Format hides the tokens in every verb.
func (Session) Format(f fmt.State, _ rune) { fmt.Fprint(f, redacted("Session")) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// redacted is what a type that holds secrets prints.
func redacted(name string) string { return "model." + name + "{***}" }
