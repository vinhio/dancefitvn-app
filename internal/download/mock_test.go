package download_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/download"
)

const mockDir = "../../mock/dancefitvn-mock-api"

// staticToken is a TokenSource for the mock, which wants a Bearer.
type staticToken struct{}

func (staticToken) Token() string { return "test-token" }

func (staticToken) Refresh(context.Context, string) (string, error) { return "test-token", nil }

// TestDownloadWholeFile goes end to end: the API client over the mock
// transport (audio digest patched to the real file), the loopback media
// server of the mock and the real _sample.mp3.
func TestDownloadWholeFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		lesson   string
		wantFile string // file of the mock media folder the server answers with
	}{
		{"lesson with its own file", "l_3501", "l_3501.mp3"},
		{"lesson without a file gets the sample", "l_3599", "_sample.mp3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr, err := mock.New(os.DirFS(mockDir), mock.Options{
				Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = tr.Close() })
			client, err := api.New(api.Options{Transport: tr, Version: "1.0.0", Platform: "ios", InstallID: "i-1", Tokens: staticToken{}})
			require.NoError(t, err)

			ev := newEvents()
			dir := t.TempDir()
			m, err := download.NewManager(download.Options{
				Client: client, Dir: dir, Clock: clock.NewFake(t0), UserID: "u_1", OnEvent: ev.on,
				HTTP: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}},
			})
			require.NoError(t, err)
			t.Cleanup(m.Close)

			require.NoError(t, m.Start(lesson(tc.lesson), course("set-35")))
			done := ev.wait(t, download.EventFinished, tc.lesson)
			require.Positive(t, done.Total)

			p, ok := m.Path(tc.lesson)
			require.True(t, ok)
			want, err := os.ReadFile(filepath.Join(mockDir, "media", "audio", tc.wantFile))
			require.NoError(t, err)
			require.Equal(t, want, readAll(t, p))
			e := m.List()[0]
			require.Equal(t, int64(len(want)), e.Size)
			require.Len(t, e.SHA256, 64)
		})
	}
}
