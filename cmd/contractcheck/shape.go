package main

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strconv"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// severity tells how bad a finding is.
type severity int

const (
	// sevUnknown is the zero value; it is never produced.
	sevUnknown severity = iota
	// sevWarning is a field the server has and the mock has not, or an
	// array too short to compare.
	sevWarning
	// sevError is a missing key, a wrong kind or a wrong status.
	sevError
)

// String returns "warning" or "error".
func (s severity) String() string {
	switch s {
	case sevWarning:
		return "warning"
	case sevError:
		return "error"
	case sevUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// finding is one difference between the shape of the mock and the shape of
// the server.
type finding struct {
	// Path is the place in the document, "$.items[0].cover_url".
	Path string
	// Sev is the severity.
	Sev severity
	// Msg says what differs; it never holds a value of the document.
	Msg string
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// compareShape compares the structure of got with that of want (05 §5):
// object keys and the kinds string, number, bool, null, array, object. Values
// are not compared. A null in want, and an empty array in want, accept
// anything (F-4: the mock cannot show the shape of what it leaves empty);
// arrays compare their first element. A key of want that got lacks, or a
// different kind, is an error; an extra key of got, or an empty array of got
// where want has elements, is a warning. Both arguments are decoded JSON
// (any).
func compareShape(want, got any) []finding {
	var out []finding
	walk("$", want, got, &out)

	return out
}

// compareBodies decodes two JSON documents and compares their shapes. A body
// that is not JSON is one error.
func compareBodies(want, got []byte) []finding {
	w, err := decode(want)
	if err != nil {
		return []finding{{Path: "$", Sev: sevError, Msg: "mock file is not json"}}
	}
	g, err := decode(got)
	if err != nil {
		return []finding{{Path: "$", Sev: sevError, Msg: "body is not json"}}
	}

	return compareShape(w, g)
}

// decode parses JSON into any: objects are map[string]any, arrays []any,
// numbers float64.
func decode(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	return v, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// kindOf names the kind of a decoded JSON value.
func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// walk compares one node and appends the findings to out.
func walk(path string, want, got any, out *[]finding) {
	if want == nil {
		return // null in the mock: any shape
	}
	wk, gk := kindOf(want), kindOf(got)
	if wk != gk {
		*out = append(*out, finding{Path: path, Sev: sevError, Msg: "kind " + gk + ", want " + wk})

		return
	}
	switch w := want.(type) {
	case map[string]any:
		g, _ := got.(map[string]any)
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			gv, ok := g[k]
			if !ok {
				*out = append(*out, finding{Path: path + "." + k, Sev: sevError, Msg: "missing key"})

				continue
			}
			walk(path+"."+k, w[k], gv, out)
		}
		extra := make([]string, 0)
		for k := range g {
			if _, ok := w[k]; !ok {
				extra = append(extra, k)
			}
		}
		slices.Sort(extra)
		for _, k := range extra {
			*out = append(*out, finding{Path: path + "." + k, Sev: sevWarning, Msg: "extra key"})
		}
	case []any:
		g, _ := got.([]any)
		if len(w) == 0 {
			return // empty in the mock: any element
		}
		if len(g) == 0 {
			*out = append(*out, finding{Path: path, Sev: sevWarning, Msg: "empty array, elements not compared"})

			return
		}
		walk(path+"["+strconv.Itoa(0)+"]", w[0], g[0], out)
	default:
		// string, number, bool: same kind is enough
	}
}
