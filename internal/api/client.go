package api

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// TokenSource gives the access token and renews it. auth.Manager implements
// it; a nil TokenSource makes every call anonymous.
type TokenSource interface {
	// Token returns the current access token, "" when signed out.
	Token() string
	// Refresh renews the session after failed was refused with
	// 401 token_expired, and returns the new access token. It returns
	// ErrSignedOut when the refresh token is refused.
	Refresh(ctx context.Context, failed string) (string, error)
}

// Options configures New.
type Options struct {
	// Transport carries the requests (Live, mock or Hybrid); required. It
	// owns the base URL.
	Transport Transport
	// Version is the app version, sent as X-App-Version; required.
	Version string
	// Platform is "ios", "android" or "tvos", sent as X-Platform; required.
	Platform string
	// InstallID is the id of this install, sent as X-Install-Id.
	InstallID string
	// Tokens supplies the access token; nil for anonymous.
	Tokens TokenSource
	// Cache keeps ETag answers; nil turns the cache off.
	Cache Cache
	// OnNetwork is called when the client learns the network went down
	// (false) or came back (true); only on a change, the first state is
	// online. It runs on the calling goroutine.
	OnNetwork func(online bool)
}

// Client is the client of the DanceFITVN API: one method per route. It is
// safe for concurrent use.
type Client struct {
	tr        Transport
	version   string
	platform  string
	installID string
	tokens    TokenSource
	cache     Cache
	onNetwork func(online bool)

	mu      sync.Mutex
	offline bool
	flights map[string]*flight
}

// flight is one refresh in progress, shared by every call that failed with
// the same token.
type flight struct {
	done  chan struct{}
	token string
	err   error
}

// call describes one request of a route.
type call struct {
	id          string
	pathID      string
	query       url.Values
	body        []byte
	contentType string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// nfcStrings is the option of Decode: a JSON string decoded into a string
// comes out composed; any other JSON value (null, a number, an error)
// goes to the default decoding, untouched (errors.ErrUnsupported).
var nfcStrings = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, s *string) error {
	if dec.PeekKind() != '"' {
		return errors.ErrUnsupported
	}
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	*s = format.NFC(tok.String())

	return nil
}))

// New returns a Client. It fails with core.ErrInvalid when the transport,
// the version or the platform is missing or wrong.
func New(o Options) (*Client, error) {
	switch {
	case o.Transport == nil:
		return nil, fmt.Errorf("api: transport required: %w", core.ErrInvalid)
	case o.Version == "":
		return nil, fmt.Errorf("api: version required: %w", core.ErrInvalid)
	case o.Platform != "ios" && o.Platform != "android" && o.Platform != "tvos":
		return nil, fmt.Errorf("api: platform must be ios, android or tvos: %w", core.ErrInvalid)
	}

	return &Client{
		tr:        o.Transport,
		version:   o.Version,
		platform:  o.Platform,
		installID: o.InstallID,
		tokens:    o.Tokens,
		cache:     o.Cache,
		onNetwork: o.OnNetwork,
		flights:   map[string]*flight{},
	}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Login signs in (auth.login). The device fields left empty are the
// client's own.
func (c *Client) Login(ctx context.Context, email, password string, dev model.DeviceInfo) (model.Session, error) {
	return c.LoginReplacing(ctx, email, password, dev, "")
}

// LoginReplacing is Login for an account at its limit of devices: replaceID is
// one of the devices of the 409 device_limit answer, which the server revokes
// once the password is verified before it completes the login. An empty
// replaceID is Login's body, without the field.
func (c *Client) LoginReplacing(ctx context.Context, email, password string, dev model.DeviceInfo, replaceID string) (model.Session, error) {
	return send[model.Session](ctx, c, call{id: "auth.login", body: mustJSON(model.LoginRequest{
		Email: email, Password: password, Device: c.device(dev), ReplaceDeviceID: replaceID,
	})})
}

// Refresh trades a refresh token for new tokens (auth.refresh). It is the
// TokenSource's tool: the client never calls it by itself.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (model.Tokens, error) {
	return send[model.Tokens](ctx, c, call{id: "auth.refresh", body: mustJSON(model.RefreshRequest{
		RefreshToken: refreshToken, InstallID: c.installID,
	})})
}

// Logout ends the session on the server (auth.logout).
func (c *Client) Logout(ctx context.Context) error {
	_, err := c.round(ctx, call{id: "auth.logout"})

	return err
}

// Forgot asks for a password reset mail (auth.forgot).
func (c *Client) Forgot(ctx context.Context, email string) (model.OKMessage, error) {
	return send[model.OKMessage](ctx, c, call{id: "auth.forgot", body: mustJSON(model.ForgotRequest{Email: email})})
}

// Me returns the user (me.get).
func (c *Client) Me(ctx context.Context) (model.Me, error) {
	return send[model.Me](ctx, c, call{id: "me.get"})
}

// UpdateMe changes the profile (me.update).
func (c *Client) UpdateMe(ctx context.Context, r model.MeUpdateRequest) (model.MeUpdate, error) {
	return send[model.MeUpdate](ctx, c, call{id: "me.update", body: mustJSON(r)})
}

// ChangePassword changes the password (me.password).
func (c *Client) ChangePassword(ctx context.Context, r model.PasswordRequest) (model.PasswordResult, error) {
	return send[model.PasswordResult](ctx, c, call{id: "me.password", body: mustJSON(r)})
}

// UploadAvatar sends a picture as multipart/form-data, field "file", with
// PUT /me/avatar (me.avatar). name is the file name the server sees.
func (c *Client) UploadAvatar(ctx context.Context, image []byte, name string) (model.Avatar, error) {
	body, ctype, err := multipartFile(image, name)
	if err != nil {
		return model.Avatar{}, err
	}

	return send[model.Avatar](ctx, c, call{id: "me.avatar", body: body, contentType: ctype})
}

// CoachProfile returns the coach profile (me.coach.get).
func (c *Client) CoachProfile(ctx context.Context) (model.CoachProfile, error) {
	return send[model.CoachProfile](ctx, c, call{id: "me.coach.get"})
}

// UpdateCoachProfile changes the coach profile (me.coach.update).
func (c *Client) UpdateCoachProfile(ctx context.Context, r model.CoachUpdateRequest) (model.CoachProfile, error) {
	return send[model.CoachProfile](ctx, c, call{id: "me.coach.update", body: mustJSON(r)})
}

// Home returns the home screen data (me.home).
func (c *Client) Home(ctx context.Context) (model.Home, error) {
	return send[model.Home](ctx, c, call{id: "me.home"})
}

// MyCourses returns one page of the courses of the user (me.courses). An
// empty cursor is the first page; limit 1..50, other values give the server
// default of 20 (nothing is sent).
func (c *Client) MyCourses(ctx context.Context, cursor string, limit int) (model.CoursePage, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit >= 1 && limit <= 50 {
		q.Set("limit", strconv.Itoa(limit))
	}

	return send[model.CoursePage](ctx, c, call{id: "me.courses", query: q})
}

// Course returns one course with its lessons (courses.show).
func (c *Client) Course(ctx context.Context, id string) (model.Course, error) {
	return send[model.Course](ctx, c, call{id: "courses.show", pathID: id})
}

// Stream returns the HLS address of a lesson (lessons.stream).
func (c *Client) Stream(ctx context.Context, lessonID string) (model.Stream, error) {
	return send[model.Stream](ctx, c, call{id: "lessons.stream", pathID: lessonID})
}

// Audio returns the audio file of a lesson (lessons.audio).
func (c *Client) Audio(ctx context.Context, lessonID string) (model.Audio, error) {
	return send[model.Audio](ctx, c, call{id: "lessons.audio", pathID: lessonID})
}

// SaveProgress saves the position of a lesson (lessons.progress).
func (c *Client) SaveProgress(ctx context.Context, lessonID string, r model.ProgressRequest) (model.ProgressResult, error) {
	return send[model.ProgressResult](ctx, c, call{id: "lessons.progress", pathID: lessonID, body: mustJSON(r)})
}

// SendProgressBatch sends the positions saved offline (progress.batch).
func (c *Client) SendProgressBatch(ctx context.Context, r model.BatchRequest) (model.BatchResult, error) {
	return send[model.BatchResult](ctx, c, call{id: "progress.batch", body: mustJSON(r)})
}

// Devices lists the devices of the user (devices.list).
func (c *Client) Devices(ctx context.Context) (model.DeviceList, error) {
	return send[model.DeviceList](ctx, c, call{id: "devices.list"})
}

// DeleteDevice removes a device (devices.delete).
func (c *Client) DeleteDevice(ctx context.Context, id string) error {
	_, err := c.round(ctx, call{id: "devices.delete", pathID: id})

	return err
}

// StartPairing asks a TV code (tv.pair.start). The device fields left empty
// are the client's own.
func (c *Client) StartPairing(ctx context.Context, dev model.DeviceInfo) (model.PairStart, error) {
	return send[model.PairStart](ctx, c, call{id: "tv.pair.start", body: mustJSON(model.PairStartRequest{Device: c.device(dev)})})
}

// PollPairing asks whether the code was confirmed (tv.pair.poll).
func (c *Client) PollPairing(ctx context.Context, code, pollSecret string) (model.PairPoll, error) {
	return send[model.PairPoll](ctx, c, call{id: "tv.pair.poll", body: mustJSON(model.PairPollRequest{Code: code, PollSecret: pollSecret})})
}

// ConfirmPairing confirms a TV code from a signed-in phone (tv.pair.confirm).
func (c *Client) ConfirmPairing(ctx context.Context, code string) (model.PairConfirm, error) {
	return send[model.PairConfirm](ctx, c, call{id: "tv.pair.confirm", body: mustJSON(model.PairConfirmRequest{Code: code})})
}

// LogoutTV signs the TV out (tv.logout).
func (c *Client) LogoutTV(ctx context.Context) error {
	_, err := c.round(ctx, call{id: "tv.logout"})

	return err
}

// Config returns the remote configuration (app.config).
func (c *Client) Config(ctx context.Context) (model.AppConfig, error) {
	return send[model.AppConfig](ctx, c, call{id: "app.config"})
}

// Decode unmarshals JSON data into out as json.Unmarshal does, every
// string value in Unicode normal form C (format.NFC): course and lesson
// text typed in some editors comes decomposed (NFD), which Android's text
// engine draws with its marks beside their letters or as boxes. That holds
// for map keys and the strings of an any too; strings of named types decode
// as they are.
func Decode(data []byte, out any) error {
	return json.Unmarshal(data, out, nfcStrings)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// send runs a call and decodes its JSON answer, every string in NFC
// (Decode).
func send[T any](ctx context.Context, c *Client, cl call) (T, error) {
	var out T

	resp, err := c.round(ctx, cl)
	if err != nil {
		return out, err
	}
	if err := Decode(resp.Body, &out); err != nil {
		return out, fmt.Errorf("api: %s: unreadable answer: %w: %w", cl.id, ErrServer, err)
	}

	return out, nil
}

// mustJSON encodes a request body. Request types are plain structs, so a
// failure is a bug.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v, json.FormatNilSliceAsNull(false), json.FormatNilMapAsNull(false))
	if err != nil {
		panic("api: encode request: " + err.Error())
	}

	return b
}

// device fills the empty fields of dev with the client's own.
func (c *Client) device(dev model.DeviceInfo) model.DeviceInfo {
	if dev.Platform == "" {
		dev.Platform = c.platform
	}
	if dev.AppVersion == "" {
		dev.AppVersion = c.version
	}
	if dev.InstallID == "" {
		dev.InstallID = c.installID
	}

	return dev
}

// round sends a call, renews the token once on 401 token_expired, and turns
// an error answer into *Error. The response it returns has a 2xx status.
func (c *Client) round(ctx context.Context, cl call) (*Response, error) {
	route, ok := RouteByID(cl.id)
	if !ok {
		return nil, fmt.Errorf("api: unknown route %q: %w", cl.id, core.ErrInvalid)
	}
	path := route.Path
	if strings.Contains(path, "{id}") {
		if cl.pathID == "" {
			return nil, fmt.Errorf("api: %s: id required: %w", cl.id, core.ErrInvalid)
		}
		path = strings.Replace(path, "{id}", url.PathEscape(cl.pathID), 1)
	}

	tok := ""
	if route.Auth && c.tokens != nil {
		tok = c.tokens.Token()
	}

	resp, err := c.once(ctx, route, path, cl, tok)
	if err != nil {
		return nil, err
	}

	if resp.Status == http.StatusUnauthorized && route.Auth && c.tokens != nil && errorCode(resp) == "token_expired" {
		fresh, rerr := c.refresh(ctx, tok)
		if rerr != nil {
			if isNetwork(rerr) && !errors.Is(rerr, ErrOffline) {
				rerr = offline(rerr)
			}

			return nil, fmt.Errorf("api: refresh: %w", rerr)
		}

		resp, err = c.once(ctx, route, path, cl, fresh)
		if err != nil {
			return nil, err
		}
		if resp.Status == http.StatusUnauthorized {
			return nil, fmt.Errorf("api: %s: %w: %w", cl.id, ErrSignedOut, newError(resp))
		}
	}

	if resp.Status < 200 || resp.Status > 299 {
		return nil, newError(resp)
	}

	return resp, nil
}

// refresh renews the token that failed. Calls that failed with the same
// token share one Refresh; a call that failed with an older token takes the
// current one. The leader's context governs the shared call.
func (c *Client) refresh(ctx context.Context, failed string) (string, error) {
	c.mu.Lock()
	if cur := c.tokens.Token(); cur != "" && cur != failed {
		c.mu.Unlock()

		return cur, nil
	}
	if f, ok := c.flights[failed]; ok {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.token, f.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	f := &flight{done: make(chan struct{})}
	c.flights[failed] = f
	c.mu.Unlock()

	f.token, f.err = c.tokens.Refresh(ctx, failed)

	c.mu.Lock()
	delete(c.flights, failed)
	c.mu.Unlock()
	close(f.done)

	return f.token, f.err
}

// once sends the request with token tok and applies the ETag cache.
func (c *Client) once(ctx context.Context, route Route, path string, cl call, tok string) (*Response, error) {
	req := &Request{
		RouteID: route.ID,
		Method:  route.Method,
		Path:    path,
		Query:   cl.query,
		Header:  c.header(route, cl, tok),
		Body:    cl.body,
	}

	var key, etag string
	var cached []byte
	var hit bool
	if route.Cacheable && c.cache != nil {
		key = cacheKey(route.Method, path, cl.query)
		if etag, cached, hit = c.cache.Get(key); hit && etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
	}

	resp, err := c.tr.Do(ctx, req)
	if err != nil {
		if isNetwork(err) {
			c.setOnline(false)
			if hit {
				return &Response{Status: http.StatusOK, Body: cached}, nil
			}

			return nil, offline(err)
		}
		if errors.Is(err, context.Canceled) {
			return nil, err
		}

		return nil, fmt.Errorf("api: %s: %w", route.ID, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("api: %s: no response: %w", route.ID, ErrServer)
	}
	c.setOnline(true)

	switch {
	case resp.Status == http.StatusNotModified && hit:
		return &Response{Status: http.StatusOK, Header: resp.Header, Body: cached}, nil
	case resp.Status == http.StatusNotModified:
		return nil, fmt.Errorf("api: %s: 304 without a cached answer: %w", route.ID, ErrServer)
	case resp.Status == http.StatusOK && key != "":
		if tag := resp.Header.Get("ETag"); tag != "" {
			_ = c.cache.Put(key, tag, resp.Body) // best effort
		}
	}

	return resp, nil
}

// ClientHeader returns the headers every request of the app carries: Accept,
// Accept-Language, X-App-Version, X-Platform and, when set, X-Install-Id.
func ClientHeader(version, platform, installID string) http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json")
	h.Set("Accept-Language", "vi")
	h.Set("X-App-Version", version)
	h.Set("X-Platform", platform)
	if installID != "" {
		h.Set("X-Install-Id", installID)
	}

	return h
}

// header builds the headers of one request.
func (c *Client) header(route Route, cl call, tok string) http.Header {
	h := ClientHeader(c.version, c.platform, c.installID)
	if route.Auth && tok != "" {
		h.Set("Authorization", "Bearer "+tok)
	}
	if len(cl.body) > 0 {
		ct := cl.contentType
		if ct == "" {
			ct = "application/json"
		}
		h.Set("Content-Type", ct)
	}

	return h
}

// setOnline records the network state and reports a change.
func (c *Client) setOnline(online bool) {
	c.mu.Lock()
	changed := c.offline == online
	c.offline = !online
	c.mu.Unlock()

	if changed && c.onNetwork != nil {
		c.onNetwork(online)
	}
}
