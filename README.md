# go-port

Rafael Francisco's portfolio site — software projects, photography gallery, and a journal — served server-side-rendered. Pure Go (no cgo, no Node runtime), single-binary deploys with embedded assets and migrations.

| Concern | Library |
| --- | --- |
| HTTP server + OpenAPI | [fuego](https://github.com/go-fuego/fuego) — spec generated from handlers, served at `/swagger` |
| HTML templates | [templ](https://github.com/a-h/templ) — type-safe, compiled to Go |
| Reactivity | [htmx](https://htmx.org) — vendored in `static/js`, embedded in the binary |
| CSS | [Tailwind CSS v4](https://tailwindcss.com) — standalone binary, no Node |
| Errors | [cockroachdb/errors](https://github.com/cockroachdb/errors) — stack traces via `%+v` |
| SQL codegen | [sqlc](https://sqlc.dev) — type-safe queries from `internal/db/queries.sql` |
| Database | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — pure-Go SQLite (driver name `"sqlite"`) |
| Migrations | [goose](https://github.com/pressly/goose) — embedded, applied at startup |
| Durable workflows | [DBOS](https://github.com/dbos-inc/dbos-transact-golang) — SQLite system DB via `Config.SqliteSystemDB` |

Pages: home (`/`), `/software` (hardcoded project list in `internal/content`), `/photography` (placeholder tiles until the Flickr integration lands; photo-detail modal is an HTMX fragment), `/journal` (posts stored as Markdown in SQLite, rendered with [goldmark](https://github.com/yuin/goldmark), plus an RSS feed at `/journal/feed.xml`) and `/about`. IBM Plex fonts are self-hosted and embedded. No DBOS workflows are registered yet; the wiring stays in place for future scheduled jobs (e.g. a Flickr cache refresh).

## Quick start

```sh
make tools     # installs templ/sqlc/goose/air into ./bin, downloads tailwindcss + htmx
make run       # generate + css + go run ./cmd/server  → http://localhost:8080
```

Or `make dev` for live reload (air regenerates templ + tailwind on save).

## Layout

```
cmd/server/main.go        wiring: connect → migrate → DBOS → fuego
internal/content/         hardcoded portfolio data (projects, gallery placeholders)
internal/db/              sqlc-generated queries + goose migrations (embedded)
internal/handlers/        fuego handlers and route registration
internal/views/           templ templates (page + HTMX fragments)
internal/workflows/       DBOS durable workflows (none yet)
static/                   htmx + compiled tailwind + IBM Plex fonts, embedded via embed.FS
```

## How the pieces fit

- **Handlers return templ components directly** — `fuego.Templ` is an alias for anything with `Render(ctx, w)`, which every templ component satisfies. Full pages and HTMX fragments (the photo-detail modal at `/photography/photos/{index}`) are the same mechanism.
- **Two SQLite files** — `app.db` (application data: goose-migrated, sqlc-queried — the journal posts, including their seed content) and `dbos.db` (DBOS workflow bookkeeping), so durable-workflow writes never contend with app writes. Both opened with WAL + busy timeout in `internal/db/connect.go`.
- **Workflows** — none registered yet; `internal/workflows.Register` is a stub that keeps the DBOS wiring in `main` intact for future scheduled jobs.
- **Graceful shutdown** — SIGINT/SIGTERM drains in-flight HTTP requests (10s budget), then shuts down DBOS and closes both databases in order.
- **Ops** — `GET /healthz` pings both SQLite handles. fuego logs every request/response with a request ID out of the box (configurable via `fuego.WithLoggingMiddleware`); add cross-cutting middleware with `fuego.Use`.
- **Errors** — wrap with `errors.Wrap(err, "context")` everywhere; `main` logs `%+v` for full stack traces. Return `fuego.NotFoundError` / `fuego.BadRequestError` for expected 4xx (rendered as RFC 9457 problem JSON).

## Everyday commands

```sh
make generate                 # templ generate && sqlc generate
make css                      # rebuild static/css/app.css
make test                     # handler tests (httptest)
make migration name=add_x     # new goose migration in internal/db/migrations
make migrate-status           # goose status against app.db
make clean                    # remove tmp/, *.db
```

Generated files (`*_templ.go`, `internal/db/*.go`, `static/css/app.css`) are committed so `go build ./...` works straight from a fresh clone; regenerate them with `make generate && make css` after editing templates, queries, or styles. Tool versions are pinned in the Makefile, and CI (`.github/workflows/ci.yml`) regenerates everything and fails on drift, so committed output can never silently diverge from its sources.

## Docker

```sh
docker build -t go-port .
docker run -p 8080:8080 -v go-port-data:/data go-port
```

Single static binary on a distroless base; both SQLite files live in the `/data` volume.

## Configuration

`ADDR`, `APP_DB`, `DBOS_DB` env vars override the defaults (`localhost:8080`, `app.db`, `dbos.db`).

`doc/openapi.json` is gitignored because fuego rewrites it on every server start; commit it instead if you want spec changes reviewed in PRs.
