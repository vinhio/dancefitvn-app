package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockMediaScheme is the scheme of the media URLs in the mock's files
// (mock-media://avatars/u_412.jpg): the mock replaces it with the address of
// its media server, whose port changes at every launch.
const mockMediaScheme = "mock-media://"

// mediaServer is the mock's media server (mock.Server): where the
// mock-media:// URLs point in this run.
type mediaServer interface {
	Ensure() error
	BaseURL() string
}

// overlayTags is the transport between devTransport and the mock's overlay
// (store.Overlay), the upper of the two edges around it.
//
// An answer the overlay changed carries no ETag (its body is not the
// mock's), so the API client would keep none of them in its cache and the
// app would have nothing to show offline (03 section 2: Home, the courses
// and the detail read from the cache). overlayTags gives every 200 of a GET
// that has no ETag one made of its body, and answers 304 when the
// request's If-None-Match is that tag, as a live server does.
//
// The overlay keeps the media URLs as the mock's files write them
// (plainMedia, below it), so that what it saved one launch still points at
// the media server the next, on another port: overlayTags puts this run's
// address back. It is safe for concurrent use.
type overlayTags struct {
	inner api.Transport
	media mediaServer // nil: no media URL to put back
}

// plainMedia is the lower edge, between the overlay and the mock: it turns
// the media URLs of the mock's answers back into mock-media:// ones, which
// the overlay records (an avatar's URL, the course of "Tiếp tục tập").
type plainMedia struct {
	inner api.Transport
	media mediaServer
}

var (
	_ api.Transport = overlayTags{}
	_ api.Transport = plainMedia{}
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do asks inner, puts this run's media address in its answer and tags it.
func (t overlayTags) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	resp, err := t.inner.Do(ctx, req)
	if err != nil || resp == nil {
		return resp, err
	}
	if t.media != nil && bytes.Contains(resp.Body, []byte(mockMediaScheme)) {
		if err := t.media.Ensure(); err != nil {
			return nil, fmt.Errorf("app: mock media: %w", err)
		}
		resp = withBody(resp, bytes.ReplaceAll(resp.Body, []byte(mockMediaScheme), []byte(t.media.BaseURL()+"/")))
	}
	if req.Method != http.MethodGet || resp.Status != http.StatusOK || resp.Header.Get("ETag") != "" {
		return resp, nil
	}
	tag := contentTag(resp.Body)
	h := resp.Header.Clone()
	if h == nil {
		h = http.Header{}
	}
	h.Set("ETag", tag)
	if req.Header.Get("If-None-Match") == tag {
		return &api.Response{Status: http.StatusNotModified, Header: h}, nil
	}

	return &api.Response{Status: resp.Status, Header: h, Body: resp.Body}, nil
}

// Do asks inner and writes its media URLs as the mock's files do.
func (t plainMedia) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	resp, err := t.inner.Do(ctx, req)
	if err != nil || resp == nil {
		return resp, err
	}
	base := t.media.BaseURL()
	if base == "" || !bytes.Contains(resp.Body, []byte(base+"/")) {
		return resp, nil
	}

	return withBody(resp, bytes.ReplaceAll(resp.Body, []byte(base+"/"), []byte(mockMediaScheme))), nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// contentTag is the ETag of body: the first 8 bytes of its SHA-256, quoted.
func contentTag(body []byte) string {
	sum := sha256.Sum256(body)

	return `"ov-` + hex.EncodeToString(sum[:8]) + `"`
}

// withBody returns a copy of resp with body.
func withBody(resp *api.Response, body []byte) *api.Response {
	return &api.Response{Status: resp.Status, Header: resp.Header, Body: body}
}
