package qbittorrent

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Client for callers' unit tests. Its methods are safe
// for concurrent use; read Calls only after the calls under test return.
type Fake struct {
	TorrentList []Torrent
	FilesByHash map[string][]File
	Prefs       Preferences
	Calls       []string
	Err         error

	mu sync.Mutex // guards Calls
}

var _ Client = (*Fake)(nil)

func (f *Fake) record(format string, args ...any) error {
	call := fmt.Sprintf(format, args...)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, call)
	return f.Err
}

func (f *Fake) Version(context.Context) (string, error) {
	return "fake", f.record("Version()")
}

func (f *Fake) Torrents(_ context.Context, filter, category string) ([]Torrent, error) {
	return f.TorrentList, f.record("Torrents(%q, %q)", filter, category)
}

func (f *Fake) Files(_ context.Context, hash string) ([]File, error) {
	return f.FilesByHash[hash], f.record("Files(%s)", hash)
}

func (f *Fake) Delete(_ context.Context, hashes []string, deleteFiles bool) error {
	return f.record("Delete(%v, %t)", hashes, deleteFiles)
}

func (f *Fake) Reannounce(_ context.Context, hashes []string) error {
	return f.record("Reannounce(%v)", hashes)
}

func (f *Fake) Resume(_ context.Context, hashes []string) error {
	return f.record("Resume(%v)", hashes)
}

func (f *Fake) Preferences(context.Context) (Preferences, error) {
	return f.Prefs, f.record("Preferences()")
}

func (f *Fake) SetPreferences(_ context.Context, patch map[string]any) error {
	return f.record("SetPreferences(%v)", patch)
}
