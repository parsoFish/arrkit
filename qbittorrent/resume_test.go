package qbittorrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/parsoFish/arrkit/httpx"
)

// resumeServer answers torrents/start and torrents/resume with the given
// status codes, the way a qBittorrent 5.x server (start 200, resume 404) or
// 4.x server (start 404, resume 200) does, and records the hashes each
// endpoint receives. Login always succeeds.
type resumeServer struct {
	startStatus, resumeStatus int

	mu     sync.Mutex
	hashes map[string][]string // path -> hashes form values, in order
}

func (s *resumeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v2/auth/login":
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "abc", Path: "/"})
		_, _ = w.Write([]byte("Ok."))
	case "/api/v2/torrents/start", "/api/v2/torrents/resume":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.hashes[r.URL.Path] = append(s.hashes[r.URL.Path], r.FormValue("hashes"))
		s.mu.Unlock()
		status := s.startStatus
		if r.URL.Path == "/api/v2/torrents/resume" {
			status = s.resumeStatus
		}
		w.WriteHeader(status)
	default:
		http.NotFound(w, r)
	}
}

func (s *resumeServer) seen(path string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.hashes[path]...)
}

func newResumeClient(t *testing.T, startStatus, resumeStatus int) (*HTTPClient, *resumeServer) {
	t.Helper()
	qb := &resumeServer{startStatus: startStatus, resumeStatus: resumeStatus, hashes: map[string][]string{}}
	srv := httptest.NewServer(qb)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, "admin", "adminadmin")
	if err != nil {
		t.Fatal(err)
	}
	return c, qb
}

func TestResumeUsesStartOnQBittorrent5(t *testing.T) {
	c, qb := newResumeClient(t, http.StatusOK, http.StatusNotFound)
	if err := c.Resume(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if got := qb.seen("/api/v2/torrents/start"); len(got) != 1 || got[0] != "a|b" {
		t.Errorf("torrents/start hashes = %q, want one call with a|b", got)
	}
	if got := qb.seen("/api/v2/torrents/resume"); len(got) != 0 {
		t.Errorf("torrents/resume called %d times, want 0 when start succeeds", len(got))
	}
}

func TestResumeFallsBackToResumeOnQBittorrent4(t *testing.T) {
	c, qb := newResumeClient(t, http.StatusNotFound, http.StatusOK)
	if err := c.Resume(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if got := qb.seen("/api/v2/torrents/start"); len(got) != 1 {
		t.Errorf("torrents/start called %d times, want 1 (tried first)", len(got))
	}
	if got := qb.seen("/api/v2/torrents/resume"); len(got) != 1 || got[0] != "a|b" {
		t.Errorf("torrents/resume hashes = %q, want one call with a|b", got)
	}
}

func TestResumeDoesNotFallBackOnOtherErrors(t *testing.T) {
	c, qb := newResumeClient(t, http.StatusInternalServerError, http.StatusOK)
	err := c.Resume(context.Background(), []string{"a"})
	if !httpx.IsStatus(err, http.StatusInternalServerError) {
		t.Fatalf("Resume error = %v, want the start endpoint's HTTP 500", err)
	}
	if got := qb.seen("/api/v2/torrents/resume"); len(got) != 0 {
		t.Errorf("torrents/resume called %d times, want 0 after a non-404 error", len(got))
	}
}

func TestResumeFailsWhenNeitherEndpointExists(t *testing.T) {
	c, _ := newResumeClient(t, http.StatusNotFound, http.StatusNotFound)
	err := c.Resume(context.Background(), []string{"a"})
	if !httpx.IsStatus(err, http.StatusNotFound) {
		t.Fatalf("Resume error = %v, want HTTP 404", err)
	}
}
