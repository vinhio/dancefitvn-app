package mock

import (
	"io"
	"net/http"
	"strconv"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// maxRequestBody caps the body the handler reads.
const maxRequestBody = 8 << 20

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Handler returns the mock API as an http.Handler: the paths below the base
// path ("/api/v1/me") answer as Do does. It is what contractcheck -self and
// end-to-end tests call over HTTP. It does not serve the media; the Server
// does.
func (t *Transport) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.serveAPI(w, r, t) })
}

// HandlerOver is Handler with the requests going through next (an Overlay
// in front of t, say) instead of t itself. Each request carries the id of
// its route, as the client's do, which the Overlay needs. It is what
// cmd/mockserve serves.
func (t *Transport) HandlerOver(next api.Transport) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.serveAPI(w, r, next) })
}

// RouteID returns the id of the route of a request, "" when none matches.
func (t *Transport) RouteID(method, path string) string {
	r, _, ok := t.table.match(method, path)
	if !ok {
		return ""
	}

	return r.ID
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// serveAPI turns an HTTP request into an api.Request and back.
func (t *Transport) serveAPI(w http.ResponseWriter, r *http.Request, next api.Transport) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)

		return
	}
	if len(body) == 0 {
		body = nil
	}
	method := r.Method
	if method == http.MethodHead { // answered as GET, without the body
		method = http.MethodGet
	}
	resp, err := next.Do(r.Context(), &api.Request{
		RouteID: t.RouteID(method, r.URL.Path),
		Method:  method,
		Path:    r.URL.Path,
		Query:   r.URL.Query(),
		Header:  r.Header,
		Body:    body,
	})
	if err != nil {
		if r.Context().Err() == nil {
			http.Error(w, "mock unavailable", http.StatusServiceUnavailable)
		}

		return
	}
	h := w.Header()
	for k, v := range resp.Header {
		h[k] = v
	}
	if resp.Status != http.StatusNoContent && resp.Status != http.StatusNotModified {
		h.Set("Content-Length", strconv.Itoa(len(resp.Body)))
	}
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(resp.Body)
	}
}
