package overseerr

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Client for callers' unit tests. Its methods are safe
// for concurrent use; read Calls only after the calls under test return.
type Fake struct {
	StatusValue Status
	RequestList []Request
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

func (f *Fake) Status(context.Context) (Status, error) {
	return f.StatusValue, f.record("Status()")
}

func (f *Fake) Requests(_ context.Context, filter string) ([]Request, error) {
	return f.RequestList, f.record("Requests(%s)", filter)
}

func (f *Fake) DeclineRequest(_ context.Context, id int64) error {
	return f.record("DeclineRequest(%d)", id)
}
