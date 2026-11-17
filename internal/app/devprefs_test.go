package app

import (
	"testing"

	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDevPrefs: DFV_PREFS's keys, and what it refuses.
func TestParseDevPrefs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		spec    string
		want    devPrefSet
		wantErr bool
	}{
		{spec: "", want: devPrefSet{}},
		{spec: "quality=720p,autonext=off", want: devPrefSet{quality: store.QualityP720, setAutoNext: true}},
		{spec: " quality=1080p ", want: devPrefSet{quality: store.QualityP1080}},
		{spec: "autonext=on", want: devPrefSet{autoNext: true, setAutoNext: true}},
		{spec: "quality=auto", want: devPrefSet{quality: store.QualityAuto}},
		{spec: "quality=480p", wantErr: true},
		{spec: "quality=4k", wantErr: true},
		{spec: "autonext=yes", wantErr: true},
		{spec: "wifi=off", wantErr: true},
		{spec: "quality", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			got, err := parseDevPrefs(tc.spec)
			if tc.wantErr {
				require.ErrorIs(t, err, core.ErrInvalid)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
