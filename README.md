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

Pages: home (`/`), `/software` (hardcoded project list in `internal/content`), `/photography` (public Flickr albums cached in SQLite; photo details are HTMX fragments), `/journal` (published Markdown posts from SQLite plus RSS at `/journal/feed.xml`), `/about` and `/lightroom` (hardcoded plug-in list in `internal/content`, downloads link to GitHub releases). A protected `/admin` backoffice manages journal drafts/publication, uploaded images, site identity/contact settings, and the read-only Flickr connection. IBM Plex fonts are self-hosted and embedded. DBOS refreshes the Flickr cache every 12 hours.

## Quick start

The server listens on two loopback-only ports: public pages on `ADDR`
(default `127.0.0.1:8081`) and `/admin` on `ADMIN_ADDR` (default
`127.0.0.1:8082`), each bound to its own configured origin. `PUBLIC_BASE_URL`
and `ADMIN_BASE_URL` are required and must be `https://` except in explicit
local development (`APP_ENV=development`), where `http://localhost` or
`http://127.0.0.1` origins matching the listen ports are allowed.

```sh
make tools     # installs templ/sqlc/goose/air into ./bin, downloads tailwindcss + htmx
make adminhash # interactively generate the required password hash
export ADMIN_PASSWORD_HASH='$argon2id$...'
export APP_ENV=development
export PUBLIC_BASE_URL='http://127.0.0.1:8081'
export ADMIN_BASE_URL='http://127.0.0.1:8082'
# Add the four Flickr variables from Configuration to enable photography.
make run       # generate + css + go run ./cmd/server
```

Or `make dev` for live reload (air regenerates templ + tailwind on save).

Open `http://127.0.0.1:8082/admin/login` and sign in with `ADMIN_USERNAME`
(default `admin`) and the password used by `make adminhash`.

## Layout

```
cmd/server/main.go        wiring: connect → migrate → DBOS → fuego
internal/content/         hardcoded software-project data
internal/db/              sqlc-generated queries + goose migrations (embedded)
internal/flickr/          OAuth client, encryption, sync, and snapshot cache
internal/handlers/        fuego handlers and route registration
internal/markdown/        shared Goldmark + bluemonday safe renderer
internal/views/           templ templates (page + HTMX fragments)
internal/workflows/       DBOS scheduled Flickr refresh
static/                   htmx + compiled tailwind + IBM Plex fonts, embedded via embed.FS
```

## How the pieces fit

- **Handlers return templ components directly** — `fuego.Templ` is an alias for anything with `Render(ctx, w)`, which every templ component satisfies. Full pages and HTMX fragments (the photo-detail modal at `/photography/photos/{photo-id}`) are the same mechanism.
- **Two SQLite files** — `app.db` (journal posts, admin sessions, site settings, redirects, and media metadata) and `dbos.db` (DBOS workflow bookkeeping), so durable-workflow writes never contend with application writes. Both use WAL + a busy timeout.
- **Admin security** — the only credential is supplied through the environment. Passwords use Argon2id behind a bounded concurrency limiter, session tokens are random and stored only as SHA-256 hashes, sessions carry the credential's generation so a password rotation or `revoke-sessions`/"sign out all sessions" invalidates every existing session immediately, sessions expire after 12 hours, and unsafe requests require same-origin and CSRF validation. Login attempts are reserved atomically before hashing: five per IP and 30 total within 15 minutes.
- **Publication** — drafts never appear on public pages, archive counts, adjacent navigation, or RSS. Changing a published slug records a permanent redirect. Updates use `updated_at` optimistic concurrency to protect against stale editor tabs.
- **Markdown** — public articles and the HTMX preview share one Goldmark/bluemonday renderer. It supports figures, code, links, and uploaded images while stripping scripts, event attributes, unsafe URLs, and unsupported HTML.
- **Media** — verified JPEG, PNG, WebP, and GIF files up to 10 MiB are stored under `UPLOAD_DIR` with randomized names. `/media/{filename}` uses immutable caching and `nosniff`; referenced files cannot be deleted.
- **Flickr** — OAuth 1.0a requests read-only access. Access credentials are AES-256-GCM encrypted before SQLite storage. Each sync discovers every visible album, independently confirms every cached photo is public, and builds a complete snapshot before atomically activating it. Empty/private-only albums are omitted; Flickr remains the image host.
- **Workflows** — DBOS runs the Flickr refresh every 12 hours, safely ahead of the 24-hour cache limit. Connecting and the admin “sync now” action also refresh immediately.
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

The `Dockerfile` builds a working binary but Docker is not a supported
deployment path: the server only ever binds loopback addresses (see Quick
start), which ordinary `docker run -p` port publishing cannot reach from
outside the container. Production deploys via the systemd/Caddy path below.

The distroless image keeps `app.db`, `dbos.db`, and uploaded media in the
single `/data` volume. `UPLOAD_DIR` defaults to `/data/uploads` in Docker.

### Production on smalljammy

Production runs as a hardened systemd service under the dedicated, unprivileged
`portfolio` account. It binds only to loopback — `127.0.0.1:8081` for public
pages, `127.0.0.1:8082` for `/admin` — and Caddy is the public HTTPS entry
point, routing both under the one public hostname by path. Application data
survives releases under `/srv/new-portfolio/data`.

Prepare the host once as root:

```sh
useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin portfolio
install -d -o portfolio -g portfolio -m 0750 /srv/new-portfolio/data
install -d -o github -g github -m 0750 /srv/new-portfolio/staging
install -d -o root -g root -m 0755 /opt/new-portfolio/releases
install -o root -g root -m 0755 deploy/deploy-new-portfolio /usr/local/sbin/deploy-new-portfolio
install -o root -g root -m 0644 deploy/new-portfolio.service /etc/systemd/system/new-portfolio.service
install -o root -g root -m 0440 deploy/new-portfolio.sudoers /etc/sudoers.d/new-portfolio
visudo -cf /etc/sudoers.d/new-portfolio
systemctl daemon-reload
systemctl enable new-portfolio.service
```

Create `/etc/new-portfolio.env` with mode `0640`, owned by `root:portfolio`:

```dotenv
ADMIN_PASSWORD_HASH=$argon2id$...
ADMIN_USERNAME=admin
ADDR=127.0.0.1:8081
ADMIN_ADDR=127.0.0.1:8082
APP_DB=/srv/new-portfolio/data/app.db
DBOS_DB=/srv/new-portfolio/data/dbos.db
UPLOAD_DIR=/srv/new-portfolio/data/uploads
FLICKR_API_KEY=your-app-garden-key
FLICKR_API_SECRET=your-app-garden-secret
FLICKR_TOKEN_ENCRYPTION_KEY=base64-encoded-32-byte-key
PUBLIC_BASE_URL=https://imaginarymakings.dev
ADMIN_BASE_URL=https://imaginarymakings.dev
```

`PUBLIC_BASE_URL` and `ADMIN_BASE_URL` are the same public hostname here —
Caddy presents one HTTPS origin and routes `/admin*` internally to the admin
port. They only differ in local development, where there is no proxy to
unify the two ports.

This file is intentionally managed on the production host, not generated by
GitHub Actions and not embedded in the binary. Deployments replace only the
application release and leave `/etc/new-portfolio.env` untouched. The site can
be deployed before the Flickr values are added; `/photography` will show its
temporary-unavailable state and `/admin/flickr` will explain which values are
missing.

When the credentials are available, edit the file as an administrator, verify
its ownership and permissions, and restart the service:

```sh
sudoedit /etc/new-portfolio.env
sudo chown root:portfolio /etc/new-portfolio.env
sudo chmod 0640 /etc/new-portfolio.env
sudo systemctl restart new-portfolio.service
sudo systemctl status new-portfolio.service
```

Generate the hash locally with `make adminhash`; never put the password or hash
in Git. Then add this site to `/etc/caddy/Caddyfile` and reload Caddy:

```caddyfile
imaginarymakings.dev {
	encode zstd gzip

	# The host firewall must allow inbound 80/443 only from Cloudflare's
	# published ranges (https://www.cloudflare.com/ips/), or CF-Connecting-IP
	# below can be forged by connecting to the origin directly.
	@admin path /admin /admin/*
	handle @admin {
		reverse_proxy 127.0.0.1:8082 {
			header_up X-Forwarded-For {header.CF-Connecting-IP}
		}
	}
	handle {
		reverse_proxy 127.0.0.1:8081 {
			header_up X-Forwarded-For {header.CF-Connecting-IP}
		}
	}
}
```

The app only trusts `X-Forwarded-For` when the direct TCP peer is loopback —
i.e. Caddy itself — so this explicit rewrite from Cloudflare's own
client-IP header is what the per-client login throttle (`clientIP` in
`internal/handlers/admin.go`) actually keys on. Without it (or without the
firewall restriction), unrelated visitors sharing a Cloudflare edge IP would
share one rate-limit bucket, or an attacker bypassing Cloudflare could forge
their own IP. `X-Forwarded-Proto` is deliberately ignored; secure cookies and
scheme checks are decided entirely by the configured `PUBLIC_BASE_URL` /
`ADMIN_BASE_URL` scheme, not by a header a client could send directly.

On pushes to `main`, CI tests and regenerates committed outputs first. After CI
succeeds, the `Deploy to smalljammy` job builds an ARM64 binary, hashes it,
and passes that digest to the self-hosted runner's `sudo deploy-new-portfolio
<sha> <sha256>` call. The root-owned helper copies the staged binary through
file descriptors that never follow a symlink, verifies its digest, activates
it atomically, restarts only `new-portfolio.service`, verifies `/healthz`,
and rolls back a failed release. The runner has neither Docker access nor
general sudo access.

`deploy/portfolio-health-check` runs every five minutes via
`portfolio-health.timer` (installed the same way as the backup timer above)
and checks `/healthz`, the service's memory high-watermark, free disk space,
and repeated throttling in the last five minutes of logs; wire its JSON
output into whatever alerting the host already has.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADMIN_PASSWORD_HASH` | none; required | Argon2id hash generated by `make adminhash` |
| `ADMIN_USERNAME` | `admin` | Single administrator username |
| `UPLOAD_DIR` | `uploads` | Persistent uploaded media |
| `ADDR` | `127.0.0.1:8081` | Loopback address for public pages |
| `ADMIN_BASE_URL` | none; required | `/admin`'s public HTTPS origin; `http://localhost`/`http://127.0.0.1` only with `APP_ENV=development` |
| `ADMIN_ADDR` | `127.0.0.1:8082` | Loopback address for `/admin` |
| `APP_ENV` | none | Set to `development` to allow the `http://` + localhost exception above |
| `APP_DB` | `app.db` | Application SQLite database |
| `DBOS_DB` | `dbos.db` | DBOS system SQLite database |
| `FLICKR_API_KEY` | none | Flickr App Garden API key |
| `FLICKR_API_SECRET` | none | Flickr App Garden API secret |
| `FLICKR_TOKEN_ENCRYPTION_KEY` | none | Base64-encoded 32-byte AES key used only for Flickr tokens |
| `PUBLIC_BASE_URL` | none; required | Public HTTPS origin used to construct RSS links and the admin OAuth callback; same local/development exception as `ADMIN_BASE_URL` |

Never put the password or generated hash in the repository. Pass the hash
through the deployment platform's secret/environment configuration.
`ADMIN_PASSWORD_HASH` changes and server restarts do not by themselves
invalidate existing sessions — run `go run ./cmd/revoke-sessions` (with
`APP_DB` set to the production database) immediately after rotating the
password, or sign in and use the dashboard's "sign out all sessions" button.

### Flickr setup and operation

1. Create a non-commercial app in Flickr's App Garden and configure its OAuth
   callback for `https://your-public-origin/admin/flickr/callback`. Set
   `ADMIN_BASE_URL` to that origin without a trailing path (in production
   this is the same hostname as `PUBLIC_BASE_URL`; see above).
2. Put the API key and secret in the server environment. Generate the token
   encryption secret with `openssl rand -base64 32`; store it in the same
   secret manager as the API secret.
3. Sign in at `/admin`, open **Flickr**, choose **connect Flickr**, and approve
   read-only access. The callback performs the first synchronization.
4. Use **sync now** after changing albums, or wait for the scheduled refresh.
   **Disconnect** deletes the encrypted connection, pending OAuth state, and
   cached gallery.

The site never caches a photo until `flickr.photos.getInfo` confirms it is
public. Album filters with no public photos are omitted, and withheld EXIF or
location stays blank. A failed refresh leaves the previous atomic snapshot in
place, but `/photography` stops serving it once its last successful sync is 24
hours old and displays a temporary-unavailable message instead. Flickr-hosted
images preserve their aspect ratio and every detail view links to its Flickr
photo page.

## Backup and restore

The application database, upload directory, and
`FLICKR_TOKEN_ENCRYPTION_KEY` form one logical backup. Keep them together so
media metadata and files cannot drift and the encrypted Flickr connection can
still be decrypted after restore. Losing or rotating the key requires
reconnecting Flickr; never store the key inside the backup or repository.

In production, `deploy/portfolio-backup` runs daily via
`portfolio-backup.timer`: it briefly stops the service for a consistent copy,
backs up `app.db` and `dbos.db` with SQLite's own online backup API (not a
raw file copy), copies `uploads`, writes a SHA-256 manifest, restores that
backup into a throwaway directory to verify it, and keeps seven days. It
deliberately never copies `/etc/new-portfolio.env`, so
`FLICKR_TOKEN_ENCRYPTION_KEY` and the admin password hash still need their
own secret-manager backup. Install it the same way as the service unit:

```sh
install -o root -g root -m 0755 deploy/portfolio-backup /usr/local/sbin/portfolio-backup
install -o root -g root -m 0644 deploy/portfolio-backup.service deploy/portfolio-backup.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now portfolio-backup.timer
```

For a local deployment, stop writes (or stop the server), then copy:

```sh
cp app.db backup/app.db
cp -R uploads backup/uploads
```

Restore the database and uploads to the same configured paths before starting
the server. SQLite WAL files are transient; use SQLite's backup command or
stop the application before a raw file copy when the database is active.

`doc/openapi.json` is gitignored because fuego rewrites it on every server start; commit it instead if you want spec changes reviewed in PRs.
