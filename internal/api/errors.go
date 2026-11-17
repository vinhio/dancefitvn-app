package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of this package. ErrOffline wraps core.ErrClosed and ErrSignedOut
// wraps ErrUnauthorized. The status sentinels (ErrUnauthorized, ErrForbidden,
// ErrConflict, ErrGone, ErrLocked, ErrRateLimited, ErrServer) wrap no core
// sentinel on purpose: core has none that fits, and a borrowed one would make
// errors.Is(err, core.ErrNotFound) and the like match the wrong failures.
var (
	// ErrOffline is a request that did not reach the server: no network, a
	// timeout, a connection cut. Cancellation of the context is not one.
	ErrOffline = fmt.Errorf("api: offline: %w", core.ErrClosed)
	// ErrUnauthorized is any 401.
	ErrUnauthorized = errors.New("api: unauthorized")
	// ErrSignedOut is a session the server no longer accepts: the refresh
	// token is refused, or the access token is refused again after a refresh.
	// The auth package re-exports it.
	ErrSignedOut = fmt.Errorf("api: signed out: %w", ErrUnauthorized)
	// ErrForbidden is any 403 (not_enrolled ...).
	ErrForbidden = errors.New("api: forbidden")
	// ErrConflict is any 409 (device_limit ...).
	ErrConflict = errors.New("api: conflict")
	// ErrGone is any 410 (pair_code_expired).
	ErrGone = errors.New("api: gone")
	// ErrLocked is any 423 (account_locked).
	ErrLocked = errors.New("api: locked")
	// ErrRateLimited is any 429.
	ErrRateLimited = errors.New("api: rate limited")
	// ErrServer is any 5xx, and an answer the client cannot read.
	ErrServer = errors.New("api: server error")
)

// Error is an HTTP error answer of the API (05 §3). Callers branch on Code,
// never on Message. errors.Is reaches the sentinel of the status: 404
// core.ErrNotFound, 426 core.ErrUnsupported, 400 and 413 core.ErrInvalid,
// 401 ErrUnauthorized, 403 ErrForbidden, 409 ErrConflict, 410 ErrGone, 423
// ErrLocked, 429 ErrRateLimited, 5xx ErrServer.
type Error struct {
	// Status is the HTTP status.
	Status int
	// Code is the error code of the body, "server_error" for a 5xx without
	// one, "http_<status>" for another status without one.
	Code string
	// Message is the sentence of the server; for logs of developers only.
	Message string
	// Body is the whole error object of the answer.
	Body model.APIError

	sentinel error
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Error implements error. It leaves the server's message out: the code is
// stable, the message is for people.
func (e *Error) Error() string {
	return "api: http " + strconv.Itoa(e.Status) + " " + e.Code
}

// Unwrap returns the sentinel of the status, nil when the status has none.
func (e *Error) Unwrap() error { return e.sentinel }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newError builds the *Error of a response with Status >= 300.
func newError(resp *Response) *Error {
	e := &Error{Status: resp.Status, sentinel: sentinelOf(resp.Status)}

	var body model.ErrorBody
	if err := Decode(resp.Body, &body); err == nil && body.Error.Code != "" {
		e.Body = body.Error
		e.Code = body.Error.Code
		e.Message = body.Error.Message

		return e
	}

	if resp.Status >= 500 {
		e.Code = "server_error"
	} else {
		e.Code = "http_" + strconv.Itoa(resp.Status)
	}

	return e
}

// sentinelOf returns the sentinel of an HTTP status.
func sentinelOf(status int) error {
	switch {
	case status == http.StatusBadRequest, status == http.StatusRequestEntityTooLarge:
		return core.ErrInvalid
	case status == http.StatusUnauthorized:
		return ErrUnauthorized
	case status == http.StatusForbidden:
		return ErrForbidden
	case status == http.StatusNotFound:
		return core.ErrNotFound
	case status == http.StatusConflict:
		return ErrConflict
	case status == http.StatusGone:
		return ErrGone
	case status == http.StatusLocked:
		return ErrLocked
	case status == http.StatusTooManyRequests:
		return ErrRateLimited
	case status == 426:
		return core.ErrUnsupported
	case status >= 500:
		return ErrServer
	default:
		return nil
	}
}

// errorCode returns the code of an error answer, "" when it has none.
func errorCode(resp *Response) string {
	if resp.Status < 400 {
		return ""
	}

	return newError(resp).Code
}

// offline wraps a transport failure in ErrOffline and keeps the cause.
func offline(err error) error { return fmt.Errorf("%w: %w", ErrOffline, err) }

// isNetwork reports whether err is a failure of the network (what Go reports
// in English: *url.Error, net.Error, a DNS error, a body cut short, a
// deadline). A canceled context is not one.
//
// Adapted from jplayer-mobile/internal/sources/errors.go isNetwork.
func isNetwork(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var ue *url.Error
	var ne net.Error
	var de *net.DNSError

	return errors.As(err, &ue) || errors.As(err, &ne) || errors.As(err, &de) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}
