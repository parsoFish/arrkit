package radarr_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/parsoFish/arrkit/radarr"
)

// ExampleNew lists Radarr's health warnings. The httptest server stands in
// for a real Radarr; in your code the base URL is e.g. http://radarr:7878.
func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"source":"IndexerStatusCheck","type":"warning","message":"Indexers unavailable"}]`))
	}))
	defer srv.Close()

	c, err := radarr.New(srv.URL, "your-api-key")
	if err != nil {
		log.Fatal(err)
	}
	items, err := c.Health(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, it := range items {
		fmt.Printf("%s %s: %s\n", it.Type, it.Source, it.Message)
	}
	// Output: warning IndexerStatusCheck: Indexers unavailable
}

// ExampleFake stands in for Radarr in a caller's unit test: seed the fields
// the code under test reads, pass the Fake as a radarr.Client, then check
// Calls.
func ExampleFake() {
	f := &radarr.Fake{Missing: 3}
	var c radarr.Client = f

	n, err := c.WantedMissingCount(context.Background())
	fmt.Println(n, err)
	fmt.Println(f.Calls)
	// Output:
	// 3 <nil>
	// [WantedMissingCount()]
}
