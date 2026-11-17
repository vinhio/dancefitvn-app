//go:build !live

package main

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
)

func files(t *testing.T, fsys fs.FS) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[p] = info.Size()

		return nil
	}))

	return out
}

func TestMockAPIHasEverything(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"routes.json",
		"media/hls/_sample/master.m3u8",
		"media/hls/_sample/480p_000.ts",
		"media/audio/_sample.mp3",
		"media/audio/l_3503.mp3",
		"media/hls/l_3503/master.m3u8",
		"errors/token_expired.json",
		"requests/auth.login.json",
	} {
		_, err := fs.Stat(mockAPI(), name)
		assert.NoError(t, err, name)
	}
}

func TestMockAPIEqualsFolder(t *testing.T) {
	t.Parallel()
	assert.Equal(t, files(t, os.DirFS("mock/dancefitvn-mock-api")), files(t, mockAPI()))
}

func TestMockAPIRunsTheTransport(t *testing.T) {
	t.Parallel()
	tr, err := mock.New(mockAPI(), mock.Options{Wait: func(context.Context, time.Duration) error { return nil }})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })
	resp, err := tr.Do(context.Background(), &api.Request{
		Method: http.MethodGet, Path: "/lessons/l_9999/audio", Header: http.Header{"Authorization": {"Bearer t"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)
	assert.Len(t, tr.Routes(), 24)
}
