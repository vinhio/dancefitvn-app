package pairing_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/pairing"
)

func TestPhoneNormalises(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    string
		invalid bool
	}{
		{"7k4-m2p", "7K4M2P", false},
		{"7K4M2P", "7K4M2P", false},
		{" 7k4 m2p ", "7K4M2P", false},
		{"7K4-M2P-", "7K4M2P", false},
		{"7K4M2", "", true},
		{"7K4M2PX", "", true},
		{"", "", true},
		{"7K4_M2P", "", true},
		{"7K4.M2P", "", true},
		{"7K4M2É", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := pairing.NormalizeCode(tc.in)
			if tc.invalid {
				require.ErrorIs(t, err, core.ErrInvalid)

				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPhoneErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		scenario string
		input    string
		wantCode string
		is       error
		sends    int
	}{
		{"confirmed", "default", "7k4-m2p", "", nil, 1},
		{"unknown code", "invalid_code", "7K4M2P", "pair_code_invalid", core.ErrNotFound, 1},
		{"device limit", "device_limit", "7K4M2P", "device_limit", api.ErrConflict, 1},
		{"bad input sends nothing", "default", "12", "", core.ErrInvalid, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := mock.New(os.DirFS(mockDir), mock.Options{
				Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = m.Close() })
			require.NoError(t, m.SetScenario("tv.pair.confirm", tc.scenario))
			sp := &spy{inner: m}
			c, err := api.New(api.Options{Transport: sp, Version: "1.0.0", Platform: "ios", InstallID: "i-1", Tokens: staticToken{}})
			require.NoError(t, err)

			res, err := pairing.NewPhone(c).Confirm(context.Background(), tc.input)
			require.Equal(t, tc.sends, sp.count("tv.pair.confirm"))
			if tc.is == nil {
				require.NoError(t, err)
				require.Equal(t, "confirmed", res.Status.String())
				require.NotEmpty(t, res.Device.ID)

				return
			}
			require.ErrorIs(t, err, tc.is)
			if tc.wantCode != "" {
				var ae *api.Error
				require.ErrorAs(t, err, &ae)
				require.Equal(t, tc.wantCode, ae.Code)
			}
		})
	}
}

func TestPhoneSendsNormalisedCode(t *testing.T) {
	t.Parallel()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	var body []byte
	sp := &spy{inner: m, hook: func(_ context.Context, _ int, req *api.Request) (*api.Response, error, bool) {
		body = req.Body

		return nil, nil, false
	}}
	c, err := api.New(api.Options{Transport: sp, Version: "1.0.0", Platform: "ios", InstallID: "i-1", Tokens: staticToken{}})
	require.NoError(t, err)
	_, err = pairing.NewPhone(c).Confirm(context.Background(), "7k4-m2p")
	require.NoError(t, err)
	require.JSONEq(t, `{"code":"7K4M2P"}`, string(body))
}
