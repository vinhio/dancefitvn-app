package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTransport is the smallest Transport; it checks the contract compiles
// for implementers.
type fakeTransport struct {
	resp *Response
	err  error
}

func (f fakeTransport) Do(ctx context.Context, _ *Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return f.resp, f.err
}

var _ Transport = fakeTransport{}

func TestTransportContract(t *testing.T) {
	t.Parallel()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	netErr := errors.New("network down")
	req := &Request{RouteID: "me.get", Method: http.MethodGet, Path: "/me"}

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		tr      Transport
		status  int
		wantErr error
	}{
		{"http error is a response", context.Background(), fakeTransport{resp: &Response{Status: 404}}, 404, nil},
		{"ok", context.Background(), fakeTransport{resp: &Response{Status: 200}}, 200, nil},
		{"transport failure", context.Background(), fakeTransport{err: netErr}, 0, netErr},
		{"canceled", canceled, fakeTransport{resp: &Response{Status: 200}}, 0, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := tc.tr.Do(tc.ctx, req)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, resp)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.status, resp.Status)
		})
	}
}
