package plex

import (
	"context"
	"sync"
	"testing"
)

// TestFakeIsSafeForConcurrentUse calls the Fake from 16 goroutines at once.
// Without the lock in record, -race reports the Calls append and calls can
// be lost.
func TestFakeIsSafeForConcurrentUse(t *testing.T) {
	f := &Fake{}
	ctx := context.Background()
	const goroutines, rounds = 16, 25
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range rounds {
				_, _ = f.Libraries(ctx)
				_ = f.RefreshLibrary(ctx, "1")
			}
		})
	}
	wg.Wait()
	if got, want := len(f.Calls), goroutines*rounds*2; got != want {
		t.Fatalf("len(Calls) = %d, want %d", got, want)
	}
}
