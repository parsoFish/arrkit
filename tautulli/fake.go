package tautulli

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Fake is an in-memory Client for callers' unit tests. Its methods are safe
// for concurrent use; read Calls only after the calls under test return.
type Fake struct {
	HistoryRows []HistoryRow
	Streams     int
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

func (f *Fake) Ping(context.Context) error {
	return f.record("Ping()")
}

func (f *Fake) History(_ context.Context, since time.Time, length int) ([]HistoryRow, error) {
	return f.HistoryRows, f.record("History(%s,%d)", since.Format(time.RFC3339), length)
}

func (f *Fake) ActivityCount(context.Context) (int, error) {
	return f.Streams, f.record("ActivityCount()")
}
