package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShapeTable(t *testing.T) {
	t.Parallel()

	type want struct {
		path string
		sev  severity
		msg  string
	}
	tests := []struct {
		name string
		mock string
		got  string
		want []want
	}{
		{name: "same shape other values", mock: `{"a":"x","b":1,"c":true,"d":[1],"e":{"f":null}}`, got: `{"a":"y","b":2.5,"c":false,"d":[7,8],"e":{"f":"any"}}`},
		{name: "missing key", mock: `{"a":1,"b":2}`, got: `{"a":1}`, want: []want{{"$.b", sevError, "missing key"}}},
		{name: "missing nested key", mock: `{"a":{"b":{"c":1}}}`, got: `{"a":{"b":{}}}`, want: []want{{"$.a.b.c", sevError, "missing key"}}},
		{name: "wrong kind string for number", mock: `{"n":1}`, got: `{"n":"1"}`, want: []want{{"$.n", sevError, "kind string, want number"}}},
		{name: "wrong kind number for bool", mock: `{"b":true}`, got: `{"b":0}`, want: []want{{"$.b", sevError, "kind number, want bool"}}},
		{name: "null for string is wrong", mock: `{"s":"x"}`, got: `{"s":null}`, want: []want{{"$.s", sevError, "kind null, want string"}}},
		{name: "object for array", mock: `{"a":[1]}`, got: `{"a":{}}`, want: []want{{"$.a", sevError, "kind object, want array"}}},
		{name: "mock null is a wildcard", mock: `{"t":null}`, got: `{"t":{"deep":[1,2]}}`},
		{name: "mock null accepts null", mock: `{"t":null}`, got: `{"t":null}`},
		{name: "mock empty array is a wildcard", mock: `{"a":[]}`, got: `{"a":[{"x":1},"y"]}`},
		{name: "mock empty array wants an array", mock: `{"a":[]}`, got: `{"a":"s"}`, want: []want{{"$.a", sevError, "kind string, want array"}}},
		{name: "array compares the first element", mock: `{"a":[{"k":1}]}`, got: `{"a":[{"k":2},{"other":1}]}`},
		{name: "array first element missing key", mock: `{"a":[{"k":1}]}`, got: `{"a":[{}]}`, want: []want{{"$.a[0].k", sevError, "missing key"}}},
		{name: "nested arrays", mock: `{"a":[{"b":[{"c":"x"}]}]}`, got: `{"a":[{"b":[{"c":1}]}]}`, want: []want{{"$.a[0].b[0].c", sevError, "kind number, want string"}}},
		{name: "extra key is a warning", mock: `{"a":1}`, got: `{"a":1,"z":2}`, want: []want{{"$.z", sevWarning, "extra key"}}},
		{name: "extra key in array element", mock: `{"a":[{"k":1}]}`, got: `{"a":[{"k":1,"z":null}]}`, want: []want{{"$.a[0].z", sevWarning, "extra key"}}},
		{name: "empty server array is a warning", mock: `{"a":[{"k":1}]}`, got: `{"a":[]}`, want: []want{{"$.a", sevWarning, "empty array, elements not compared"}}},
		{name: "top level kind", mock: `{}`, got: `[]`, want: []want{{"$", sevError, "kind array, want object"}}},
		{name: "top level array", mock: `[{"a":1}]`, got: `[{"a":1}]`},
		{name: "error and warning together", mock: `{"a":1,"b":2}`, got: `{"a":"x","c":1}`, want: []want{{"$.a", sevError, "kind string, want number"}, {"$.b", sevError, "missing key"}, {"$.c", sevWarning, "extra key"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := compareBodies([]byte(tc.mock), []byte(tc.got))
			var have []want
			for _, f := range got {
				have = append(have, want{f.Path, f.Sev, f.Msg})
			}
			assert.ElementsMatch(t, tc.want, have)
		})
	}
}

func TestShapeBadJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, mock, got, msg string
	}{
		{"body not json", `{}`, `<html>`, "body is not json"},
		{"body empty", `{}`, ``, "body is not json"},
		{"mock not json", `{`, `{}`, "mock file is not json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := compareBodies([]byte(tc.mock), []byte(tc.got))
			if assert.Len(t, got, 1) {
				assert.Equal(t, sevError, got[0].Sev)
				assert.Equal(t, tc.msg, got[0].Msg)
			}
		})
	}
}

func TestKindOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   any
		want string
	}{
		{nil, "null"}, {"s", "string"}, {1.5, "number"}, {true, "bool"},
		{[]any{}, "array"}, {map[string]any{}, "object"}, {int64(1), "unknown"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, kindOf(tc.in))
	}
}
