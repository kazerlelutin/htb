# HTB — Headless Ticket Board

[English](README.md) · Français

HTB garde les tickets techniques de l’équipe dans la CLI. Les clients suivent
les US publiées, leur avancement et leurs échanges dans un portail web léger ;
ils peuvent aussi proposer des demandes. Zitadel gère la connexion, HTB les
accès aux projets et les tickets.

## Choisir son installation

| | Utiliser la version en ligne | Auto-héberger HTB |
| --- | --- | --- |
| Serveur | [htboard.xyz](https://htboard.xyz) | Votre domaine et votre infrastructure |
| À configurer | La CLI | PostgreSQL, Zitadel, HTTPS et HTB |
| Pour commencer | [Démarrage rapide](#utiliser-la-version-en-ligne) | [Guide d’auto-hébergement](docs/self-hosting.fr.md) |

### Utiliser la version en ligne

Installez la [CLI](#installer-la-cli), puis connectez-la au serveur hébergé :

```bash
htb config set-server https://htboard.xyz
htb auth login
htb project create --key SITE --name "Site web"
```

La connexion s’ouvre dans le navigateur via Zitadel. Si vous avez reçu une
invitation à un projet existant, utilisez `htb invite accept CODE` au lieu de
créer un projet.

Un client qui n’utilise pas la CLI peut ouvrir le
[portail client](https://htboard.xyz/login), se connecter et saisir un code
d’invitation. Le portail montre les US publiées et les demandes, pas le tableau
technique de l’équipe.

### Auto-héberger HTB

Il vous faut PostgreSQL, une instance Zitadel et une URL publique en HTTPS.
Dans la console Zitadel, créez un projet `HTB`, puis deux applications :

| Application | Type et flux | Redirect URI dans Zitadel | À copier dans HTB |
| --- | --- | --- | --- |
| HTB CLI | Native · Device Code | `htb://oauth/callback` **seulement si Zitadel impose une URI** ; le flux Device Code ne l’utilise pas | `HTBD_ZITADEL_DEVICE_CLIENT_ID` |
| HTB Web | Web · Authorization Code + PKCE | `https://tickets.example.org/auth/callback` | `HTBD_ZITADEL_WEB_CLIENT_ID` |

Remplacez `tickets.example.org` par votre domaine HTTPS. La Redirect URI Web
doit correspondre exactement à `HTBD_PUBLIC_URL` suivi de `/auth/callback`.
Si Zitadel demande une Post Logout Redirect URI, indiquez la racine de votre
site ; la déconnexion actuelle de HTB n’appelle pas celle de Zitadel. Copiez
le **Project ID** Zitadel dans `HTBD_ZITADEL_AUDIENCE` et l’URL de son issuer
dans `HTBD_ZITADEL_ISSUER`.

Suivez le [cheminement Zitadel](docs/self-hosting.fr.md#2-configurer-zitadel),
puis le reste du guide pour le fichier `.env`, le déploiement et les
vérifications. La même CLI se connecte ensuite à **votre** URL :

```bash
htb config set-server https://tickets.example.org
htb auth login
```

N’utilisez pas le domaine ou les identifiants Zitadel de la version hébergée
pour votre installation.

## Installer la CLI

Sous Linux, l’installateur vérifie la somme de contrôle et installe `htb`
dans `~/.local/bin`, sans `sudo` :

```bash
curl -fsSL https://github.com/kazerlelutin/htb/releases/latest/download/install.sh | sh
```

Ouvrez un nouveau terminal si la commande est introuvable. Les
[fichiers de release](https://github.com/kazerlelutin/htb/releases) peuvent
aussi être consultés avant l’installation.

## Travailler avec les tickets

```bash
htb auth status
htb project list
htb project use SITE
htb ticket create --title "Publier la page d’accueil" --type user_story
htb ticket list
htb ticket show SITE-1
htb ticket comment SITE-1 "Prêt pour la revue"
```

Le projet courant est conservé dans la configuration locale de la CLI. Les
commandes qui acceptent `--project` peuvent cibler un autre projet. Les
références sont numérotées par projet (`SITE-1`, `SITE-2`, …). Les descriptions
et commentaires acceptent le Markdown ; `htb ticket list --json` et `--csv`
sont disponibles pour les scripts.

Utilisez `htb help` ou `htb help ticket create` pour connaître les options.
Le [guide des commandes](https://htboard.xyz/commands) est aussi accessible
sur le web.

### Partager une US avec un client

```bash
htb ticket create --type technical_task --parent SITE-1 --title "Construire l’export"
htb ticket publish SITE-1
htb ticket client-comments SITE-1
htb ticket client-comment SITE-1 "L’export est prêt pour validation."
```

Seul un administrateur du projet peut publier ou masquer une US. Elles sont
privées par défaut ; `htb ticket unpublish SITE-1` en masque une à nouveau. Le
portail affiche son titre, sa description, son état et le nombre de tâches
techniques terminées sur le total. Sans tâche liée, il indique que l’US n’est
pas encore découpée. Le contenu des tâches et `htb ticket comments` ne sont pas
affichés dans le portail ; `client-comments` est une conversation distincte qui
y est visible. Un membre ayant le droit `read` peut toutefois consulter les
tickets techniques via la CLI ou l’API. Vérifiez le titre et la description
avant publication, y compris après une modification.

### Demandes client

Un administrateur de projet crée une invitation avec
`htb invite create --role read`. Après la connexion sur `/login`, le client
saisit ce code dans `/portal` et peut voir, proposer et commenter les demandes
du projet. L’équipe les traite séparément des tickets de travail :

```bash
htb request list
htb request show 7
htb request comment 7 "Pouvez-vous ajouter un exemple ?"
htb request status 7 needs_info
htb request link 7 SITE-12
```

`htb request link` rattache une demande à un ticket existant, sans le publier.
Si ce ticket est une US publiée, la demande mène vers elle dans le portail.
Le portail n’expose ni priorité, ni assignation, ni contenu des tâches ou
commentaires internes. L’authentification reste chez Zitadel ; l’invitation donne uniquement
accès au projet dans HTB.

## Développement

```bash
go test ./...
go vet ./...
go build ./cmd/htbd ./cmd/htb
```

Les tests d’intégration PostgreSQL demandent une **base de test dédiée** via
`HTBD_TEST_DATABASE_URL` ; ne les lancez jamais sur une base de production.
Consultez le [guide d’auto-hébergement](docs/self-hosting.fr.md) pour la
configuration du serveur.
