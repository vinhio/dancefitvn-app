package mock

import "strings"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// maxIDLen is the longest value {id} takes.
const maxIDLen = 64

// match finds the route of a request (rule 1): the method and the path with
// the base path cut off; the query is not part of p. It returns the value of
// {id}, "" when the route has none. A {id} is one segment of letters,
// digits, "_" and "-": nothing else gets into a file name.
func (t *table) match(method, p string) (*Route, string, bool) {
	p = strings.TrimPrefix(p, t.BasePath)
	if !strings.HasPrefix(p, "/") {
		return nil, "", false
	}
	segs := strings.Split(p[1:], "/")
	for i := range t.Routes {
		r := &t.Routes[i]
		if r.Method != method || len(r.segments) != len(segs) {
			continue
		}
		if id, ok := matchSegments(r.segments, segs); ok {
			return r, id, true
		}
	}

	return nil, "", false
}

// matchSegments compares the segments of a pattern with those of a path.
func matchSegments(pattern, segs []string) (string, bool) {
	id := ""
	for i, ps := range pattern {
		switch {
		case ps == "{id}":
			if !validID(segs[i]) {
				return "", false
			}
			id = segs[i]
		case ps != segs[i]:
			return "", false
		}
	}

	return id, true
}

// validID reports whether s is 1 to 64 characters of [A-Za-z0-9_-].
func validID(s string) bool {
	if s == "" || len(s) > maxIDLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
		if !ok {
			return false
		}
	}

	return true
}
