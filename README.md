# HTB — Headless Ticket Board

English · [Français](README.fr.md)

HTB is a ticket tracker built around a CLI and an API. Teams work in the
terminal; clients can view projects, propose requests, and comment in a small
web portal. Zitadel handles sign-in. HTB manages project access and tickets.

## Choose your setup

| | Use the hosted service | Self-host HTB |
| --- | --- | --- |
| Server | [htb.ben-to.fr](https://htb.ben-to.fr) | Your own domain and infrastructure |
| You configure | The CLI; no Zitadel or PostgreSQL setup | PostgreSQL, Zitadel, HTTPS, and HTB |
| Start here | [Hosted quick start](#use-the-hosted-service) | [Self-hosting guide](docs/self-hosting.md) |

### Use the hosted service

Install the [CLI](#install-the-cli), then connect it to the hosted server:

```bash
htb config set-server https://htb.ben-to.fr
htb auth login
htb project create --key SITE --name "Website"
```

Sign-in opens in your browser through Zitadel. You do **not** need to set up a
Zitadel application, database, or server. If someone invites you to an
existing project, use `htb invite accept CODE` instead of creating one.

Clients who do not use the CLI can open the
[client portal](https://htb.ben-to.fr/login), sign in, and enter a project
invitation code. The portal is for requests and conversation, not the team's
internal work board.

### Self-host HTB

You need PostgreSQL, a Zitadel instance, and a public HTTPS URL. In your
Zitadel console, create a project named `HTB`, then create two applications
inside it:

| Application | Type and flow | Redirect URI in Zitadel | Copy into HTB |
| --- | --- | --- | --- |
| HTB CLI | Native · Device Code | `htb://oauth/callback` **only if Zitadel requires a URI**; the Device Code flow does not use it | `HTBD_ZITADEL_DEVICE_CLIENT_ID` |
| HTB Web | Web · Authorization Code + PKCE | `https://tickets.example.org/auth/callback` | `HTBD_ZITADEL_WEB_CLIENT_ID` |

Replace `tickets.example.org` with your HTTPS domain. The Web Redirect URI
must equal `HTBD_PUBLIC_URL` plus `/auth/callback`, exactly. If Zitadel asks
for a Post Logout Redirect URI, use your site's root URL; HTB's current
logout does not call Zitadel's logout endpoint. Copy the Zitadel **Project
ID** to `HTBD_ZITADEL_AUDIENCE` and its issuer URL to
`HTBD_ZITADEL_ISSUER`.

Follow the [Zitadel walkthrough](docs/self-hosting.md#1-configure-zitadel),
then the rest of the self-hosting guide for `.env`, deployment, and checks.
Your users then connect the same CLI to **your** URL:

```bash
htb config set-server https://tickets.example.org
htb auth login
```

Do not use the hosted service's Zitadel IDs or domain for your installation.

## Install the CLI

On Linux, the release installer verifies the checksum and installs `htb`
under `~/.local/bin` without `sudo`:

```bash
curl -fsSL https://github.com/kazerlelutin/htb/releases/latest/download/install.sh | sh
```

Open a new shell if the command is not found. You can also inspect the
[release files](https://github.com/kazerlelutin/htb/releases) before installing.

## Work with tickets

```bash
htb auth status
htb project list
htb project use SITE
htb ticket create --title "Publish the homepage" --type user_story
htb ticket list
htb ticket show SITE-1
htb ticket comment SITE-1 "Ready for review"
```

The current project is stored in your local CLI configuration. Commands that
support `--project` can target another project. Ticket references are numbered
per project (`SITE-1`, `SITE-2`, …). Descriptions and comments support
Markdown; `htb ticket list --json` and `--csv` are available for scripts.

Run `htb help` or `htb help ticket create` for command options. The
[command guide](https://htb.ben-to.fr/commands) is also available on the web.

### Client requests

A project admin creates an invitation with `htb invite create --role read`.
After signing in to `/login`, the client enters that code in `/portal` and can
view, propose, and comment on requests for the project. The team handles
these separately from internal tickets:

```bash
htb request list
htb request show 7
htb request comment 7 "Could you add an example?"
htb request status 7 needs_info
htb request link 7 SITE-12
```

`htb request link` connects a client request to an existing work ticket. The
client portal does not expose internal priority, assignment, or ticket status.
Authentication remains with Zitadel; a project invitation only grants access
inside HTB.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/htbd ./cmd/htb
```

PostgreSQL integration tests need a **dedicated test database** through
`HTBD_TEST_DATABASE_URL`; never point them at a production database. See
[the self-hosting guide](docs/self-hosting.md) for server configuration.
