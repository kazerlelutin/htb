# Auto-héberger HTB

[English](self-hosting.md) · Français

Ce guide concerne votre propre serveur HTB. Pour la version hébergée, suivez
le [démarrage rapide](../README.fr.md#utiliser-la-version-en-ligne).

HTB comprend un serveur HTTP (`htbd`), une base PostgreSQL et une CLI (`htb`).
Zitadel gère les comptes, l’inscription, la vérification des e-mails et la
connexion. HTB conserve les projets, les droits et les tickets ; le portail
navigateur ne présente que les demandes client. Pour que les cookies du portail
fonctionnent, publiez le serveur sous **HTTPS**.

## 1. Préparer l’infrastructure

- Une base PostgreSQL accessible depuis le conteneur ou le processus HTB.
- Une instance Zitadel accessible par HTB et par les navigateurs.
- Un domaine HTTPS, par exemple `https://tickets.example.org`, qui pointe vers
  HTB derrière un proxy inverse. La valeur de `HTBD_PUBLIC_URL` doit être
  exactement cette origine, sans chemin final.
- Docker ou CapRover pour le déploiement ; Go 1.25 pour un lancement depuis les
  sources.

La base, les sauvegardes, le certificat HTTPS et les e-mails envoyés par
Zitadel sont sous votre responsabilité. HTB n’envoie pas de message de
connexion et n’a pas besoin de serveur SMTP.

## 2. Configurer Zitadel

Remplacez `tickets.example.org` par votre domaine HTTPS dans tout ce guide.
Donnez cette origine à `HTBD_PUBLIC_URL`, sans barre oblique finale.

### Projet

1. Dans la console Zitadel, ouvrez **Organization → Projects → New**.
2. Créez un projet `HTB` (pas le projet système `ZITADEL`).
3. Copiez son **Project ID** dans `HTBD_ZITADEL_AUDIENCE`.

[Guide des projets Zitadel](https://zitadel.com/docs/guides/manage/console/projects-overview)

### Application CLI

1. Ouvrez le projet `HTB`, puis **Applications → New**.
2. Nommez l’application `HTB CLI` et choisissez **Native**.
3. Choisissez **Device Code**.
4. Si une **Redirect URI** est imposée, saisissez `htb://oauth/callback`.
5. Créez l’application.
6. Copiez son **Client ID** dans `HTBD_ZITADEL_DEVICE_CLIENT_ID`.

Le flux Device Code n’utilise pas cette Redirect URI : la connexion se
termine sur la page de validation des appareils de Zitadel.
[Guide Device Code](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization)

### Application Web

1. Dans le même projet, ouvrez **Applications → New**.
2. Nommez l’application `HTB Web` et choisissez **Web**.
3. Choisissez **PKCE** comme méthode d’authentification.
4. Saisissez cette **Redirect URI** : `https://tickets.example.org/auth/callback`.
5. Si nécessaire, saisissez cette **Post Logout Redirect URI** : `https://tickets.example.org/`.
6. Créez l’application.
7. Copiez son **Client ID** dans `HTBD_ZITADEL_WEB_CLIENT_ID`.

La Redirect URI Web doit correspondre exactement à `HTBD_PUBLIC_URL` suivi de
`/auth/callback`. HTB n’utilise pas de Client Secret et n’appelle pas la
déconnexion Zitadel.
[Guide Web / PKCE](https://zitadel.com/docs/guides/integrate/login/oidc/login-users)

### Jetons et rôles

1. Ouvrez `HTB CLI` → **Token Settings**.
2. Choisissez des access tokens **JWT**.
3. Laissez **Check Role Assignment on Authentication** désactivé pour les personnes invitées.
4. Si nécessaire, créez le rôle `superadmin` dans le projet Zitadel.
5. Si vous utilisez `superadmin`, incluez les rôles dans les access tokens.
6. Attribuez `superadmin` seulement aux administrateurs globaux.

Les rôles de projet (`read`, `write`, `admin`) sont gérés par HTB.
[Guide des rôles Zitadel](https://zitadel.com/docs/guides/manage/console/projects-overview)

### Utilisateurs

1. Pour permettre l’inscription, activez **Register allowed** dans les réglages de connexion Zitadel.
2. Configurez les e-mails de vérification Zitadel pour les nouvelles inscriptions.
3. Sinon, invitez les utilisateurs depuis la console Zitadel.
4. Vérifiez que leur adresse e-mail est confirmée avant l’accès au portail HTB.

[Guide Zitadel sur l’inscription](https://zitadel.com/docs/guides/integrate/onboarding/end-users).
Le code d’invitation HTB donne accès à un projet après connexion ; ce n’est pas
un code de connexion.

## 3. Renseigner la configuration HTB

Copiez [`.env.example`](../.env.example) vers un fichier `.env` non commité et
remplacez les valeurs d’exemple :

```bash
cp .env.example .env
openssl rand -hex 32
```

Copiez le résultat de la seconde commande dans `HTBD_WEB_SESSION_KEY`. Il doit
rester secret, distinct des Client IDs, et faire au moins 32 caractères.

| Variable | Valeur à fournir |
| --- | --- |
| `HTBD_DATABASE_URL` | URL PostgreSQL accessible par le serveur HTB. L’adresse `127.0.0.1` de l’exemple ne fonctionne que si PostgreSQL est dans le même environnement réseau. Utilisez TLS pour une base distante. |
| `HTBD_ZITADEL_ISSUER` | URL de l’issuer Zitadel, par exemple `https://id.example.org`, sans chemin ajouté. |
| `HTBD_ZITADEL_AUDIENCE` | Project ID Zitadel du projet HTB. |
| `HTBD_ZITADEL_DEVICE_CLIENT_ID` | Client ID de l’application Native / Device Code. Nécessaire à la connexion CLI. |
| `HTBD_ZITADEL_WEB_CLIENT_ID` | Client ID de l’application Web / PKCE. Laisser vide désactive `/login` et le portail. |
| `HTBD_WEB_SESSION_KEY` | Secret aléatoire pour les états OIDC et formulaires du portail. À définir si le Client ID Web est renseigné. |
| `HTBD_PUBLIC_URL` | Origine HTTPS publique exacte, utilisée pour construire la Redirect URI Web. |
| `HTBD_RELEASE_URL` | Page GitHub Releases utilisée par le lien de téléchargement du site. |

`HTBD_LISTEN_ADDR` vaut `:8080` par défaut. Les variables
`HTBD_ZITADEL_SUPERADMIN_*` servent uniquement au rôle global.

## 4. Déployer et vérifier

Avec CapRover, déployez ce dépôt (`captain-definition` et `Dockerfile` sont
fournis), renseignez les mêmes variables dans l’application et activez HTTPS.
Ne copiez pas un `.env` contenant des secrets dans Git.

Avec Docker derrière votre proxy HTTPS :

```bash
docker build -t htb:local .
docker run --name htb --env-file .env -p 8080:8080 htb:local
```

Le conteneur doit pouvoir joindre PostgreSQL et Zitadel ; adaptez
`HTBD_DATABASE_URL` à son réseau. Les migrations PostgreSQL s’exécutent au
démarrage de `htbd`. Pour un essai depuis les sources, utilisez
`set -a; source .env; set +a; go run ./cmd/htbd` avec Go 1.25.

Après mise en ligne, vérifiez :

```bash
curl -fsS https://tickets.example.org/health
curl -fsS https://tickets.example.org/auth/device-config
```

La première route doit répondre `{"status":"ok"}` ; la seconde expose
l’issuer, l’audience et le Client ID **public** de la CLI. Si l’application
Web est configurée, `https://tickets.example.org/login` doit rediriger vers
Zitadel. Après connexion, HTB ouvre `/portal`. Les cookies de la session
applicative HTB durent 30 jours et sont révocables à la déconnexion ; les
identifiants, méthodes de connexion et jetons restent gérés par Zitadel.

## 5. Connecter l’équipe et inviter un client

Installez la [CLI](../README.fr.md#installer-la-cli), puis utilisez **votre**
domaine, pas celui de l’instance en ligne :

```bash
htb config set-server https://tickets.example.org
htb auth login
htb project create --key SITE --name "Site web"
htb invite create --project SITE --role read
```

Transmettez le code d’invitation au client par votre canal habituel. Celui-ci
ouvre `https://tickets.example.org/login`, se connecte via votre Zitadel, puis
saisit le code dans `/portal`. Il peut alors voir les demandes du projet, en
proposer et commenter. L’équipe continue à gérer les tickets de travail dans
la CLI ; `htb request` sert à qualifier les demandes client.

## Exploitation

PostgreSQL est la seule donnée persistante de HTB : sauvegardez la base et
testez sa restauration. Gardez l’issuer Zitadel et la Redirect URI cohérents
avec `HTBD_PUBLIC_URL`. Si `/login` renvoie 404, vérifiez le Client ID Web ; si
le serveur refuse de démarrer, vérifiez aussi la clé de session et la
connectivité vers Zitadel. Pour désactiver le portail sans supprimer ses
données, videz `HTBD_ZITADEL_WEB_CLIENT_ID` puis redéployez.
