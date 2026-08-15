# Embedded Web UI Image Design

## Goal

Build the React admin console into the Go executable so the production Docker image contains one executable, runs one process, and serves both the frontend and API from port 8080.

## Selected Approach

Use a build-tagged `go:embed` package. Docker builds `web/dist` first, copies it into the embed package, and compiles the server with `-tags webui`. Normal local Go builds omit the tag and keep the current separate frontend development workflow.

This avoids committing generated frontend assets and avoids adding a second process such as Nginx to the application image.

## Components

### Web UI package

Add `internal/platform/webui` with three responsibilities:

- Provide embedded frontend assets when built with the `webui` tag.
- Report that no embedded assets are available in ordinary local builds.
- Register a Gin fallback handler that serves files and SPA routes from an `fs.FS`.

The tagged implementation uses `//go:embed all:dist` and `fs.Sub` so callers see the contents of `dist` as the filesystem root. The untagged implementation returns no filesystem and leaves existing API-only behavior unchanged.

### Router integration

Register the web UI after all API routes have been configured.

- Existing `/v1/*` and `/healthz` handlers keep precedence.
- Unknown `/v1/*` paths remain 404 responses and never receive `index.html`.
- Existing frontend files are served with their detected content type.
- Frontend navigation routes such as `/orders`, `/refunds`, and `/checkout/:orderNo` return `index.html`.
- Missing paths with a file extension, such as `/static/missing.js`, return 404 instead of HTML.
- Only GET and HEAD requests are eligible for frontend fallback.

Hashed assets under `/static/` receive `Cache-Control: public, max-age=31536000, immutable`. `index.html` and SPA fallback responses receive `Cache-Control: no-cache`.

### Docker build

Extend `Dockerfile` to use three stages:

1. A Bun stage installs locked frontend dependencies and builds `web/dist`.
2. A Go stage downloads Go modules, copies the source and frontend output into `internal/platform/webui/dist`, then builds with `-tags webui`.
3. The final Alpine stage contains only `/app/payment-gateway` and runs it as the existing non-root `app` user.

Add `.dockerignore` entries for Git metadata, local environment files, frontend dependencies, build output, and temporary artifacts. No secrets or `.env` files may enter the build context.

## Local Development

`make dev`, `go test ./...`, and `go build ./cmd/server` continue to build without the `webui` tag and do not require frontend assets. `make web-dev` remains the frontend development command.

Production image builds are the only normal path that enables `webui`.

## Error Handling

- Failure to locate `index.html` during a tagged build is a startup-visible configuration error, not a silent API-only fallback.
- Static file read failures return 404 without exposing filesystem paths.
- API errors keep the global `{ code, message, data, error }` response shape because the web UI fallback does not intercept registered API handlers.

## Testing

Add focused tests using `testing/fstest.MapFS` for:

- Serving an existing static asset.
- Returning `index.html` for a frontend navigation route.
- Returning 404 for unknown API routes.
- Returning 404 for missing asset paths.
- Rejecting frontend fallback for non-GET/HEAD methods.
- Applying immutable cache headers to `/static/` assets and no-cache to HTML.

Verification must include:

```bash
go test ./...
make web-build
go test -tags webui ./internal/platform/webui ./internal/platform/http
docker build --platform linux/amd64 -t ghcr.io/zmoyi/starpay:v0.0.1-beta .
```

The built image must report `linux/amd64`, run as the non-root `app` user, and contain no standalone frontend directory in the final layer. A configured container smoke test must verify `/`, a frontend route, `/healthz`, and an unknown `/v1/*` path.

## Publishing

After verification, replace the existing beta image at `ghcr.io/zmoyi/starpay:v0.0.1-beta` and verify the GHCR manifest digest. This mutable replacement is acceptable only because the requested tag is explicitly a beta tag.
