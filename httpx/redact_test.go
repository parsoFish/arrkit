package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// secret is the password every test below plants; no error may contain it.
const secret = "s3cr3t-pw"

func TestRedactURLMasksUserinfoPasswordAndQuery(t *testing.T) {
	got := redactURL("http://admin:" + secret + "@example.com/api?apikey=" + secret)
	if want := "http://admin:xxxxx@example.com/api?apikey=REDACTED"; got != want {
		t.Fatalf("redactURL = %q, want %q", got, want)
	}
}

func TestRedactURLMasksUserinfoPasswordWithoutQuery(t *testing.T) {
	got := redactURL("http://admin:" + secret + "@example.com/api")
	if want := "http://admin:xxxxx@example.com/api"; got != want {
		t.Fatalf("redactURL = %q, want %q", got, want)
	}
}

func TestRedactURLReplacesUnparsableInput(t *testing.T) {
	got := redactURL("http://admin:" + secret + "@exa mple.com/")
	if want := "(unparsable URL, redacted)"; got != want {
		t.Fatalf("redactURL = %q, want %q", got, want)
	}
}

func TestAPIErrorRedactsUserinfoPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	base.User = url.UserPassword("admin", secret)
	c, err := New(base.String())
	if err != nil {
		t.Fatal(err)
	}
	err = c.GetJSON(context.Background(), "/status", nil, nil)
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(ae.URL, secret) {
		t.Fatalf("APIError leaked the userinfo password: %v", err)
	}
}

func TestTransportErrorRedactsUserinfoPassword(t *testing.T) {
	// Port 1 is privileged and unlikely to have a listener, so the dial
	// fails at once with a transport error.
	c, err := New("http://admin:" + secret + "@127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	err = c.GetJSON(context.Background(), "/x", nil, nil)
	if err == nil {
		t.Fatal("expected a transport error connecting to a closed port")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("transport error leaked the userinfo password: %v", err)
	}
}

func TestNewInvalidBaseURLDoesNotLeakSecrets(t *testing.T) {
	cases := map[string]string{
		"parse error in host":       "http://admin:" + secret + "@exa mple.com",
		"parse error, control byte": "http://admin:" + secret + "@example.com/\x7f",
		"parse error, port slot":    "http://nas:" + secret,
		"no scheme, userinfo":       "admin:" + secret + "@nas:8080",
		"no host, query":            "/radarr?apikey=" + secret,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(raw)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "invalid base URL") {
				t.Errorf("error %q does not say invalid base URL", err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error leaked the secret: %q", err)
			}
		})
	}
}
