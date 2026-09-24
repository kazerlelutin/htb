# Self-host HTB

English · [Français](self-hosting.fr.md)

Use this guide for your own HTB server. For the managed instance, follow the
[hosted quick start](../README.md#use-the-hosted-service) instead.

## Requirements

- PostgreSQL reachable from HTB.
- A Zitadel instance reachable from HTB and users' browsers.
- A public HTTPS URL such as `https://tickets.example.org`, routed to HTB.
- Docker or CapRover; Go 1.25 if running from source.

Zitadel handles accounts, email verification, and sign-in. HTB needs no SMTP
server. HTTPS is required for the portal's secure session cookie.

## 1. Create the Zitadel applications

In the console of **your** Zitadel instance:

1. Under **Organization → Projects**, create a project named `HTB`. Copy its
   **Project ID** for `HTBD_ZITADEL_AUDIENCE`. Do not use Zitadel's built-in
   system project. [Zitadel project guide](https://zitadel.com/docs/guides/manage/console/projects-overview).
2. Under **Applications**, create a **Native** app named `HTB CLI` and select
   **Device Code**. Copy its **Client ID** for
   `HTBD_ZITADEL_DEVICE_CLIENT_ID`. If the console requires a native Redirect
   URI, enter `htb://oauth/callback`; HTB's Device Code flow does not call it.
   [Device Code guide](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization).
3. For browser access, create a separate **Web** app named `HTB Web` with
   **Authorization Code + PKCE**. Register the exact Redirect URI
   `https://tickets.example.org/auth/callback`, replacing the domain with
   your `HTBD_PUBLIC_URL`. Copy its **Client ID** for
   `HTBD_ZITADEL_WEB_CLIENT_ID`. The HTB PKCE setup does not use a Client
   Secret. [Web / PKCE guide](https://zitadel.com/docs/guides/integrate/login/oidc/login-users).
4. Configure the CLI app to issue **JWT access tokens**. If you use the
   optional global `superadmin` role, create it in the Zitadel project,
   assign it sparingly, and include roles in access tokens. Project roles
   (`read`, `write`, `admin`) are managed by HTB. Leave **Check Role
   Assignment on Authentication** off so an HTB project invitee can sign
   in without a Zitadel role. [Role settings](https://zitadel.com/docs/guides/manage/console/projects-overview).
5. If people may create their own accounts, enable **Register allowed** in
   Zitadel's login behavior and configure its verification emails. Otherwise
   create or invite them in Zitadel. The HTB portal requires a verified email
   claim. [Zitadel onboarding](https://zitadel.com/docs/guides/integrate/onboarding/end-users).

Zitadel chooses the actual sign-in method: password, passkey, identity
provider, or MFA. A project invitation code in HTB is **not** a sign-in code.

## 2. Configure HTB

Copy [`.env.example`](../.env.example) to an untracked `.env` file and replace
its example values:

```bash
cp .env.example .env
openssl rand -hex 32
```

Use the random output as `HTBD_WEB_SESSION_KEY` when enabling the Web app.
Keep it secret and separate from Client IDs.

| Variable | Value |
| --- | --- |
| `HTBD_DATABASE_URL` | PostgreSQL URL reachable from HTB. `127.0.0.1` in the example only works when PostgreSQL shares HTB's network environment. Use TLS for a remote database. |
| `HTBD_ZITADEL_ISSUER` | Your Zitadel issuer URL, for example `https://id.example.org`. |
| `HTBD_ZITADEL_AUDIENCE` | Zitadel Project ID from step 1. |
| `HTBD_ZITADEL_DEVICE_CLIENT_ID` | Native / Device Code app Client ID. Required for CLI sign-in. |
| `HTBD_ZITADEL_WEB_CLIENT_ID` | Web / PKCE app Client ID. Leave empty to disable `/login` and the portal. |
| `HTBD_WEB_SESSION_KEY` | Random secret of at least 32 characters, required when the Web Client ID is set. |
| `HTBD_PUBLIC_URL` | Exact public HTTPS origin; HTB appends `/auth/callback` for Zitadel. |
| `HTBD_RELEASE_URL` | GitHub Releases page used by the site's download link. |

`HTBD_LISTEN_ADDR` defaults to `:8080`. `HTBD_ZITADEL_SUPERADMIN_*` applies
only to the global role.

## 3. Deploy and check

For CapRover, deploy this repository using its `captain-definition` and
`Dockerfile`, set the environment variables in the app, and enable HTTPS.
Do not commit your `.env` file.

For Docker behind an HTTPS reverse proxy:

```bash
docker build -t htb:local .
docker run --name htb --env-file .env -p 8080:8080 htb:local
```

The container must reach PostgreSQL and Zitadel. Adjust the database hostname
for the container network. HTB applies database migrations on startup. For a
source run with Go 1.25:

```bash
set -a; source .env; set +a
go run ./cmd/htbd
```

After deployment, check your domain:

```bash
curl -fsS https://tickets.example.org/health
curl -fsS https://tickets.example.org/auth/device-config
```

`/health` returns `{"status":"ok"}`. The device-config endpoint returns
the public issuer, audience, and CLI Client ID. With the Web app enabled,
`/login` redirects to Zitadel and then `/portal`. HTB keeps a revocable
30-day application session; Zitadel remains responsible for credentials and
sign-in.

## 4. Connect users

Install the [CLI](../README.md#install-the-cli) on your workstation and use
**your** server URL:

```bash
htb config set-server https://tickets.example.org
htb auth login
htb project create --key SITE --name "Website"
htb invite create --project SITE --role read
```

Send the invitation code through your usual channel. The client opens
`https://tickets.example.org/login`, signs in with your Zitadel, and enters
the code in `/portal`. They can then follow and comment on project requests.
The team works with internal tickets through the CLI and handles client
requests with `htb request`.

## Operations

PostgreSQL holds HTB's persistent data: back it up and test restoration. If
`/login` returns 404, check the Web Client ID. If startup fails, also check
the session key and Zitadel connectivity. To turn off the portal without
deleting its data, clear `HTBD_ZITADEL_WEB_CLIENT_ID` and redeploy.
