# <img src="internal/web/static/logo.svg" alt="" width="40" height="40" align="top"> Je viens aider

[![build](https://github.com/arnaudmorin/jeviensaider/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/arnaudmorin/jeviensaider/actions/workflows/build.yml)

A small web app that helps a parents' association (APE) find volunteers for
school events such as the *fête de l'école* or the *marché de Noël*.

It works like a simpler Doodle or Framadate, organised by **stand** and
**time slot**. For example, *Pêche aux canards* from 14:00 to 17:00 with
2 people per hour gives 3 slots and 6 places to fill.

The interface is in French. The whole app is one Go binary, and its data lives
in a single SQLite file.

## Features

**For organisers**
- Create an event and get a **secret admin link**, with no account to create.
  The organiser's e-mail is required: the link is also sent there, and can be
  **sent again** from the admin page (a lost e-mail, or a new address).
- Get an **e-mail for each sign-up or cancellation** (who, which slot, what
  is still missing), which can be turned off per event.
- Add stands, with a description of what volunteers do there, then generate
  their slots in one step: start, end, slot length and number of people.
- Follow progress on a dashboard that shows who is signed up, with their
  contact details, and how many places are still missing.
- Open and close sign-ups.
- Add a volunteer by hand.
- Export to CSV; the file opens directly in French Excel.
- Download the planning as a PDF to print and post on the day: one A4 page
  per stand, with empty lines to write in the missing names. Lines shrink to
  keep a stand on one page; a very long stand continues on the next pages.
- Duplicate an event to reuse it next year.
- Erase personal data after the event (GDPR). Whole events are also deleted
  automatically 60 days after they end, and the admin page shows the date.
  A private instance can turn this off and keep its events forever.

**For parents**
- One **overview** of the whole event: a table of stands × hours, coloured
  red / orange / green, shown as a list grouped by hour on phones, and
  "Il reste encore N places !" at the top. A filter shows only the slots
  still to fill.
- Each stand's description under its name, cut after two lines; "Voir plus"
  opens the whole text. The sign-up form shows it too.
- Sign up in a few seconds with just a name; a phone number is optional, and
  no e-mail is asked or stored. There is no account: the browser remembers
  them, and a personal link lets them manage their slots from another device.
  A link shared by mistake can be replaced with a new one.
- **"Mes créneaux"**: the same table with only their own slots, an "Annuler"
  button on each, and a warning when two of them are at the same time.
- An **"Ajouter à mon agenda"** button that downloads their slots as an
  `.ics` file for any calendar app. Each entry has a reminder one day and one
  hour before. The file does not contain the personal link, and it is not
  available while the event is a draft. Importing it again updates the
  entries of the slots that still exist and does not add copies. A slot that
  the volunteer cancels, or that an organiser deletes, stays in the calendar
  app until the volunteer removes it there. The times have no time zone, so a
  calendar shows the same clock time as the event page.

## Quick start

Requires Go ≥ 1.26; with an older Go, the `go` command downloads the right
toolchain by itself. No C toolchain is needed: the SQLite driver is pure Go.

```sh
make run      # http://localhost:8080, database ./benevole.db
make test     # unit + HTTP tests
make build    # ./benevole, a static binary
make generate # generate the pages' Go code (the other targets do it first)
```

Pages are written with [templ](https://templ.guide). Their generated Go code
(`*_templ.go`) is not committed: run `make generate` (or
`go tool templ generate`) once after cloning and after editing a `.templ`
file, before a plain `go build` or `go test`. The `make` targets, CI and the
Dockerfile do it for you.

Open http://localhost:8080, create an event, and keep the admin link it gives
you.

Prebuilt binaries for `linux-amd64`, `linux-arm64` (Raspberry Pi 4/5) and
`linux-armv7` are attached to every
[build on `master`](https://github.com/arnaudmorin/jeviensaider/actions/workflows/build.yml)
as downloadable artifacts. They can also be built locally with
`make build-arm64` or `make build-armv7`.

## Configuration

| Flag | Environment variable | Default | Purpose |
|---|---|---|---|
| `-addr` | `BENEVOLE_ADDR` | `:8080` | Listen address |
| `-db` | `BENEVOLE_DB` | `benevole.db` | SQLite file, created if missing |
| `-base-url` | `BENEVOLE_BASE_URL` | *(from request)* | Public URL used in shareable links, e.g. `https://jeviensaider.fr` |
| `-trust-proxy` | `BENEVOLE_TRUST_PROXY` | `false` | Trust `X-Forwarded-*` headers when behind Caddy or nginx |
| `-retention-days` | `BENEVOLE_RETENTION_DAYS` | `60` | Delete events this many days after they end; `0` or `off` keeps them forever |
| — | `BENEVOLE_CREATE_PASSWORD` | *(none)* | Password required to create an event |

On a public server, set `BENEVOLE_CREATE_PASSWORD` so that only the APE can
create events. It is read from the environment only, so it never shows up in
`ps` output. It is also asked for when an organiser duplicates an event.

**E-mails.** The app e-mails organisers: the admin link when an event is
created or duplicated, or when they ask for it again, then, unless they turn it off, one message per sign-up
or cancellation. Volunteers never receive e-mails. Configure an SMTP server
with these variables (environment only):

| Environment variable | Default | Purpose |
|---|---|---|
| `BENEVOLE_SMTP_HOST` | *(none)* | SMTP server; empty disables sending (messages are only logged, without their content) |
| `BENEVOLE_SMTP_PORT` | `587` | `587` (STARTTLS) or `465` (TLS) |
| `BENEVOLE_SMTP_TLS` | `starttls`, or `tls` on 465 | `starttls`, `tls`, or `none` for a relay on this machine or a private network, without authentication |
| `BENEVOLE_SMTP_USERNAME` / `BENEVOLE_SMTP_PASSWORD` | *(none)* | Credentials, if the server requires them |
| `BENEVOLE_SMTP_FROM` | `Je viens aider <noreply@…>` | Sender, by default on the domain of `BENEVOLE_BASE_URL` |
| `BENEVOLE_OPEN_CREATION` | `false` | Allow e-mails without `BENEVOLE_CREATE_PASSWORD` (see below) |

With SMTP enabled, `BENEVOLE_BASE_URL` is required: links in e-mails are
built from it, never from the request. The connection is always encrypted
(STARTTLS is required, not optional) unless `none` is chosen, and a password
is never sent over an unencrypted connection. `none` is refused unless the
SMTP server's address is a loopback or private one. Messages are sent in the
background. If the server is unavailable they are retried for about
15 minutes without holding up the others, and a message the server rejects
(an unknown address, for example) is not retried. Settings are checked at
startup.

Creating an event sends a mail to the address the visitor types. So with
e-mails enabled, the server refuses to start without
`BENEVOLE_CREATE_PASSWORD`; otherwise anybody could use it to e-mail anyone.
On a private network, `BENEVOLE_OPEN_CREATION=true` lifts this. Mails are
limited in any case. Each limit is a number, or `0` / `unlimited` for no
limit; an invalid value stops the server:

| Environment variable | Default | Limit |
|---|---|---|
| `BENEVOLE_MAIL_CREATED_PER_ADDRESS_PER_DAY` | `3` | Mails with an event's links (creation, duplication, resend) to one address, per day |
| `BENEVOLE_MAIL_CREATED_PER_HOUR` | `30` | Mails with an event's links to all addresses, per hour |
| `BENEVOLE_MAIL_SIGNUPS_PER_EVENT_PER_HOUR` | `20` | Sign-up notifications for one event, per hour; past it the organiser gets one "notifications en pause" message |
| `BENEVOLE_MAIL_PER_HOUR` | `300` | All mails, per hour |

The counters are kept in memory and start again from zero when the server
restarts.

To check the settings, send a test e-mail. It reads the same variables and
prints the SMTP server's error if sending fails:

```sh
benevole mail-test you@example.org
docker compose exec app benevole mail-test you@example.org   # with Docker
```

**Keeping events forever.** By default, events and their volunteers'
contact details are deleted 60 days after the event, which suits a public
instance. For a private instance that wants to keep its history, set
`BENEVOLE_RETENTION_DAYS=0` (or `off`) in `.env`: nothing is deleted
automatically, the admin page no longer announces a deletion date, and old
events stay editable. Organisers can still delete an event or erase its
personal data from the admin page. An invalid value stops the server with an
error rather than falling back to 60 days.

## Super-admin command line

The same binary includes a tool to manage every event directly in the
database, for example to delete one before it expires. Anyone who can run it
on the server can see and delete all events, so there is no password.

```sh
benevole events list                  # all events, with counts and expiry date
benevole events show <event>          # details, public and admin links
benevole events delete <event>...     # delete events and all their data (asks first; -y to skip)
```

`<event>` is a slug (as in `/e/<slug>`) or an ID from `list`. The tool reads
`BENEVOLE_DB`, `BENEVOLE_BASE_URL` and `BENEVOLE_RETENTION_DAYS` like the
server, or the `-db`, `-base-url` and `-retention-days` flags. It is safe to
use while the server is running.

With Docker:

```sh
docker compose exec app benevole events list
docker compose exec app benevole events delete fete-de-l-ecole-ab12cd34
```

`show` prints the admin link, which is handy when an organiser has lost it.

## Docker

```sh
cp .env.example .env          # set BENEVOLE_BASE_URL and BENEVOLE_CREATE_PASSWORD
docker compose pull && docker compose up -d   # prebuilt image
# or: docker compose up -d --build            # build it locally
```

Every push to `master` publishes a multi-arch image (amd64, arm64, armv7) to
`ghcr.io/arnaudmorin/jeviensaider`. It is tagged `latest` and with the
commit's short SHA (`sha-xxxxxxx`) for pinning a version.

- The app listens on `127.0.0.1:8080` (change `PORT` in `.env`). Put a reverse
  proxy in front of it for HTTPS.
- The database is stored in the `data` volume, at `/data/benevole.db`.
- The image is small (about 15 MB) and based on distroless. It runs as a non-root user
  with a read-only root filesystem, and has a built-in health check
  (`benevole -healthcheck`).

To back up the database from the volume:

```sh
docker compose cp app:/data/benevole.db ./backup-$(date +%F).db
```

Stop the app first (`docker compose stop`) to be sure the copy is consistent,
or use the `sqlite3 .backup` command below on the host.

## Deployment without Docker

1. Copy the binary to the server as `/usr/local/bin/benevole`.
2. Install the [systemd unit](deploy/benevole.service) and set
   `BENEVOLE_BASE_URL` in it.
3. Put a reverse proxy in front of the app for HTTPS: [Caddy](deploy/Caddyfile)
   (automatic certificates) or [nginx](deploy/nginx.conf) (with certbot).
   Either way, set `BENEVOLE_TRUST_PROXY=true` and keep the app listening on
   `127.0.0.1` only.

The nginx template forwards the client's address and protocol, redirects
HTTP to HTTPS, and keeps the secret admin and personal links out of its
access log (their tokens are replaced with `***`, and the Referer, which can
contain them, is not logged).

**Backups.** The database is a single file in WAL mode. This command takes a
consistent copy while the server is running, for example from a daily cron job:

```sh
sqlite3 /var/lib/benevole/benevole.db ".backup '/var/backups/benevole-$(date +%F).db'"
```

## Under the hood

```
cmd/benevole/      entry point: configuration, server, CLI subcommands
internal/model/    domain types and rules, fill status, planning view
internal/slotgen/  splits a time range into slots
internal/store/    SQLite access, embedded migrations, sign-up transaction
internal/mail/     SMTP sender and background queue
internal/ical/     iCalendar writer (escaping, line folding) for the .ics file
internal/web/      handlers, middleware, CSV, PDF and calendar exports,
                   pages as templ components (*.templ), static files (htmx)
```

[`docs/SPEC.md`](docs/SPEC.md) describes in detail what the app does and how
the code is organised.

- **No overbooking.** Each sign-up checks capacity and inserts the row in a
  single `BEGIN IMMEDIATE` transaction. A test runs 20 concurrent sign-ups for
  the last places.
- **Secret links.** Admin and personal links carry 256-bit random tokens.
  Tokens are redacted from logs, `Referrer-Policy: same-origin` stops them
  leaking to other sites, and admin pages are never cached.
- **Protection.**
  - Cross-site POSTs are rejected using the `Sec-Fetch-Site` and `Origin`
    headers.
  - POSTs are rate-limited per IP (per /64 for IPv6). Behind a proxy, only
    the last `X-Forwarded-For` entry, the one the proxy adds, is trusted.
  - Request bodies are capped at 64 KB, and an event has at most 50 stands
    and 500 slots, so no single request or event can exhaust the server.
  - Over HTTPS, cookies are `Secure` and HSTS is sent. Pages that can show
    personal data are never cached, including by htmx's history cache.
  - Signing up under a different name from the one the browser remembers
    creates a new volunteer, so a shared personal link or a family computer
    never exposes or renames someone else's sign-ups.
  - The sign-up form has a honeypot field to catch bots.
  - A strict CSP is sent.
  - The CSV export guards against formula injection.
- **Automatic cleanup.** At startup and then every hour, the app deletes
  every event whose last day is more than `BENEVOLE_RETENTION_DAYS` days ago,
  together with its stands, slots and volunteers. An event's last day is the
  latest of its date, its last slot and its creation date, so an event that
  is still being prepared is never deleted.
- **E-mails.** Links come from `BENEVOLE_BASE_URL`, so a forged `Host`
  header cannot turn them into links to another site. Headers are encoded
  and line breaks refused, so names cannot inject headers. E-mails require a
  creation password (or an explicit opt-out), and every kind of mail is
  rate-limited, so the site cannot be used to flood an inbox. A rejected
  address or a down SMTP server cannot block or drop the other messages.
- **Privacy.** Volunteers give only a name and, optionally, a phone number;
  no e-mail is stored. Public pages show only "Prénom N.", phone numbers are
  visible to the organisers only, and pages are marked `noindex`.

## Continuous integration

The [GitHub Actions workflow](.github/workflows/build.yml) runs on every push
or pull request to `master`, and once a week:

- `gofmt`, `go vet`, [`staticcheck`](https://staticcheck.dev), the tests with
  the race detector, and [`govulncheck`](https://go.dev/doc/security/vuln/) to catch known
  vulnerabilities in Go and the dependencies (the weekly run catches newly
  published ones);
- the three Linux binaries;
- the Docker image: built only for pull requests, built and pushed to
  `ghcr.io` for pushes to `master`. Only that job may write packages.

Actions are pinned to commit SHAs and the Docker base images to digests, so
a moved tag cannot change what runs. [Dependabot](.github/dependabot.yml)
proposes updates for them and for the Go modules every week.
