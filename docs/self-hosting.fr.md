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

### ChatGPT / MCP (facultatif)

HTB expose des outils MCP **en lecture seule** à `https://tickets.example.org/mcp`.
ChatGPT ne réutilise ni le Device Code ni un jeton de la CLI : il crée un
client OAuth PKCE éphémère par Dynamic Client Registration (DCR), puis la
personne se connecte directement chez Zitadel.

**Il n’y a donc pas d’URL de redirection MCP fixe à saisir dans Zitadel.** Ne
créez pas une troisième application `HTB MCP` et n’y copiez pas
`/auth/callback` : cette URI concerne uniquement le portail web. L’URL que
vous renseignez manuellement est `https://tickets.example.org/mcp`, dans
ChatGPT. Lors de la liaison, ChatGPT enregistre lui-même son client et son URI
de retour auprès de Zitadel par DCR.

1. Activez la **Dynamic Client Registration** et son mode **Open registration**
   (`allowUnauthenticated`) dans les paramètres **d’instance** de Zitadel. La
   version actuelle de la console peut ne pas exposer ce réglage : un compte
   `IAM_OWNER` peut alors l’activer via l’API :

   ```bash
   curl --request PUT https://id.example.org/v2/settings/security \
     --header 'Authorization: Bearer <jeton-d-un-IAM_OWNER>' \
     --header 'Content-Type: application/json' \
     --data '{"dynamicClientRegistration":{"enabled":true,"allowUnauthenticated":true}}'
   ```

   Remplacez `id.example.org` par l’issuer Zitadel. C’est le mode requis par
   les clients MCP qui s’enregistrent avant qu’une personne soit connectée.
   Conservez le rate limiting de Zitadel et limitez les URI de redirection aux
   domaines de confiance.
2. Zitadel crée le projet `ZITADEL DCR`. Copiez son **Project ID** dans
   `HTBD_ZITADEL_MCP_AUDIENCE`. Les jetons des clients DCR portent cette
   audience, différente de l’audience du projet HTB.
3. Définissez `HTBD_MCP_ENABLED=true` dans HTB. L’endpoint reste désactivé par
   défaut, afin de ne pas l’exposer avant cette configuration.
4. Gardez les access tokens au format **JWT** et vérifiez que les scopes OIDC
   `openid`, `profile` et `email` sont autorisés pour les clients DCR.
5. Après déploiement, dans ChatGPT activez le mode développeur, ajoutez une
   connexion avec l’URL HTTPS `https://tickets.example.org/mcp`, puis liez le
   compte quand ChatGPT affiche la page Zitadel.

Si votre politique Zitadel impose une allowlist précise des URI de redirection,
utilisez l’URI exacte affichée dans la page de gestion de cette connexion MCP
dans ChatGPT. Selon la prise en charge de l’identification d’issuer par votre
instance, elle est soit `https://chatgpt.com/connector_platform_oauth_redirect`,
soit une URI propre à la connexion sous
`https://chatgpt.com/connector/oauth/...`; ne la devinez pas et n’utilisez pas
`/auth/callback`.

Le proxy HTTPS doit également accepter et vérifier le certificat client mTLS
présenté par ChatGPT avant de transmettre `/mcp` à HTB. Cela authentifie le
client ChatGPT ; le jeton OAuth continue d’authentifier la personne. Ne rendez
pas les futures actions d’écriture disponibles avant cette vérification et une
conception de confirmation explicite.

[DCR Zitadel pour les clients MCP](https://zitadel.com/docs/guides/integrate/dynamic-client-registration) ·
[authentification MCP selon OpenAI](https://developers.openai.com/plugins/build/auth)

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
| `HTBD_ZITADEL_MCP_AUDIENCE` | Project ID du projet `ZITADEL DCR`. Requis pour accepter les jetons ChatGPT créés par DCR ; sinon HTB utilise l’audience principale. |
| `HTBD_MCP_ENABLED` | `true` pour exposer `/mcp` et la découverte OAuth ; reste `false` tant que DCR et le mTLS du proxy ne sont pas configurés. |
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
curl -fsS https://tickets.example.org/.well-known/oauth-protected-resource
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
htb ticket create --type user_story --title "Exporter les données"
htb ticket create --type technical_task --parent SITE-1 --title "Construire l’export"
htb ticket publish SITE-1
htb invite create --project SITE --role read
```

La clé du projet (`SITE`) est l’identifiant court utilisé dans les commandes et
les références de ticket (`SITE-1`). Vous pouvez optionnellement la préfixer par
un namespace (`ALICE/SITE`) pour permettre plusieurs projets avec la même clé
courte. Réservez un namespace avant de l’utiliser : `htb namespace claim ALICE`.
L’offre Community inclut un namespace ; les offres payantes peuvent augmenter
ce quota.

Transmettez le code d’invitation au client par votre canal habituel. Celui-ci
ouvre `https://tickets.example.org/login`, se connecte via votre Zitadel, puis
saisit le code dans `/portal`. Il voit l’US publiée, son avancement calculé
depuis les tâches terminées et sa conversation client ; le détail des tâches
n’est pas affiché dans le portail. Un membre `read` peut néanmoins le consulter
avec la CLI ou l’API. Il peut aussi proposer une demande. L’équipe la retrouve
avec `htb request list` et `htb request show ID`, puis peut la rattacher à
l’US avec `htb request link ID SITE-1`. Seuls les administrateurs du projet
peuvent publier une US.

## Exploitation

PostgreSQL est la seule donnée persistante de HTB : sauvegardez la base et
testez sa restauration. Gardez l’issuer Zitadel et la Redirect URI cohérents
avec `HTBD_PUBLIC_URL`. Si `/login` renvoie 404, vérifiez le Client ID Web ; si
le serveur refuse de démarrer, vérifiez aussi la clé de session et la
connectivité vers Zitadel. Pour désactiver le portail sans supprimer ses
données, videz `HTBD_ZITADEL_WEB_CLIENT_ID` puis redéployez.

### Procédure de sauvegarde, restauration et supervision

Planifiez une sauvegarde PostgreSQL quotidienne chiffrée en dehors du
conteneur HTB. Conservez une copie dans un emplacement distinct et appliquez
la durée de rétention de votre organisation. Exemple de sauvegarde logique :

```bash
pg_dump --format=custom --no-owner "$HTBD_DATABASE_URL" > htb-$(date +%F).dump
```

Au moins chaque trimestre, restaurez une sauvegarde récente dans une base
**vide et hors production** avec `pg_restore --clean --if-exists`, démarrez
HTBD dessus, puis contrôlez `/health`, la connexion, la liste d’un projet et la
lecture d’un ticket. Ne testez jamais une restauration sur la base de
production.

Supervisez `https://votre-domaine/health` depuis l’extérieur de votre
infrastructure. Alertez en cas de réponse différente de 200 et consignez la
révision Git déployée pour chaque incident. `/health` vérifie PostgreSQL et
convient au contrôle de disponibilité et de readiness.

### Demandes relatives aux données de compte

HTB délègue la gestion d’identité à Zitadel. Pour une demande d’accès,
d’export, de rectification ou d’effacement, vérifiez d’abord la personne via
son compte Zitadel, puis consignez la demande et son résultat. N’exportez ou
ne supprimez les données HTB qu’après sauvegarde et validation du propriétaire
concerné lorsqu’un projet est affecté. La suppression d’un compte implique les
sessions web, identifiants, appartenances, invitations et contenus publiés :
ne lancez pas de SQL de suppression improvisé en production. En l’absence de
parcours libre-service, traitez ces demandes via le contact de la politique de
confidentialité.

### Migrations de base de données

HTB applique automatiquement les migrations de base de données au démarrage du
serveur. La migration `0010_namespaced_project_keys.sql` permet les préfixes de
namespace optionnels dans les clés de projet (par exemple `ALICE/SITE`). La
migration suivante, `0011_namespace_reservations.sql`, réserve les préfixes
existants à leurs propriétaires et ajoute le quota de namespaces par offre. Si
vous mettez à jour depuis une version antérieure, les migrations s’exécuteront
automatiquement ; les projets existants restent valides. Vous pouvez vérifier
les migrations appliquées avec `SELECT version FROM schema_migrations ;`.
