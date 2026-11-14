# arrkit

Go clients for the services in a Plex + *arr media stack: Radarr, Sonarr, Prowlarr, Overseerr, qBittorrent, Plex and Tautulli.

Each package gives you:

- a `Client` interface with the package's own request and response types,
- an `HTTPClient` that talks to the real service over the shared `httpx` client,
- an in-memory `Fake` that implements `Client`, for callers' unit tests,
- golden JSON fixtures in `testdata/`, captured from live services and scrubbed.

arrkit has no third-party dependencies.

It was extracted from [healarr](https://github.com/parsoFish/healarr), and is used by healarr and [simplarr](https://github.com/parsoFish/simplarr).

## Packages

| Package | Service | Auth |
|---|---|---|
| `httpx` | shared JSON/form HTTP client: secret-redacting errors, timeouts, TLS server name | — |
| `radarr`, `sonarr` | Radarr v3 API, Sonarr v3 API | `X-Api-Key` |
| `prowlarr` | Prowlarr v1 API | `X-Api-Key` |
| `overseerr` | Overseerr v1 API | `X-Api-Key` |
| `qbittorrent` | qBittorrent WebUI v2 API | username + password, cookie session |
| `plex` | Plex Media Server | `X-Plex-Token` |
| `tautulli` | Tautulli v2 API | `apikey` query parameter, redacted in errors |

## Usage

```go
c, err := radarr.New("http://radarr:7878", apiKey)
if err != nil {
    return err
}
items, err := c.Health(ctx)
```

## Tests

```bash
go test ./... -race -cover
golangci-lint run
```

Fixture rule: anything added under `testdata/` must be scrubbed before you commit it. That means no real hostnames, addresses, emails, API keys, tokens or `plex.direct` hashes. Use `example.com`, `10.0.0.x` or `192.0.2.x` and obviously fake IDs instead.

## Stability

arrkit is pre-1.0. Breaking changes can land in minor versions, and each one is called out in the release notes.
