package mock

import (
	"io/fs"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	files, err := subMedia()
	require.NoError(t, err)
	s := NewServer(files, nil)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	return s
}

func TestServerLoopbackOnly(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	assert.Empty(t, s.BaseURL())
	assert.Zero(t, s.Port())
	require.NoError(t, s.Ensure())
	require.NoError(t, s.Ensure()) // idempotent
	assert.Equal(t, uint64(1), s.Epoch())
	assert.Equal(t, "http://127.0.0.1:"+strconv.Itoa(s.Port()), s.BaseURL())
	ln := listenerOf(s)
	assert.Equal(t, "127.0.0.1", ln.Addr().(*net.TCPAddr).IP.String())
}

func TestServerReListensOnSamePort(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	require.NoError(t, s.Ensure())
	port, url := s.Port(), s.BaseURL()+"/hls/l_3503/master.m3u8"
	resp, _ := fetch(t, "GET", url, nil)
	require.Equal(t, 200, resp.StatusCode)

	// the system closes the socket in the background.
	require.NoError(t, listenerOf(s).Close())
	waitDead(t, s)
	require.NoError(t, s.Ensure())
	assert.Equal(t, port, s.Port())
	assert.Equal(t, uint64(1), s.Epoch(), "the same port keeps the old URLs right")
	resp, _ = fetch(t, "GET", url, nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Positive(t, s.Requests())
}

func TestServerTakesNewPortWhenTaken(t *testing.T) {
	t.Parallel()
	// Another parallel test may take the freed port before the thief does;
	// then the attempt is repeated on a fresh server, never skipped.
	for range 20 {
		s := newServer(t)
		require.NoError(t, s.Ensure())
		port := s.Port()
		require.NoError(t, listenerOf(s).Close())
		// the serving goroutine has seen the socket die: until then the
		// probe in Ensure could find the thief's socket alive.
		waitDead(t, s)
		// somebody else takes the port.
		thief, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			continue
		}
		defer func() { _ = thief.Close() }()
		require.NoError(t, s.Ensure())
		assert.NotEqual(t, port, s.Port())
		assert.Equal(t, uint64(2), s.Epoch())
		resp, _ := fetch(t, "GET", s.BaseURL()+"/hls/l_3503/master.m3u8", nil)
		assert.Equal(t, 200, resp.StatusCode)

		return
	}
	t.Fatal("the freed port was always taken by another test")
}

func TestServerClose(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	require.NoError(t, s.Close()) // before listening
	assert.ErrorIs(t, s.Ensure(), core.ErrClosed)

	s2 := newServer(t)
	require.NoError(t, s2.Ensure())
	url := s2.BaseURL()
	require.NoError(t, s2.Close())
	require.NoError(t, s2.Close())
	_, err := noKeepAliveClient().Get(url)
	assert.Error(t, err)
	assert.ErrorIs(t, s2.Ensure(), core.ErrClosed)
}

func TestServerConcurrentEnsure(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, s.Ensure())
			resp, err := noKeepAliveClient().Get(s.BaseURL() + "/audio/_sample.mp3")
			if assert.NoError(t, err) {
				_ = resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, uint64(1), s.Epoch())
}

func TestServerFailingProbeKeepsServingListener(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	require.NoError(t, s.Ensure())
	s.probe = func(int) bool { return false } // every probe fails, as under load
	ln := listenerOf(s)

	// a request is in flight: the listener must stay.
	s.inflight.Add(1)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, s.Ensure())
			resp, err := noKeepAliveClient().Get(s.BaseURL() + "/audio/_sample.mp3")
			if assert.NoError(t, err) {
				_ = resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	assert.Same(t, ln, listenerOf(s), "the serving listener was kept")
	assert.Equal(t, uint64(1), s.Epoch())
	s.inflight.Add(-1)

	// nothing in flight and the socket does not answer: listens again, same port.
	port := s.Port()
	require.NoError(t, s.Ensure())
	assert.NotSame(t, ln, listenerOf(s))
	assert.Equal(t, port, s.Port())
	s.probe = nil
	resp, err := noKeepAliveClient().Get(s.BaseURL() + "/audio/_sample.mp3")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTransportCloseThenMedia(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	require.NoError(t, tr.Close())
	_, err := tr.Do(t.Context(), get("/lessons/l_3503/stream"))
	assert.ErrorIs(t, err, core.ErrClosed)
	// bodies without media still answer.
	resp, err := tr.Do(t.Context(), get("/app/config"))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)
}

// listenerOf returns the live listener of s.
// waitDead yields until the server's goroutine has noticed its listener
// closed (no sleep: it only needs the scheduler).
func waitDead(t *testing.T, s *Server) {
	t.Helper()
	for range 1_000_000 {
		s.mu.Lock()
		dead := s.dead
		s.mu.Unlock()
		if dead {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("the server did not notice its socket die")
}

func listenerOf(s *Server) net.Listener {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ln
}

func subMedia() (fs.FS, error) { return fs.Sub(os.DirFS(mockDir), "media") }
