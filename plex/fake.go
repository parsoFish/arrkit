package plex

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Client for callers' unit tests. Its methods are safe
// for concurrent use; read Calls only after the calls under test return.
type Fake struct {
	IdentityValue Identity
	LibraryList   []Library
	ItemsByKey    map[string][]Item
	Calls         []string
	Err           error

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

func (f *Fake) Identity(context.Context) (Identity, error) {
	return f.IdentityValue, f.record("Identity()")
}

func (f *Fake) Libraries(context.Context) ([]Library, error) {
	return f.LibraryList, f.record("Libraries()")
}

func (f *Fake) RecentlyAdded(_ context.Context, libraryKey string, limit int) ([]Item, error) {
	return f.ItemsByKey[libraryKey], f.record("RecentlyAdded(%s,%d)", libraryKey, limit)
}

func (f *Fake) RefreshLibrary(_ context.Context, libraryKey string) error {
	return f.record("RefreshLibrary(%s)", libraryKey)
}
