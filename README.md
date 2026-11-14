# arrkit

Go clients for the services in a Plex + *arr media stack: Radarr, Sonarr, Prowlarr, Overseerr, qBittorrent, Plex and Tautulli.

Each package gives you:

- a `Client` interface with the package's own request and response types,
- an `HTTPClient` that talks to the real service over the shared `httpx` client,
- an in-memory `Fake` that implements `Client`, for callers' unit tests,
- golden JSON fixtures in `testdata/`, captured from live services and scrubbed.

arrkit has no third-party dependencies.

It was extracted from [healarr](https://github.com/parsoFish/healarr), which uses it. [simplarr](https://github.com/parsoFish/simplarr) will use it for its control-plane UI.

## Packages

| Package | Service | Auth |
|---|---|---|
| `httpx` | shared JSON/form HTTP client: URL redaction in errors (see [Errors](#errors)), timeouts, TLS server name | — |
| `radarr`, `sonarr` | Radarr v3 API, Sonarr v3 API | `X-Api-Key` |
| `prowlarr` | Prowlarr v1 API | `X-Api-Key` |
| `overseerr` | Overseerr v1 API | `X-Api-Key` |
| `qbittorrent` | qBittorrent WebUI v2 API, 4.x and 5.x | username + password, cookie session |
| `plex` | Plex Media Server | `X-Plex-Token` |
| `tautulli` | Tautulli v2 API | `apikey` query parameter, redacted in errors |

Every `HTTPClient` is safe for concurrent use, and so is every `Fake`.

qBittorrent 5.0 renamed `torrents/resume` to `torrents/start`. `Resume` tries `start` and falls back to `resume` on a 404, so it works on 4.x and 5.x. Torrent `State` values are passed through as the server sends them: 5.x reports `stoppedUP` where 4.x reported `pausedUP`.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/parsoFish/arrkit/radarr"
)

func main() {
	c, err := radarr.New("http://radarr:7878", os.Getenv("RADARR_API_KEY"))
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
}
```

`radarr/example_test.go` runs the same code against a test server, so `go test` keeps it compiling.

## Fakes in your tests

Hand a `Fake` to the code under test wherever it takes a `Client`. Seed the fields it reads, set `Err` to make every method fail, and check `Calls` afterwards:

```go
package mycheck_test

import (
	"context"
	"testing"

	"github.com/parsoFish/arrkit/radarr"
)

func TestMissingCount(t *testing.T) {
	f := &radarr.Fake{Missing: 3}
	var c radarr.Client = f // what the code under test receives

	n, err := c.WantedMissingCount(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("WantedMissingCount = %d, %v", n, err)
	}
	if len(f.Calls) != 1 || f.Calls[0] != "WantedMissingCount()" {
		t.Fatalf("Calls = %v", f.Calls)
	}
}
```

A `Fake`'s methods can be called from many goroutines at once. Read `Calls` only after those calls have returned.

## Errors

`httpx` puts every URL through one redaction step (`redactURL`) before it reaches an error:

- query parameter values become `REDACTED` (the names are kept),
- a userinfo password becomes `xxxxx` (the username is kept),
- a URL that does not parse is replaced whole.

That covers the `URL` field and message of `*httpx.APIError`, and transport errors, including the `*url.Error` the standard library wraps. `httpx.New` returns one fixed error for any invalid base URL, `httpx: invalid base URL (value not shown; it may contain credentials)`, and it never echoes the input. When a failed request carried a query string, `APIError.Body` is a placeholder, because a server may echo its query back.

Still visible in errors: the username in userinfo when there is no password, query parameter names, the URL path, and the fragment. Keep secrets out of those.

Not redacted: the body of a failed request without a query string (`APIError.Body`; the message shows its first 200 bytes) and the first 200 bytes of a response that fails to decode. Header values, including `X-Api-Key`, `X-Plex-Token` and the qBittorrent session cookie, are never put in an error.

## Working across repositories

To try an arrkit change in healarr or simplarr before it is tagged, put both checkouts in a Go workspace. From the folder that holds them:

```bash
go work init ./healarr ./arrkit        # simplarr: go work init ./simplarr/app ./arrkit
cd healarr && go test ./...            # builds against ../arrkit, not the pinned tag
```

`go env GOWORK` shows the workspace in use, and `GOWORK=off` ignores it. The `go.work` file sits outside both repositories, so nothing is committed. Once the arrkit change is tagged, bump the dependency with `go get github.com/parsoFish/arrkit@<tag>` and delete `go.work`.

## Tests

```bash
go test ./... -race -cover
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run   # the version CI pins
scripts/scan-fixtures_test.sh && scripts/scan-fixtures.sh
```

Fixture rule: anything added under `testdata/` must be scrubbed before you commit it. That means no real hostnames, addresses, emails, API keys, tokens, `plex.direct` hashes, torrent infohashes, release names or library paths. Use `example.com`, `10.0.0.x` or `192.0.2.x`, zero-padded hashes (`0000000000000000000000000000000000000001`), `Example.Show.S01E01.1080p-GROUP` and `/tv/Example Show/Season 01` instead. `scripts/scan-fixtures.sh` checks fixtures for infohashes, 32-hex keys, private addresses, emails, home paths, release names and library paths, and CI runs it on every pull request and on pushes to `main`.

## Stability

arrkit is pre-1.0. Breaking changes can land in minor versions, and each one is called out in the [release notes](https://github.com/parsoFish/arrkit/releases).
