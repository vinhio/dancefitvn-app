package main

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

// tableRows returns "route/scenario" -> verdict of the rows of the report.
func tableRows(t *testing.T, out string) map[string]string {
	t.Helper()
	rows := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || f[0] == "ROUTE" || !strings.Contains(f[0], ".") {
			continue
		}
		rows[f[0]+"/"+f[3]] = f[5]
	}

	return rows
}

// mockScenarios returns "route/scenario" of every scenario of routes.json.
func mockScenarios(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(mockDir + "/routes.json")
	require.NoError(t, err)
	var file struct {
		Routes []routeDef `json:"routes"`
	}
	require.NoError(t, json.Unmarshal(data, &file))
	var out []string
	for _, r := range file.Routes {
		for n := range r.Responses {
			out = append(out, r.ID+"/"+n)
		}
	}

	return out
}

func TestSelfPasses(t *testing.T) {
	t.Parallel()

	code, out, errOut := exec(t, "-self", "-mock", mockDir)
	require.Equal(t, exitPass, code, out+errOut)
	assert.Contains(t, out, "contractcheck self: PASS, 24 routes, 50 checks, 0 errors, 0 warnings, 0 skipped")
	assert.Empty(t, errOut)
}

func TestSelfCoversEveryRouteAndScenario(t *testing.T) {
	t.Parallel()

	code, out, _ := exec(t, "-self", "-mock", mockDir)
	require.Equal(t, exitPass, code, out)
	rows := tableRows(t, out)
	want := mockScenarios(t)
	assert.Len(t, want, 50)
	for _, key := range want {
		assert.Equal(t, "PASS", rows[key], key)
	}
	assert.Len(t, rows, len(want))
}

func TestSelfRouteTableMatchesClient(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(mockDir + "/routes.json")
	require.NoError(t, err)
	var file struct {
		Routes []routeDef `json:"routes"`
	}
	require.NoError(t, json.Unmarshal(data, &file))
	require.Len(t, api.Routes(), len(file.Routes))
	for _, r := range file.Routes {
		got, ok := api.RouteByID(r.ID)
		if assert.True(t, ok, r.ID) {
			assert.Equal(t, r.Method, got.Method, r.ID)
			assert.Equal(t, r.Path, got.Path, r.ID)
			assert.Equal(t, r.Auth, got.Auth, r.ID)
		}
	}
}

func TestSelfDetectsDrift(t *testing.T) {
	t.Parallel()

	dir := mutatedMock(t, map[string]func(any){
		"lessons/stream.json": func(v any) { delete(v.(map[string]any), "hls_url") },
		"me/courses.json": func(v any) {
			v.(map[string]any)["items"].([]any)[0].(map[string]any)["lesson_count"] = "ten"
		},
	})
	base := serveMock(t, dir, nil)
	code, out, errOut := exec(t, liveArgs(base, "-write")...)
	require.Equal(t, exitFail, code, out+errOut)
	assert.Contains(t, out, "contractcheck "+hostOf(base)+": FAIL")
	assert.Contains(t, out, "2 errors")
	assert.Contains(t, out, "error   lessons.stream [default] $.hls_url: missing key")
	assert.Contains(t, out, "error   me.courses [default] $.items[0].lesson_count: kind string, want number")
	rows := tableRows(t, out)
	assert.Equal(t, "FAIL", rows["lessons.stream/default"])
	assert.Equal(t, "FAIL", rows["me.courses/default"])
	assert.Equal(t, "PASS", rows["me.get/default"])
}

func TestDriftTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		edit  map[string]func(any)
		route string
		line  string
		sev   string
	}{
		{
			name:  "missing nested key",
			edit:  map[string]func(any){"me/me.json": func(v any) { delete(v.(map[string]any)["user"].(map[string]any), "email") }},
			route: "me.get",
			line:  "me.get [default] $.user.email: missing key",
			sev:   "error",
		},
		{
			name:  "wrong type in first array element",
			edit:  map[string]func(any){"me/devices.json": func(v any) { v.(map[string]any)["items"].([]any)[0].(map[string]any)["current"] = "yes" }},
			route: "devices.list",
			line:  "devices.list [default] $.items[0].current: kind string, want bool",
			sev:   "error",
		},
		{
			name:  "extra key is only a warning",
			edit:  map[string]func(any){"app/config.json": func(v any) { v.(map[string]any)["surprise"] = 1.0 }},
			route: "app.config",
			line:  "app.config [default] $.surprise: extra key",
			sev:   "warning",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := serveMock(t, mutatedMock(t, tc.edit), nil)
			code, out, _ := exec(t, liveArgs(base, "-write")...)
			assert.Contains(t, out, tc.sev+" ")
			assert.Contains(t, out, tc.line)
			if tc.sev == "error" {
				assert.Equal(t, exitFail, code, out)
			} else {
				assert.Equal(t, exitPass, code, out)
			}
		})
	}
}
