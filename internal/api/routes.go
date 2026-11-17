package api

import (
	"slices"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Route is one row of routes.json as the client knows it.
type Route struct {
	// ID is the route id ("me.courses").
	ID string
	// Method is the HTTP method.
	Method string
	// Path is the path below the base, with "{id}" for the one path value.
	Path string
	// Auth is true when the route needs the access token.
	Auth bool
	// Cacheable is true when a GET answer is kept under its ETag. The signed,
	// expiring URLs of lessons.stream and lessons.audio are not.
	Cacheable bool
}

// routes is the table of the 24 routes of routes.json (a test proves it).
var routes = []Route{
	{ID: "auth.login", Method: "POST", Path: "/auth/login"},
	{ID: "auth.refresh", Method: "POST", Path: "/auth/refresh"},
	{ID: "auth.logout", Method: "POST", Path: "/auth/logout", Auth: true},
	{ID: "auth.forgot", Method: "POST", Path: "/auth/forgot"},
	{ID: "me.get", Method: "GET", Path: "/me", Auth: true, Cacheable: true},
	{ID: "me.update", Method: "PATCH", Path: "/me", Auth: true},
	{ID: "me.password", Method: "PUT", Path: "/me/password", Auth: true},
	{ID: "me.avatar", Method: "PUT", Path: "/me/avatar", Auth: true},
	{ID: "me.coach.get", Method: "GET", Path: "/me/coach-profile", Auth: true, Cacheable: true},
	{ID: "me.coach.update", Method: "PUT", Path: "/me/coach-profile", Auth: true},
	{ID: "me.home", Method: "GET", Path: "/me/home", Auth: true, Cacheable: true},
	{ID: "me.courses", Method: "GET", Path: "/me/courses", Auth: true, Cacheable: true},
	{ID: "courses.show", Method: "GET", Path: "/courses/{id}", Auth: true, Cacheable: true},
	{ID: "lessons.stream", Method: "GET", Path: "/lessons/{id}/stream", Auth: true},
	{ID: "lessons.audio", Method: "GET", Path: "/lessons/{id}/audio", Auth: true},
	{ID: "lessons.progress", Method: "PUT", Path: "/lessons/{id}/progress", Auth: true},
	{ID: "progress.batch", Method: "POST", Path: "/progress/batch", Auth: true},
	{ID: "devices.list", Method: "GET", Path: "/me/devices", Auth: true, Cacheable: true},
	{ID: "devices.delete", Method: "DELETE", Path: "/me/devices/{id}", Auth: true},
	{ID: "tv.pair.start", Method: "POST", Path: "/tv/pair/start"},
	{ID: "tv.pair.poll", Method: "POST", Path: "/tv/pair/poll"},
	{ID: "tv.pair.confirm", Method: "POST", Path: "/tv/pair/confirm", Auth: true},
	{ID: "tv.logout", Method: "POST", Path: "/tv/logout", Auth: true},
	{ID: "app.config", Method: "GET", Path: "/app/config", Cacheable: true},
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Routes returns a copy of the route table.
func Routes() []Route { return slices.Clone(routes) }

// RouteByID returns the route with the id.
func RouteByID(id string) (Route, bool) {
	for _, r := range routes {
		if r.ID == id {
			return r, true
		}
	}

	return Route{}, false
}
