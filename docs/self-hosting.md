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

## 1. Configure Zitadel

Replace `tickets.example.org` with your HTTPS domain throughout this guide.
Set `HTBD_PUBLIC_URL` to that origin, without a trailing slash.

### Project

1. Open **Organization → Projects → New** in your Zitadel console.
2. Create a project named `HTB` (not the built-in `ZITADEL` project).
3. Copy its **Project ID** to `HTBD_ZITADEL_AUDIENCE`.

[Zitadel project guide](https://zitadel.com/docs/guides/manage/console/projects-overview)

### CLI application

1. Open the `HTB` project, then **Applications → New**.
2. Name the application `HTB CLI` and select **Native**.
3. Select **Device Code**.
4. If a **Redirect URI** is required, enter `htb://oauth/callback`.
5. Create the application.
6. Copy its **Client ID** to `HTBD_ZITADEL_DEVICE_CLIENT_ID`.

Device Code does not use that Redirect URI; sign-in finishes on Zitadel's
device verification page. [Zitadel Device Code guide](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization)

### Web application

1. In the same project, open **Applications → New**.
2. Name the application `HTB Web` and select **Web**.
3. Select **PKCE** as the authentication method.
4. Set **Redirect URI** to `https://tickets.example.org/auth/callback`.
5. If required, set **Post Logout Redirect URI** to `https://tickets.example.org/`.
6. Create the application.
7. Copy its **Client ID** to `HTBD_ZITADEL_WEB_CLIENT_ID`.

The Web Redirect URI must equal `HTBD_PUBLIC_URL` + `/auth/callback` exactly.
HTB does not use a Client Secret or call Zitadel's logout endpoint.
[Zitadel Web / PKCE guide](https://zitadel.com/docs/guides/integrate/login/oidc/login-users)

### Tokens and roles

1. Open `HTB CLI` → **Token Settings**.
2. Select **JWT** access tokens.
3. Leave **Check Role Assignment on Authentication** disabled for project invitees.
4. If you need a global admin, create the `superadmin` role in Zitadel.
5. If you use `superadmin`, include roles in access tokens.
6. Assign `superadmin` only to global admins.

Project roles (`read`, `write`, `admin`) are managed by HTB.
[Zitadel role settings](https://zitadel.com/docs/guides/manage/console/projects-overview)

### ChatGPT / MCP (optional)

HTB exposes **read-only** MCP tools at `https://tickets.example.org/mcp`.
ChatGPT does not reuse Device Code or a CLI token: it creates a PKCE OAuth
client through Dynamic Client Registration (DCR), then the person signs in
directly with Zitadel.

**There is no fixed MCP redirect URI to enter in Zitadel.** Do not create a
third `HTB MCP` application or copy `/auth/callback` into it: that URI belongs
only to the web portal. The URL you enter manually is
`https://tickets.example.org/mcp` in ChatGPT. When the account is linked,
ChatGPT registers its own client and callback URI with Zitadel through DCR.

1. Enable **Dynamic Client Registration** and **Open registration**
   (`allowUnauthenticated`) in Zitadel **instance** security settings. The
   current Console version may not expose this setting; an `IAM_OWNER` can
   enable it through the API:

   ```bash
   curl --request PUT https://id.example.org/v2/settings/security \
     --header 'Authorization: Bearer <IAM_OWNER-access-token>' \
     --header 'Content-Type: application/json' \
     --data '{"dynamicClientRegistration":{"enabled":true,"allowUnauthenticated":true}}'
   ```

   Replace `id.example.org` with the Zitadel issuer. MCP clients must register
   before a person has authenticated. Keep Zitadel rate limiting enabled and
   restrict redirect URIs to trusted domains.
2. Zitadel creates a `ZITADEL DCR` project. Copy its **Project ID** into
   `HTBD_ZITADEL_MCP_AUDIENCE`. DCR client tokens use this audience rather
   than the HTB project audience.
3. Set `HTBD_MCP_ENABLED=true` in HTB. The endpoint is off by default so it
   is not exposed before this configuration is complete.
4. Keep access tokens as **JWTs** and ensure `openid`, `profile`, and `email`
   OIDC scopes are allowed for DCR clients.
5. After deploying, enable developer mode in ChatGPT, add the HTTPS endpoint
   `https://tickets.example.org/mcp`, and link the account when ChatGPT shows
   the Zitadel sign-in page.

If your Zitadel policy requires an exact redirect URI allowlist, use the URI
shown on that MCP connection's management page in ChatGPT. Depending on issuer
identification support, it is either
`https://chatgpt.com/connector_platform_oauth_redirect` or a
connection-specific URI under `https://chatgpt.com/connector/oauth/...`; do
not guess it or use `/auth/callback`.

Your HTTPS proxy must also accept and verify the mTLS client certificate
presented by ChatGPT before forwarding `/mcp` to HTB. This authenticates the
ChatGPT client; OAuth still authenticates the person. Do not expose future
write actions before this check and an explicit confirmation design are in
place.

[Zitadel DCR for MCP clients](https://zitadel.com/docs/guides/integrate/dynamic-client-registration) ·
[OpenAI MCP authentication](https://developers.openai.com/plugins/build/auth)

### Users

1. To allow self-registration, enable **Register allowed** in Zitadel's login behavior.
2. Configure Zitadel's verification emails for new registrations.
3. Otherwise, invite users from the Zitadel console.
4. Make sure users verify their email before opening the HTB portal.

[Zitadel onboarding guide](https://zitadel.com/docs/guides/integrate/onboarding/end-users).
An HTB project invitation code grants project access after sign-in; it is not
a sign-in code.

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
| `HTBD_ZITADEL_MCP_AUDIENCE` | Project ID of the `ZITADEL DCR` project. Required to accept DCR-created ChatGPT tokens; HTB otherwise uses the primary audience. |
| `HTBD_MCP_ENABLED` | `true` to expose `/mcp` and OAuth discovery; keep it `false` until DCR and proxy mTLS are configured. |
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
curl -fsS https://tickets.example.org/.well-known/oauth-protected-resource
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
htb ticket create --type user_story --title "Export data"
htb ticket create --type technical_task --parent SITE-1 --title "Build the export"
htb ticket publish SITE-1
htb invite create --project SITE --role read
```

The project key (`SITE`) is the short identifier used in commands and ticket
references (`SITE-1`). You can optionally prefix it with a namespace
(`ALICE/SITE`) to allow multiple projects with the same short key. Reserve a
namespace before using it: `htb namespace claim ALICE`. The Community plan
includes one namespace; paid plans can increase this quota.

Send the invitation code through your usual channel. The client opens
`https://tickets.example.org/login`, signs in with your Zitadel, and enters
the code in `/portal`. They see the published story, its progress from
completed technical tasks, and its separate client conversation; task details
are not shown in the portal. A `read` member can still access those details
with the CLI or API. They can also propose requests. Use `htb request list` and
`htb request show ID` to read them, then `htb request link ID SITE-1` to
connect one to the story. Only project admins can publish stories.

## Operations

PostgreSQL holds HTB's persistent data: back it up and test restoration. If
`/login` returns 404, check the Web Client ID. If startup fails, also check
the session key and Zitadel connectivity. To turn off the portal without
delete its data, clear `HTBD_ZITADEL_WEB_CLIENT_ID` and redeploy.

### Backup, restore, and monitoring runbook

Schedule a daily encrypted PostgreSQL backup outside the HTB container. Keep a
copy in a separate location and retain it according to your organisation's
policy. A typical logical backup is:

```bash
pg_dump --format=custom --no-owner "$HTBD_DATABASE_URL" > htb-$(date +%F).dump
```

At least once per quarter, restore a recent backup into an **empty,
non-production** database with `pg_restore --clean --if-exists`, start HTBD
against it, and check `/health`, login, a project list, and a ticket read.
Never test a restore against the production database.

Monitor `https://your-domain/health` from outside your infrastructure. Alert on
non-200 responses and record the deployed Git revision with each incident.
`/health` checks PostgreSQL connectivity and is appropriate for readiness and
external availability monitoring.

### Account data requests

HTB delegates identity management to Zitadel. For an access, export,
correction, or erasure request, first verify the requester through the Zitadel
account, then record the request and its outcome. Export or remove HTB data
only after a backup exists and the project owner has approved any project-level
impact. Account removal requires revoking web sessions, credentials,
memberships, invitations, and authored content according to retention duties;
do not run ad-hoc deletion SQL in production. Until a self-service workflow
exists, handle requests through the publisher contact in the privacy notice.

### Database migrations

HTB automatically applies database migrations when the server starts. The
migration `0010_namespaced_project_keys.sql` enables optional namespace prefixes
in project keys (e.g., `ALICE/SITE`). The following
`0011_namespace_reservations.sql` reserves existing prefixes to their project
owners and adds the plan namespace quota. If you are upgrading from a previous
version, the migrations will run automatically; existing projects remain valid.
You can verify applied migrations with `SELECT version FROM schema_migrations;`.
