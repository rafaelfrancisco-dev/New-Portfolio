# go-port

A server-side-rendered Go web app template. Pure Go (no cgo, no Node runtime), single-binary deploys with embedded assets and migrations.

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

The included demo is a todo list: HTMX add/toggle/delete swapping templ fragments, a content-negotiated JSON API at `/api/todos`, and a "clean up completed" button that runs a two-step DBOS durable workflow (also registered as a nightly cron workflow).

## Quick start

```sh
make tools     # installs templ/sqlc/goose/air into ./bin, downloads tailwindcss + htmx
make run       # generate + css + go run ./cmd/server  → http://localhost:8080
```

Or `make dev` for live reload (air regenerates templ + tailwind on save).

## Layout

```
cmd/server/main.go        wiring: connect → migrate → DBOS → fuego
internal/db/              sqlc-generated queries + goose migrations (embedded)
internal/handlers/        fuego handlers and route registration
internal/views/           templ templates (page + HTMX fragments)
internal/workflows/       DBOS durable workflows
static/                   htmx + compiled tailwind, embedded via embed.FS
```

## How the pieces fit

- **Handlers return templ components directly** — `fuego.Templ` is an alias for anything with `Render(ctx, w)`, which every templ component satisfies. Full pages and HTMX fragments are the same mechanism.
- **One handler, JSON and HTML** — `/api/todos` returns `fuego.DataOrHTML(todos, views.TodoList(todos))`; fuego picks the representation from the `Accept` header and documents the JSON shape in OpenAPI.
- **Forms** — HTMX posts `application/x-www-form-urlencoded`; fuego decodes it into the body struct via `schema:"..."` tags (JSON clients use the same endpoint with `json` tags).
- **Two SQLite files** — `app.db` (application data: goose-migrated, sqlc-queried) and `dbos.db` (DBOS workflow bookkeeping), so durable-workflow writes never contend with app writes. Both opened with WAL + busy timeout in `internal/db/connect.go`.
- **Workflows** — deterministic function of their input; every side effect goes through `dbos.RunAsStep` so completed steps are replayed from `dbos.db` after a crash instead of re-executed. Registered in `internal/workflows` before `dbos.Launch`. `ScheduledCleanup` shows cron-triggered workflows (`dbos.WithSchedule`, 6-field cron with seconds) invoking `CleanupCompleted` as a durable child workflow.
- **Graceful shutdown** — SIGINT/SIGTERM drains in-flight HTTP requests (10s budget), then shuts down DBOS and closes both databases in order.
- **Ops** — `GET /healthz` pings both SQLite handles. fuego logs every request/response with a request ID out of the box (configurable via `fuego.WithLoggingMiddleware`); add cross-cutting middleware with `fuego.Use`.
- **Errors** — wrap with `errors.Wrap(err, "context")` everywhere; `main` logs `%+v` for full stack traces. Return `fuego.NotFoundError` / `fuego.BadRequestError` for expected 4xx (rendered as RFC 9457 problem JSON).

## Everyday commands

```sh
make generate                 # templ generate && sqlc generate
make css                      # rebuild static/css/app.css
make test                     # handler tests (httptest) + DBOS workflow integration test
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

## Adapting the template

1. Rename the module in `go.mod` and update the `go-port` imports (`grep -r go-port`).
2. Replace the todo migration/queries/views/handlers with your domain.
3. `ADDR`, `APP_DB`, `DBOS_DB` env vars override the defaults (`localhost:8080`, `app.db`, `dbos.db`).

**Deliberately not included** (add before shipping anything real): sessions/authentication and CSRF protection — the HTMX form posts here are unprotected. `doc/openapi.json` is gitignored because fuego rewrites it on every server start; commit it instead if you want spec changes reviewed in PRs.
