package qbittorrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/parsoFish/arrkit/httpx"
)

const apiBase = "/api/v2"

// HTTPClient talks to a real qBittorrent WebUI. It is safe for concurrent
// use: logins are serialised, so concurrent callers share one session, and a
// session that expires (403) is renewed once however many callers see it.
type HTTPClient struct {
	h    *httpx.Client
	user string
	pass string

	mu       sync.Mutex // serialises login; guards loggedIn and session
	loggedIn bool
	session  uint64 // bumped by every successful login
}

var _ Client = (*HTTPClient)(nil)

// New builds an HTTPClient. Session auth needs cookies shared across
// requests, so a cookiejar-backed *http.Client is built first and handed to
// httpx via WithHTTPClient (which stores a shallow copy that keeps the same
// Jar). qBittorrent also requires a matching Referer header on every request
// as a CSRF check.
func New(baseURL, user, pass string, opts ...httpx.Option) (*HTTPClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent: build cookie jar: %w", err)
	}
	hc := &http.Client{Timeout: 15 * time.Second, Jar: jar}
	all := append([]httpx.Option{httpx.WithHTTPClient(hc), httpx.WithHeader("Referer", baseURL)}, opts...)
	h, err := httpx.New(baseURL, all...)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent: %w", err)
	}
	return &HTTPClient{h: h, user: user, pass: pass}, nil
}

// login authenticates and stores the SID cookie in the shared cookie jar.
// The caller must hold c.mu. An empty user and pass means the caller relies
// on qBittorrent's subnet whitelist, so login is skipped entirely.
func (c *HTTPClient) login(ctx context.Context) error {
	if c.user == "" && c.pass == "" {
		return nil
	}
	var body string
	form := url.Values{"username": {c.user}, "password": {c.pass}}
	if err := c.h.PostForm(ctx, apiBase+"/auth/login", form, &body); err != nil {
		return fmt.Errorf("qbittorrent login: %w", err)
	}
	if strings.TrimSpace(body) != "Ok." {
		return errors.New("qbittorrent login: invalid credentials")
	}
	c.loggedIn = true
	c.session++
	return nil
}

// ensureSession logs in unless a session already exists, and returns the
// session generation the caller's request runs under.
func (c *HTTPClient) ensureSession(ctx context.Context) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return 0, err
		}
	}
	return c.session, nil
}

// renewSession logs in again after a request made under session generation
// `failed` got a 403. If another caller has already renewed that session, it
// does nothing.
func (c *HTTPClient) renewSession(ctx context.Context, failed uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn && c.session != failed {
		return nil
	}
	c.loggedIn = false
	return c.login(ctx)
}

// withAuth runs fn, logging in first if needed and once more after a 403
// (the session expired between calls).
func (c *HTTPClient) withAuth(ctx context.Context, fn func() error) error {
	session, err := c.ensureSession(ctx)
	if err != nil {
		return err
	}
	err = fn()
	if httpx.IsStatus(err, http.StatusForbidden) {
		if lerr := c.renewSession(ctx, session); lerr != nil {
			return lerr
		}
		return fn()
	}
	return err
}

// Version fetches /api/v2/app/version, a plain-text endpoint that doubles as
// a liveness and auth check.
func (c *HTTPClient) Version(ctx context.Context) (string, error) {
	var out string
	err := c.withAuth(ctx, func() error {
		return c.h.GetText(ctx, apiBase+"/app/version", nil, &out)
	})
	if err != nil {
		return "", fmt.Errorf("qbittorrent version: %w", err)
	}
	return out, nil
}

// rawTorrent mirrors the /torrents/info wire shape; AddedOn/CompletionOn are
// epoch seconds and SeedingTime is a plain integer of seconds, both
// converted to their public time.Time/time.Duration equivalents below.
type rawTorrent struct {
	Hash         string    `json:"hash"`
	Name         string    `json:"name"`
	State        string    `json:"state"`
	Category     string    `json:"category"`
	SavePath     string    `json:"save_path"`
	ContentPath  string    `json:"content_path"`
	Progress     float64   `json:"progress"`
	Ratio        float64   `json:"ratio"`
	Size         int64     `json:"size"`
	Completed    int64     `json:"completed"`
	Downloaded   int64     `json:"downloaded"`
	Uploaded     int64     `json:"uploaded"`
	AddedOn      epochTime `json:"added_on"`
	CompletionOn epochTime `json:"completion_on"`
	SeedingTime  int64     `json:"seeding_time"`
	DlSpeed      int64     `json:"dlspeed"`
	UpSpeed      int64     `json:"upspeed"`
	NumSeeds     int       `json:"num_seeds"`
	NumLeechs    int       `json:"num_leechs"`
}

// Torrents lists torrents, sending filter/category as query parameters only
// when non-empty.
func (c *HTTPClient) Torrents(ctx context.Context, filter, category string) ([]Torrent, error) {
	q := url.Values{}
	if filter != "" {
		q.Set("filter", filter)
	}
	if category != "" {
		q.Set("category", category)
	}
	var raw []rawTorrent
	err := c.withAuth(ctx, func() error {
		return c.h.GetJSON(ctx, apiBase+"/torrents/info", q, &raw)
	})
	if err != nil {
		return nil, fmt.Errorf("qbittorrent torrents: %w", err)
	}
	out := make([]Torrent, 0, len(raw))
	for _, r := range raw {
		out = append(out, Torrent{
			Hash: r.Hash, Name: r.Name, State: r.State, Category: r.Category,
			SavePath: r.SavePath, ContentPath: r.ContentPath,
			Progress: r.Progress, Ratio: r.Ratio,
			Size: r.Size, Completed: r.Completed, Downloaded: r.Downloaded, Uploaded: r.Uploaded,
			AddedOn: r.AddedOn.Time, CompletionOn: r.CompletionOn.Time,
			SeedingTime: time.Duration(r.SeedingTime) * time.Second,
			DlSpeed:     r.DlSpeed, UpSpeed: r.UpSpeed,
			NumSeeds: r.NumSeeds, NumLeechs: r.NumLeechs,
		})
	}
	return out, nil
}

// Files lists the files within a single torrent.
func (c *HTTPClient) Files(ctx context.Context, hash string) ([]File, error) {
	var out []File
	err := c.withAuth(ctx, func() error {
		return c.h.GetJSON(ctx, apiBase+"/torrents/files", url.Values{"hash": {hash}}, &out)
	})
	if err != nil {
		return nil, fmt.Errorf("qbittorrent files %s: %w", hash, err)
	}
	return out, nil
}

// Delete removes torrents, optionally deleting their files on disk.
func (c *HTTPClient) Delete(ctx context.Context, hashes []string, deleteFiles bool) error {
	form := url.Values{
		"hashes":      {strings.Join(hashes, "|")},
		"deleteFiles": {strconv.FormatBool(deleteFiles)},
	}
	err := c.withAuth(ctx, func() error {
		return c.h.PostForm(ctx, apiBase+"/torrents/delete", form, nil)
	})
	if err != nil {
		return fmt.Errorf("qbittorrent delete: %w", err)
	}
	return nil
}

// Reannounce asks the tracker(s) to re-announce the given torrents.
func (c *HTTPClient) Reannounce(ctx context.Context, hashes []string) error {
	form := url.Values{"hashes": {strings.Join(hashes, "|")}}
	err := c.withAuth(ctx, func() error {
		return c.h.PostForm(ctx, apiBase+"/torrents/reannounce", form, nil)
	})
	if err != nil {
		return fmt.Errorf("qbittorrent reannounce: %w", err)
	}
	return nil
}

// Resume resumes the given torrents.
func (c *HTTPClient) Resume(ctx context.Context, hashes []string) error {
	form := url.Values{"hashes": {strings.Join(hashes, "|")}}
	err := c.withAuth(ctx, func() error {
		return c.h.PostForm(ctx, apiBase+"/torrents/resume", form, nil)
	})
	if err != nil {
		return fmt.Errorf("qbittorrent resume: %w", err)
	}
	return nil
}

// rawPreferences mirrors /app/preferences' actual snake_case wire shape.
// Preferences itself carries camelCase json tags for --json output, so
// decoding goes through this private type instead.
type rawPreferences struct {
	SavePath          string  `json:"save_path"`
	TempPath          string  `json:"temp_path"`
	TempPathEnabled   bool    `json:"temp_path_enabled"`
	MaxRatio          float64 `json:"max_ratio"`
	MaxSeedingTime    int     `json:"max_seeding_time"`
	ExcludedFileNames string  `json:"excluded_file_names"`
}

// Preferences fetches the application preferences.
func (c *HTTPClient) Preferences(ctx context.Context) (Preferences, error) {
	var raw rawPreferences
	err := c.withAuth(ctx, func() error {
		return c.h.GetJSON(ctx, apiBase+"/app/preferences", nil, &raw)
	})
	if err != nil {
		return Preferences{}, fmt.Errorf("qbittorrent preferences: %w", err)
	}
	// rawPreferences and Preferences share identical field names, order and
	// types (only their json tags differ), so a direct conversion is both
	// correct and self-maintaining if a field is ever added to both.
	return Preferences(raw), nil
}

// SetPreferences merges patch into the application preferences.
func (c *HTTPClient) SetPreferences(ctx context.Context, patch map[string]any) error {
	body, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("qbittorrent set preferences: encode patch: %w", err)
	}
	form := url.Values{"json": {string(body)}}
	err = c.withAuth(ctx, func() error {
		return c.h.PostForm(ctx, apiBase+"/app/setPreferences", form, nil)
	})
	if err != nil {
		return fmt.Errorf("qbittorrent set preferences: %w", err)
	}
	return nil
}
