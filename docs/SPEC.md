# Je viens aider — specification

This document describes what the application does and how the code is
organised. It is the reference for reviews: the code should do what is
written here, in the place written here, and nothing much more.

## 1. Purpose

A parents' association (APE) organises school events (*fête de l'école*,
*marché de Noël*). It needs volunteers on **stands** (bar, crêpes, games…),
each stand split into **time slots** that need a minimum number of people.

The app lets organisers publish the planning, and parents sign up for slots
in a few seconds, without creating an account. It is a small, self-hosted
alternative to Doodle or Framadate, specialised for this job.

Everything visible to users is in French. Code, comments and docs are in
English.

## 2. Users and how they are identified

There are no accounts and no passwords for users. Identity is carried by
secret links (random 256-bit tokens, base64url).

| Role | Identified by | Can |
|---|---|---|
| **Visitor** | nothing | see an open event's planning, sign up |
| **Volunteer** | personal link `/e/<slug>/me/<token>`, also kept in a cookie (`v<eventID>`, path `/e/<slug>`, HttpOnly, SameSite=Lax, 1 year) | see and cancel their slots, change their name and phone, replace their link |
| **Organiser** | admin link `/a/<token>` | everything about one event |
| **Super-admin** | shell access to the server | list, show and delete any event (`benevole events`) |
| **Creator** | the creation password, if `BENEVOLE_CREATE_PASSWORD` is set | create (or duplicate) events |

## 3. Domain

- **Event**: title, description, location, optional date, status, organiser
  e-mail (mandatory), "notify me of sign-ups" flag, public slug, admin token.
  - Status: `draft` (planning not public, no sign-ups), `open` (sign-ups
    allowed), `closed` (read-only for volunteers; the organiser can still add
    people).
- **Stand**: belongs to an event; name, description, colour (`#rrggbb`),
  display position. At most **50** stands per event.
- **Slot**: belongs to a stand; start and end (local wall-clock time, no
  timezone), minimum people (≥ 1), optional maximum. At most **500** slots
  per event.
- **Volunteer**: belongs to an event; name (required), phone (optional), edit
  token. No e-mail is ever asked or stored.
- **Sign-up**: a volunteer on a slot, with an optional comment. One per
  (slot, volunteer).

Derived notions (computed, not stored):

- **Fill status** of a slot: `empty` (0 people), `partial` (below minimum),
  `ok` (minimum reached), `full` (maximum reached).
- **Missing**: people still needed to reach the minimum, per slot, stand and
  event.
- **Overlap**: two slots whose time ranges intersect.
- **Public name**: "Marie Dupont" is shown as "Marie D." on public pages.

## 4. Features

### 4.1 Creating an event (home page)

- Form: title (required), date, location, description, organiser e-mail
  (required, strict format), "e-mail me at each sign-up" (checked by
  default), and the creation password when configured.
- On success the organiser lands on the admin page. The admin link is also
  e-mailed to them (when e-mail is enabled and within the limits, §6).

### 4.2 Public event page `/e/<slug>`

- Header: title (a link back to the overview), date, location, description,
  progress bar and "Il reste encore N places !" (or "Tous les créneaux sont
  couverts, merci !").
- Draft: only a "not open yet" notice. Closed: a "closed" notice.
- **Overview** ("Vue d'ensemble"):
  - Wide screens: a table, one row per stand, one column per distinct time
    range. Each row header shows the stand's colour dot, name and description
    cut after two lines; a "Voir plus" link (only when the text is cut) opens
    the full description.
  - Phones: a timeline, one group per time range, one card per slot.
  - A cell shows count/minimum, coloured by fill status, the public names,
    and "Vous" on the visitor's own slots ("Vous ⚠" if two of their slots
    overlap). A free cell opens the sign-up form.
  - Filter "Créneaux à compléter" (`?todo=1`) hides slots that reached their
    minimum.
- **"Mes créneaux"** (`/e/<slug>/me/<token>`), next to "Vue d'ensemble" once
  the browser knows the volunteer:
  - "Mon planning": the same table, restricted to the volunteer's own slots,
    with a summary (number of slots, total time, stands, overlap warning) and
    an "Annuler" button in each cell (while the event is open).
  - "Ajouter à mon agenda" (`/e/<slug>/me/<token>/agenda.ics`), when the
    volunteer has at least one slot: an iCalendar file with one entry per
    slot (stand and event names, the stand description, the location and the
    public event link) and two reminders, one day and one hour before. Times
    are written without a time zone, like the stored times. Each entry has a
    stable UID (event, slot, volunteer), so a second import updates the
    entries that still exist. A cancelled or deleted slot stays in the
    calendar app. Not available for a draft event (404), like the sign-up
    form; a closed event still gives its file.
  - Personal link with "Copier" and "Changer de lien".
  - "Mes coordonnées": change name and phone (while the event is open).
  - "Ce n'est pas moi": forget this browser's cookie.
- **Stand view** `/e/<slug>/stands/<id>`: name and full description, as a
  dialog (htmx) or a page; a "Fermer" button.

### 4.3 Signing up `/e/<slug>/slots/<id>/signup`

- Dialog (htmx) or page with: stand, day, time, count, who is already there,
  the stand's full description, then name, optional phone, optional comment.
  Pre-filled for a known volunteer.
- Warns (does not block) when the slot overlaps another of the volunteer's
  slots; the button then reads "M'inscrire quand même".
- Refused: event not open, slot full, already signed up on that slot.
- The capacity check and the insert happen in one transaction, so two people
  cannot take the last place.
- First sign-up: creates the volunteer, sets the cookie, shows the personal
  link ("welcome"). Later sign-ups reuse the volunteer.
- If the browser remembers someone with a **different name**, a new volunteer
  is created instead (shared computer, or a personal link shared by mistake).
  Names are compared ignoring case and spacing.
- Honeypot field against bots.

### 4.4 Organiser page `/a/<token>`

- Status: draft / open / closed.
- Event settings, including the organiser e-mail and the notification flag.
- Stands: add, edit, delete, move up/down.
- Slots: a generator ("from 14:00 to 17:00, every 60 min, 2 people" → three
  slots), and per-slot edit (times, min, max) and delete.
- Volunteers: full names and phones, add someone by hand (allowed whatever
  the status, capacity still applies), remove a sign-up.
- CSV export (`;`, UTF-8 BOM for Excel, one line per sign-up and one per
  missing person, formula injection neutralised).
- Planning as a PDF to print (`/a/<token>/planning.pdf`): one A4 page per
  stand, with a line per volunteer and an empty line per missing person.
  Lines and text shrink (down to a readable minimum) to keep a stand on one
  page; a longer stand continues on more pages, without cutting a slot when
  it fits on one page.
- Erase personal data (keeps stands and slots).
- Duplicate the event (stands and slots, shifted to a new date; asks for the
  creation password when one is configured).
- Delete the event.
- Shows when the event will be deleted automatically.
- Sends the event's links to the organiser again, when e-mail is set up.

### 4.5 E-mails (to organisers only)

- On creation and duplication: the admin and public links. The organiser can
  have them sent again from the admin page ("Renvoyer les liens par e-mail"),
  to the current organiser address; this counts in the same limits.
- On each sign-up and cancellation, if the organiser asked: who, which slot,
  phone and comment, and what is still missing.
- Sent in the background through SMTP (STARTTLS required by default, or
  implicit TLS; `none` only towards a loopback or private address). Temporary
  failures are retried (30 s, 2 min, 10 min) without blocking other mails;
  permanent (5xx) failures are not retried.
- Without SMTP configured, messages are only logged (recipient and subject,
  never the body, which holds secret links).
- Links in e-mails always come from `BENEVOLE_BASE_URL` (required when SMTP
  is set), never from the request. Links shown on pages fall back to the
  request's host when it is not set.

### 4.6 Automatic deletion

- Every hour, events whose last day (latest of: date, last slot, creation)
  is more than `BENEVOLE_RETENTION_DAYS` days ago (default 60) are deleted
  with everything in them. `0`/`off` keeps events forever.
- Dates are accepted up to two years ahead and, when deletion is on, back to
  max(365, retention) days.

### 4.7 Command line

- `benevole` — run the web server.
- `benevole events list|show|delete` — super-admin tool on the database.
- `benevole mail-test <address>` — send one test e-mail with the SMTP
  settings and print the server's error.
- `benevole -healthcheck` — for container health checks.

## 5. Configuration

Flags and environment variables, documented in the README and
`.env.example`: listen address, database file, public URL, trust proxy
headers, retention, creation password, SMTP settings, "open creation"
opt-out, and the four e-mail limits. An empty variable counts as unset.
Invalid values stop the program at startup with a clear message; nothing
silently falls back to a default.

## 6. Security and privacy rules

- **Secrets**: admin and personal tokens are never logged (paths are
  redacted), never sent in Referer (`Referrer-Policy: same-origin`), and
  pages that show them are `Cache-Control: no-store`.
- **Calendar file**: the `.ics` file of a volunteer holds no token and no
  data about volunteers (no name, phone or comment). Text fields are escaped
  and control characters and line breaks are removed, so a text typed by an
  organiser cannot add a calendar property. The response is `no-store`.
- **CSRF**: cross-site POSTs are refused (`Sec-Fetch-Site`, else `Origin`).
- **XSS**: all output goes through templ, which escapes it; user text is never marked
  as trusted HTML. Strict CSP (`script-src 'self'`). The JavaScript never
  builds HTML from text. htmx `eval` is off.
- **Abuse limits**: 64 KB request bodies; POST rate limit per IP (per /64 for
  IPv6; behind a trusted proxy, the last `X-Forwarded-For` entry); 50 stands
  and 500 slots per event; bounded memory for every in-memory counter.
- **E-mail abuse**: with SMTP on, the server refuses to start without a
  creation password (unless `BENEVOLE_OPEN_CREATION=true`). Limits, all
  configurable: 3 mails with an event's links (creation, duplication or
  resend) per address per day, 30 per hour in total,
  20 sign-up notifications per event per hour (then one "paused" notice),
  300 mails per hour overall. No CR/LF in headers.
- **Privacy**: public pages show "Prénom N." only; phones are visible to the
  organiser and the volunteer only; `noindex`; personal data is erased on
  request and deleted with the event.
- **HTTPS**: HSTS and `Secure` cookies when served over HTTPS.

## 7. Architecture

One Go binary, no CGO, all assets embedded. SQLite through
`modernc.org/sqlite`. Server-rendered HTML with templ components, plus htmx
for dialogs and partial updates; every feature also works without
JavaScript.

```
cmd/benevole/        main.go: wiring, server lifecycle, background loops
                     config.go, smtp.go, retention.go, maillimits.go: reading
                     flags and environment
                     events.go, mailtest.go: the CLI subcommands
internal/model/      domain types and pure rules (Slot.Check, fill status,
                     statuses, public names); BuildPlanning turns stands +
                     slots + sign-ups into a ready-to-render view
internal/slotgen/    pure slot generator (start, end, step, min, max)
internal/store/      SQLite access: one file per table group (events,
                     stands, slots, signups), cleanup.go for automatic
                     deletion, embedded migrations tracked with
                     PRAGMA user_version
internal/mail/       SMTP sender, message formatting, background queue
internal/ical/       iCalendar writer: text escaping, line folding, reminders
internal/web/        HTTP handlers, middleware, templates, static files
  server.go          routes and middleware chain
  public.go          home, event page, sign-up, volunteer self-service
  admin.go           organiser pages
  forms.go           reading and validating forms; userMessage turns
                     errors into French sentences for users
  render.go          template loading and helpers, flash messages
  middleware.go      security headers, CSRF, body limit, rate limit, logs
  notify.go          organiser e-mails
  mailbudget.go      e-mail limits
  export.go          CSV export
  agenda.go          the personal calendar file (.ics) of a volunteer
  pdf.go             planning PDF (layout, then drawing with gopdf)
  *.templ            pages (templ); *_templ.go is generated by `make generate`
  view.go            helpers shared by the pages (slot wording, urgent slots)
  static/            CSS, JS, htmx, Atkinson Hyperlegible font (OFL)
deploy/              systemd unit, Caddy and nginx examples
```

Dependency direction: `cmd` → `web`, `store`, `mail` → `model`. `model`
imports nothing from the project, `slotgen` only `model`. `store` knows
nothing about HTTP; `web` holds no SQL. Errors from `store`, `model` and
`slotgen` are plain Go errors; only `web` writes French.

### Data

Tables `events`, `stands`, `slots`, `volunteers`, `signups`, all with
`ON DELETE CASCADE` from the event down. Times are stored as
`YYYY-MM-DD HH:MM` local wall-clock text. Migrations live in
`internal/store/migrations/NNN_name.sql` and run in order at startup.

### Request flow

`middleware chain → route → withEvent / withVolunteer / withAdmin (loads the
event, checks the token, sets no-store) → handler → store → model.BuildPlanning
→ template`. State changes answer with a 303 redirect and a flash message
(POST/redirect/GET). Validation errors on the sign-up form re-render the form.

## 8. Quality expectations

The code should stay small and readable by one person in an afternoon.

- Plain Go and the standard library first; a dependency needs a real reason.
- Short functions with one job; names that say what they do; a comment
  explaining *why* where the reason is not obvious.
- No layer that only forwards calls, no interface with a single
  implementation unless tests need it, no framework.
- Rules live in one place, not repeated across handlers: form reading and
  validation in `forms.go`, SQL in `store`, domain rules and everything a
  view needs to decide (can I sign up here? is this mine?) in `model`.
  Templates only display; they do not compute rules.
- Every feature and every security rule above has a test; tests read like
  the scenario they check.
- `gofmt`, `go vet`, `staticcheck` and `govulncheck` are clean in CI.

## 9. Non-goals

- User accounts, logins, roles beyond the secret links.
- E-mails or SMS to volunteers.
- Several organisations or tenants; any APE runs its own instance.
- Translations: the interface is French only.
- A JavaScript front-end framework or a build step.
