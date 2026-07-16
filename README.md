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

Pages: home (`/`), `/software` (hardcoded project list in `internal/content`), `/photography` (placeholder tiles until the Flickr integration lands; photo-detail modal is an HTMX fragment), `/journal` (published Markdown posts from SQLite plus RSS at `/journal/feed.xml`) and `/about`. A protected `/admin` backoffice manages journal drafts/publication, uploaded images, and site identity/contact settings. IBM Plex fonts are self-hosted and embedded. No DBOS workflows are registered yet; the wiring stays in place for future scheduled jobs (e.g. a Flickr cache refresh).

## Quick start

```sh
make tools     # installs templ/sqlc/goose/air into ./bin, downloads tailwindcss + htmx
make adminhash # interactively generate the required password hash
export ADMIN_PASSWORD_HASH='$argon2id$...'
make run       # generate + css + go run ./cmd/server  → http://localhost:8080
```

Or `make dev` for live reload (air regenerates templ + tailwind on save).

Open `http://localhost:8080/admin/login` and sign in with `ADMIN_USERNAME`
(default `admin`) and the password used by `make adminhash`.

## Layout

```
cmd/server/main.go        wiring: connect → migrate → DBOS → fuego
internal/content/         hardcoded portfolio data (projects, gallery placeholders)
internal/db/              sqlc-generated queries + goose migrations (embedded)
internal/handlers/        fuego handlers and route registration
internal/markdown/        shared Goldmark + bluemonday safe renderer
internal/views/           templ templates (page + HTMX fragments)
internal/workflows/       DBOS durable workflows (none yet)
static/                   htmx + compiled tailwind + IBM Plex fonts, embedded via embed.FS
```

## How the pieces fit

- **Handlers return templ components directly** — `fuego.Templ` is an alias for anything with `Render(ctx, w)`, which every templ component satisfies. Full pages and HTMX fragments (the photo-detail modal at `/photography/photos/{index}`) are the same mechanism.
- **Two SQLite files** — `app.db` (journal posts, admin sessions, site settings, redirects, and media metadata) and `dbos.db` (DBOS workflow bookkeeping), so durable-workflow writes never contend with application writes. Both use WAL + a busy timeout.
- **Admin security** — the only credential is supplied through the environment. Passwords use Argon2id, session tokens are random and stored only as SHA-256 hashes, sessions expire after seven days, and unsafe requests require same-origin and CSRF validation. Five failed logins from one IP within 15 minutes trigger throttling.
- **Publication** — drafts never appear on public pages, archive counts, adjacent navigation, or RSS. Changing a published slug records a permanent redirect. Updates use `updated_at` optimistic concurrency to protect against stale editor tabs.
- **Markdown** — public articles and the HTMX preview share one Goldmark/bluemonday renderer. It supports figures, code, links, and uploaded images while stripping scripts, event attributes, unsafe URLs, and unsupported HTML.
- **Media** — verified JPEG, PNG, WebP, and GIF files up to 10 MiB are stored under `UPLOAD_DIR` with randomized names. `/media/{filename}` uses immutable caching and `nosniff`; referenced files cannot be deleted.
- **Workflows** — none registered yet; `internal/workflows.Register` is a stub that keeps the DBOS wiring in `main` intact for future scheduled jobs.
- **Graceful shutdown** — SIGINT/SIGTERM drains in-flight HTTP requests (10s budget), then shuts down DBOS and closes both databases in order.
- **Ops** — `GET /healthz` pings both SQLite handles. fuego logs every request/response with a request ID out of the box (configurable via `fuego.WithLoggingMiddleware`); add cross-cutting middleware with `fuego.Use`.
- **Errors** — wrap with `errors.Wrap(err, "context")` everywhere; `main` logs `%+v` for full stack traces. Return `fuego.NotFoundError` / `fuego.BadRequestError` for expected 4xx (rendered as RFC 9457 problem JSON).

## Everyday commands

```sh
make generate                 # templ generate && sqlc generate
make css                      # rebuild static/css/app.css
make test                     # handler tests (httptest)
make adminhash                 # interactive Argon2id password hash
make migration name=add_x     # new goose migration in internal/db/migrations
make migrate-status           # goose status against app.db
make clean                    # remove tmp/, *.db
```

Generated files (`*_templ.go`, `internal/db/*.go`, `static/css/app.css`) are committed so `go build ./...` works straight from a fresh clone; regenerate them with `make generate && make css` after editing templates, queries, or styles. Tool versions are pinned in the Makefile, and CI (`.github/workflows/ci.yml`) regenerates everything and fails on drift, so committed output can never silently diverge from its sources.

## Docker

```sh
docker build -t go-port .
docker run -p 8080:8080 \
  -e ADMIN_PASSWORD_HASH='$argon2id$...' \
  -v go-port-data:/data \
  go-port
```

The distroless image keeps `app.db`, `dbos.db`, and uploaded media in the
single `/data` volume. `UPLOAD_DIR` defaults to `/data/uploads` in Docker.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADMIN_PASSWORD_HASH` | none; required | Argon2id hash generated by `make adminhash` |
| `ADMIN_USERNAME` | `admin` | Single administrator username |
| `UPLOAD_DIR` | `uploads` locally, `/data/uploads` in Docker | Persistent uploaded media |
| `ADDR` | `localhost:8080` | HTTP listen address |
| `APP_DB` | `app.db` | Application SQLite database |
| `DBOS_DB` | `dbos.db` | DBOS system SQLite database |

Never put the password or generated hash in the repository. Pass the hash
through the deployment platform's secret/environment configuration. If TLS is
terminated by a reverse proxy, forward `X-Forwarded-Proto: https` so admin
cookies are marked Secure.

## Backup and restore

The application database and upload directory form one logical backup. Keep
them together so media metadata and files cannot drift.

For a local deployment, stop writes (or stop the server), then copy:

```sh
cp app.db backup/app.db
cp -R uploads backup/uploads
```

For Docker, back up `/data/app.db` and `/data/uploads` from the persistent
volume. `dbos.db` should also be retained if workflows are added later.
Restore the database and uploads to the same configured paths before starting
the server. SQLite WAL files are transient; use SQLite's backup command or
stop the application before a raw file copy when the database is active.

`doc/openapi.json` is gitignored because fuego rewrites it on every server start; commit it instead if you want spec changes reviewed in PRs.
