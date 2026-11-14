package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sessionServer is a qBittorrent stand-in that is safe for concurrent
// requests. Each login issues a new SID ("s1", "s2", ...) and takes
// loginDelay, so concurrent callers that are not serialised overlap inside
// login. /app/version answers 200 only for the current SID and 403 for any
// other, the way a real server treats an expired session.
type sessionServer struct {
	logins     atomic.Int64
	loginDelay time.Duration

	mu    sync.Mutex
	valid string // SID /app/version accepts; "" accepts none
}

func (s *sessionServer) expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.valid = ""
}

func (s *sessionServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v2/auth/login":
		n := s.logins.Add(1)
		time.Sleep(s.loginDelay)
		sid := "s" + strconv.FormatInt(n, 10)
		s.mu.Lock()
		s.valid = sid
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: sid, Path: "/"})
		_, _ = w.Write([]byte("Ok."))
	case "/api/v2/app/version":
		c, err := r.Cookie("SID")
		s.mu.Lock()
		ok := err == nil && c.Value == s.valid
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("v5.2.3"))
	default:
		http.NotFound(w, r)
	}
}

// callVersionConcurrently releases n goroutines at once, each calling
// Version, and fails the test if any call errors.
func callVersionConcurrently(t *testing.T, c *HTTPClient, n int) {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			<-start
			_, err := c.Version(context.Background())
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("Version: %v", err)
		}
	}
}

func TestConcurrentCallersShareOneLogin(t *testing.T) {
	qb := &sessionServer{loginDelay: 50 * time.Millisecond}
	srv := httptest.NewServer(qb)
	defer srv.Close()
	c, err := New(srv.URL, "admin", "adminadmin")
	if err != nil {
		t.Fatal(err)
	}

	callVersionConcurrently(t, c, 8)

	if got := qb.logins.Load(); got != 1 {
		t.Fatalf("logins = %d, want 1 shared by 8 concurrent callers", got)
	}
}

func TestConcurrentCallersRenewExpiredSessionOnce(t *testing.T) {
	qb := &sessionServer{loginDelay: 50 * time.Millisecond}
	srv := httptest.NewServer(qb)
	defer srv.Close()
	c, err := New(srv.URL, "admin", "adminadmin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Version(context.Background()); err != nil {
		t.Fatalf("first Version: %v", err)
	}
	qb.expire() // every caller's next request carries a dead SID

	callVersionConcurrently(t, c, 8)

	if got := qb.logins.Load(); got != 2 {
		t.Fatalf("logins = %d, want 2 (first login + one renewal shared by 8 callers)", got)
	}
}
