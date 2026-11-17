package main

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestsCarryClientHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		extra    []string
		platform string
		version  string
		install  string
	}{
		{"defaults", nil, "ios", "1.0.0", ""},
		{"android", []string{"-platform", "android", "-app-version", "2.3.4", "-install-id", "abc"}, "android", "2.3.4", "abc"},
		{"tvos", []string{"-platform", "tvos"}, "tvos", "1.0.0", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var mu sync.Mutex
			seen := map[string][]http.Header{}
			base := serveMock(t, mockDir, func(_ http.ResponseWriter, r *http.Request) bool {
				mu.Lock()
				seen[r.URL.Path] = append(seen[r.URL.Path], r.Header.Clone())
				mu.Unlock()

				return false
			})
			code, _, errOut := exec(t, liveArgs(base, append([]string{"-write"}, tc.extra...)...)...)
			require.Contains(t, []int{exitPass, exitFail}, code, errOut)
			mu.Lock()
			defer mu.Unlock()
			require.NotEmpty(t, seen)
			ids := map[string]bool{}
			for path, hs := range seen {
				for _, h := range hs {
					want := tc.platform
					if path == apiPath+"/tv/pair/start" || path == apiPath+"/tv/pair/poll" {
						want = "tvos"
					}
					assert.Equal(t, want, h.Get("X-Platform"), path)
					assert.Equal(t, tc.version, h.Get("X-App-Version"), path)
					assert.Equal(t, "vi", h.Get("Accept-Language"), path)
					assert.NotEmpty(t, h.Get("X-Install-Id"), path)
					if tc.install != "" {
						assert.Equal(t, tc.install, h.Get("X-Install-Id"), path)
					}
					ids[h.Get("X-Install-Id")] = true
				}
			}
			assert.Len(t, ids, 1, "install id stable within a run")
		})
	}
}
