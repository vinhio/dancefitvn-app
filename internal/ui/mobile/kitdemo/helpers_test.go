//go:build !tvos

package kitdemo_test

import (
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/ui/mobile/kitdemo"
	"dancefitvn-app/locales"

	"github.com/stretchr/testify/require"
)

// repoRoot returns the folder with go.mod above the test.
func repoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
}

// loadData loads the pages' data from the mock API folder of the repository
// (the app passes its embedded copy: the same files).
func loadData(t testing.TB) *kitdemo.Data {
	t.Helper()
	d, err := kitdemo.Load(os.DirFS(filepath.Join(repoRoot(t), "mock", "dancefitvn-mock-api")), locales.MustTexts())
	require.NoError(t, err)

	return d
}
