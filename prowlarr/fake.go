package prowlarr

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Client for callers' unit tests. Its methods are safe
// for concurrent use; read Calls only after the calls under test return.
type Fake struct {
	HealthItems []HealthItem
	IndexerList []Indexer
	Statuses    []IndexerStatus
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

func (f *Fake) Health(context.Context) ([]HealthItem, error) {
	return f.HealthItems, f.record("Health()")
}

func (f *Fake) Indexers(context.Context) ([]Indexer, error) {
	return f.IndexerList, f.record("Indexers()")
}

func (f *Fake) IndexerStatus(context.Context) ([]IndexerStatus, error) {
	return f.Statuses, f.record("IndexerStatus()")
}

func (f *Fake) DeleteIndexer(_ context.Context, id int64) error {
	return f.record("DeleteIndexer(%d)", id)
}

func (f *Fake) TestIndexer(_ context.Context, id int64) error {
	return f.record("TestIndexer(%d)", id)
}
