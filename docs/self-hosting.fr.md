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

`https://tickets.example.org` est un exemple : remplacez ce domaine partout,
y compris dans la Redirect URI Web et `HTBD_PUBLIC_URL`. Ne mettez pas de
barre oblique finale à `HTBD_PUBLIC_URL`.

1. **Créer le projet.** Dans la console Zitadel, ouvrez **Organization →
   Projects → New**, créez `HTB` et copiez son **Project ID** dans
   `HTBD_ZITADEL_AUDIENCE`. N’utilisez pas le projet système `ZITADEL`.
   [Guide des projets](https://zitadel.com/docs/guides/manage/console/projects-overview).
2. **Créer l’application CLI.** Dans ce projet, ouvrez **Applications → New**,
   nommez l’application `HTB CLI`, choisissez **Native**, puis **Device Code**.
   Si Zitadel impose un champ **Redirect URI**, indiquez
   `htb://oauth/callback`. HTB ne visite pas cette URI : le flux se termine
   sur la page de validation des appareils de Zitadel. Une fois l’application
   créée, copiez son **Client ID** dans `HTBD_ZITADEL_DEVICE_CLIENT_ID`.
   [Guide Device Code](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization).
3. **Créer l’application Web.** Toujours dans ce projet, ouvrez
   **Applications → New**, nommez l’application `HTB Web`, choisissez **Web**,
   puis **PKCE**. Saisissez exactement cette **Redirect URI** :
   `https://tickets.example.org/auth/callback`. C’est le callback réellement
   servi par HTB après connexion. Si Zitadel exige une **Post Logout Redirect
   URI**, saisissez `https://tickets.example.org/` ; HTB révoque actuellement
   sa session locale, sans appeler la déconnexion Zitadel. Une fois
   l’application créée, copiez son **Client ID** dans
   `HTBD_ZITADEL_WEB_CLIENT_ID`. Aucun Client Secret n’est utilisé par HTB
   pour ce flux PKCE.
   [Guide Web / PKCE](https://zitadel.com/docs/guides/integrate/login/oidc/login-users).
4. **Régler les jetons et les rôles.** Ouvrez `HTB CLI` → **Token Settings**,
   choisissez des **access tokens JWT**. Si vous utilisez le rôle global
   `superadmin`, créez-le dans le projet Zitadel, incluez les rôles dans le
   jeton et ne l’attribuez qu’aux administrateurs globaux. Les rôles `read`,
   `write` et `admin` des projets HTB restent gérés par HTB. Laissez
   **Check Role Assignment on Authentication** désactivé : un client invité
   dans HTB n’a pas besoin d’un rôle Zitadel. [Rôles et jetons Zitadel](https://zitadel.com/docs/guides/manage/console/projects-overview).
5. **Choisir l’accueil des utilisateurs.** S’ils doivent créer eux-mêmes leur
   compte, activez **Register allowed** dans le comportement de connexion et
   configurez les e-mails de vérification. Sinon, créez ou invitez-les dans
   Zitadel. Le portail HTB exige un e-mail vérifié dans l’ID token.
   [Accueil des utilisateurs Zitadel](https://zitadel.com/docs/guides/integrate/onboarding/end-users).

Le mécanisme présenté à l’utilisateur (mot de passe, passkey, fournisseur
externe, MFA) dépend de Zitadel. Le login hébergé peut proposer des
[passkeys](https://zitadel.com/docs/guides/integrate/login/hosted-login) ;
ne supposez pas qu’un code reçu par e-mail soit un premier facteur de
connexion. Le **code d’invitation HTB** sert uniquement à rejoindre un projet,
après la connexion Zitadel.

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
