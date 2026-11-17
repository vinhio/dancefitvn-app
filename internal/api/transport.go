package api

import (
	"context"
	"net/http"
	"net/url"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Request is one call to the API, before the transport turns it into bytes
// on a wire (Live) or into a file of the mock API (mock).
type Request struct {
	// RouteID is the id of the route in routes.json ("me.courses").
	RouteID string
	// Method is the HTTP method.
	Method string
	// Path is relative to the base path ("/me/courses"), without query.
	Path string
	// Query holds the query parameters.
	Query url.Values
	// Header holds the request headers (authorization, ETag condition ...).
	Header http.Header
	// Body is the encoded request body; nil for none.
	Body []byte
}

// Response is the answer of the API.
type Response struct {
	// Status is the HTTP status code.
	Status int
	// Header holds the response headers.
	Header http.Header
	// Body is the response body; nil for 204 and 304.
	Body []byte
}

// Transport carries a Request to the API and brings the Response back.
//
// Do returns a non-nil error only for a failure of the transport itself
// (network down, timeout, canceled context). An HTTP error is a Response
// with Status >= 400 and a nil error. Do is safe for concurrent use.
type Transport interface {
	Do(ctx context.Context, req *Request) (*Response, error)
}
