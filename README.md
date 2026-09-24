# HTB — Headless Ticket Board

HTB est un tableau de tickets pour terminal. Zitadel gère entièrement les
comptes, l'inscription, la connexion et les mots de passe. HTB ne voit qu'une
identité Zitadel déjà connectée et gère les projets et leurs droits.

## Site public, confidentialité et futures offres

Le site public est servi par HTB, avec une landing en français et anglais, le
guide de téléchargement, les [mentions légales](/mentions-legales), les
[CGU](/cgu) et la [politique de confidentialité](/privacy). La mesure
d'audience Ben-to ne se charge qu'après un consentement explicite dans le
bandeau de préférences ; le choix peut être modifié depuis le pied de page.

Les données de compte et de projet sont traitées par HTB et l'authentification
reste déléguée à Zitadel. Les coordonnées juridiques publiées doivent être
vérifiées avant toute mise en production.

Le schéma prépare des offres par compte avec un plafond de projets possédés.
La limite Community est initialisée à trois projets, mais aucune restriction
n'est appliquée tant que `HTBD_ENFORCE_PROJECT_LIMITS=false`. Lorsque cette
variable sera activée, le contrôle est effectué côté serveur sur chaque
création de projet ; un projet simplement rejoint par invitation ne compte pas
dans le quota.

## Installer la CLI sous Linux

Une seule commande installe la CLI dans le `PATH` utilisateur, sans `sudo` :

```bash
curl -fsSL https://github.com/kazerlelutin/htb/releases/latest/download/install.sh | sh
```

L'installateur vérifie le checksum, installe `htb` dans `~/.local/bin` et
configure Bash ou Zsh si nécessaire. Ouvrir un nouveau terminal ensuite.

## Le parcours utilisateur

```bash
htb config set-server https://tickets.example.org
htb auth login
```

La CLI ouvre automatiquement la page de connexion Zitadel dans le navigateur.
La personne peut s'y connecter ou créer son compte. Au retour, la CLI est
connectée et affiche ses projets.

Si aucun projet n'est accessible, elle propose simplement :

```bash
# Créer son premier projet : la personne devient admin de ce projet.
htb project create --key SITE --name "Site de l'association"

# Ou rejoindre un projet grâce à un code reçu.
htb invite accept <code>
```

Les commandes utiles au quotidien sont :

```bash
htb auth status          # état de connexion, projets et projet courant
htb project list         # projets accessibles
htb project status       # avancement de tous les projets accessibles
htb project use SITE     # change le projet courant
htb ticket list          # utilise le projet courant
```

Un projet courant est conservé dans le fichier de configuration local de la
CLI. Les commandes acceptent encore `--project` pour agir explicitement sur un
autre projet. Les références de tickets sont numérotées indépendamment dans
chaque projet, à partir de `KEY-1`.

## Référence CLI

La CLI décrit toutes ses commandes localement :

```bash
htb help
htb help ticket create
htb ticket update --help
```

| Besoin | Commande |
| --- | --- |
| Configurer / se connecter | `htb config set-server URL`, `htb auth login`, `htb auth status` |
| Gérer le projet | `htb project list`, `htb project status`, `htb project create --key KEY --name NAME`, `htb project use KEY` |
| Gérer l’équipe | `htb project members`, `htb project member set-role --user ID --role write`, `htb project member remove --user ID` |
| Organiser la roadmap | `htb feature create --key KEY --name NAME --due-date YYYY-MM-DD` |
| Créer un ticket | `htb ticket create --title TITRE --type user_story` |
| Créer une tâche d'US | `htb ticket create --type technical_task --parent SITE-1 --title TITRE` |
| Voir les tickets | `htb ticket list --status blocked --priority urgent`, `htb ticket list --label site --query export`, `htb ticket show SITE-1` |
| Suivre la conversation | `htb ticket show SITE-1`, `htb ticket comments SITE-1`, `htb ticket activity SITE-1` — les descriptions et commentaires Markdown sont formatés pour le terminal |
| Filtrer par fonctionnalité | `htb ticket list --feature newsletter` |
| Mettre à jour | `htb ticket update --version 2 --status in_progress SITE-2` |
| Collaborer | `htb ticket comment SITE-2 "Texte"`, `htb ticket claim SITE-2`, `htb ticket release SITE-2` |
| Historique | `htb ticket versions SITE-2`, `htb ticket restore --version 3 SITE-2 1` |
| Invitations | `htb invite create --role write`, `htb invite list`, `htb invite revoke ID`, `htb invite accept CODE` |
| Demandes client | `htb request list`, `htb request show 7`, `htb request comments 7`, `htb request comment 7 "Réponse"`, `htb request status 7 in_progress`, `htb request link 7 SITE-12` |

La sortie est conçue pour le terminal. Pour automatiser une liste, utiliser
`htb ticket list --json` ou `htb ticket list --csv`.

`htb project status` offre une vue portefeuille : pour chaque projet accessible,
il affiche une barre pour les US terminées, une autre pour tous les tickets
terminés, ainsi que le nombre de tickets dans chaque état. Les US affichent la
même barre de progression à partir de leurs tâches techniques, par exemple
`[██████░░░░] 2/3 tâches (66%)`.

Les titres, références, barres, états et messages d'erreur sont colorés
automatiquement dans un terminal. Définir `NO_COLOR=1` ou `HTB_COLOR=never`
pour les désactiver, et `HTB_COLOR=always` pour les forcer. Les sorties JSON et
CSV ne contiennent jamais de couleurs.

## Mettre Zitadel en place

Ces étapes sont à effectuer une seule fois dans la console Zitadel.

1. Dans **Settings → Login behavior**, activer **Register allowed** si les
   membres doivent créer eux-mêmes leur compte. Configurer aussi la livraison
   des e-mails de vérification si nécessaire.
2. Dans **Organization → Projects**, créer un projet nommé `HTB`. Relever son
   **Project ID** : il sera utilisé comme audience par HTB.
3. Dans ce projet, ouvrir **Roles** et créer le rôle applicatif `superadmin`
   si des personnes doivent administrer HTB dans son ensemble. L'attribuer
   uniquement à ces personnes ; tous les rôles de projet restent gérés par HTB.
4. Dans **Applications**, créer `HTB CLI` : choisir **Native**, puis
   activer **Device Code**. Copier son **Client ID**. La console demande aussi
   une Redirect URI pour toute application native : saisir
   `htb://oauth/callback`. Elle ne sera pas appelée par le flux Device Code de
   HTB ; elle satisfait uniquement le réglage obligatoire de l'application
   native.
5. Dans les réglages du projet et de l'application CLI :
   - activer l'assertion des rôles à l'authentification ;
   - inclure les rôles utilisateur dans l'**access token** ;
   - choisir des access tokens **JWT** ;
   - laisser désactivé **Check Role Assignment on Authentication** : un membre
     invité n'a pas besoin d'un rôle Zitadel pour entrer dans HTB.

La documentation Zitadel sur l'[inscription](https://zitadel.com/docs/guides/integrate/onboarding/end-users),
le [Device Code](https://zitadel.com/docs/guides/integrate/login/oidc/device-authorization)
et les [rôles dans les tokens](https://zitadel.com/docs/guides/manage/console/projects-overview)
complète ces étapes.

Pour activer le socle de session navigateur, créer aussi une application
**Web** Zitadel avec **Authorization Code + PKCE**, enregistrer la Redirect URI
`https://<votre-domaine>/auth/callback`. Le parcours de connexion effectivement
présenté à l’utilisateur dépend de la configuration Zitadel. Sa page hébergée
ne fournit pas, à elle seule, un lien magique par email comme premier facteur.
Pour une connexion sans mot de passe gérée par Zitadel, activer les
[passkeys dans son login hébergé](https://zitadel.com/docs/guides/integrate/login-ui/login-app).
HTB ne reçoit qu’une identité OIDC dont l’adresse email a été vérifiée ; les
jetons OAuth restent côté serveur. Après connexion, la page `/portal` affiche
les projets accessibles. Une nouvelle identité peut y saisir un code
d’invitation sans installer la CLI ; la connexion et la vérification de son
email restent entièrement chez Zitadel. La route `/portal/api/projects`
renvoie seulement les noms et clés de ses projets.

## Espace client léger

Une personne invitée ouvre `/login`, se connecte via Zitadel, puis saisit dans
`/portal` le code d’invitation transmis par l’administrateur du projet. Dans
chaque projet, elle voit les demandes et leur état simplifié : reçue, en
cours, besoin d’information ou terminée. Elle peut proposer une demande et
commenter la conversation. Les descriptions et commentaires acceptent un
Markdown limité, affiché sans HTML actif. Le portail ne montre ni priorité,
ni responsable, ni estimation ou état interne des tickets de travail.

Une proposition reste une *demande client* distincte. L’équipe la traite via
`htb request list`, `htb request show`, `htb request comment` et `htb request
status` ; elle peut créer un ticket de travail normal, puis relier la demande
avec `htb request link ID TICKET-REF`. Le ticket lié n’est pas affiché dans le
portail. Un membre du projet ayant le rôle `read` peut consulter, proposer et
commenter ; le rôle `write` est nécessaire pour changer l’état ou lier un
ticket. L’accès est revérifié à chaque requête. Les formulaires du portail
sont protégés par un jeton anti-CSRF associé à la session navigateur.

Reporter ensuite les valeurs relevées dans la configuration CapRover :

```dotenv
HTBD_ZITADEL_ISSUER=https://<votre-instance-zitadel>
HTBD_ZITADEL_AUDIENCE=<Project ID de HTB>
HTBD_ZITADEL_DEVICE_CLIENT_ID=<Client ID de HTB CLI>
HTBD_ZITADEL_WEB_CLIENT_ID=<Client ID de l’application Web, optionnel>
HTBD_WEB_SESSION_KEY=<au-moins-32-octets-aleatoires>
HTBD_PUBLIC_URL=https://htb.ben-to.fr
HTBD_ENFORCE_PROJECT_LIMITS=false
```

## Configurer et lancer HTB

```bash
cp .env.example .env
# renseigner PostgreSQL, Zitadel et les Client IDs CLI/Web si le portail client est activé
set -a && source .env && set +a
go run ./cmd/htbd
```

Les migrations sont appliquées au démarrage sur une base PostgreSQL vide. La
CLI ne demande jamais l'issuer, l'audience ou le Client ID : elle les récupère
depuis `https://tickets.example.org/auth/device-config`.

## Déploiement CapRover

Le dépôt contient le `Dockerfile` et `captain-definition`. Dans CapRover,
définir les variables de `.env.example` avec les valeurs de production. Le
portail client est désactivé tant que `HTBD_ZITADEL_WEB_CLIENT_ID` est absent.
Lorsqu’il est activé, la Redirect URI `https://<votre-domaine>/auth/callback`
doit être enregistrée dans Zitadel et `HTBD_WEB_SESSION_KEY` doit contenir au
moins 32 octets aléatoires, distincts des autres secrets. La Redirect URI
`htb://oauth/callback` reste un paramètre interne requis par l'application
native Zitadel ; HTB ne l'expose pas. Ne jamais exposer les clés de comptes de
service.

HTB est stateless : PostgreSQL est la seule donnée persistante. Les sauvegardes
et tests de restauration doivent donc couvrir la base PostgreSQL.

## Développement

```bash
go test ./...
go vet ./...
go build ./cmd/htbd ./cmd/htb
```

Pour le parcours HTTP complet avec PostgreSQL (création et lecture d'un ticket
avec tags), utiliser une **base de test dédiée**, jamais la base locale ou de
production :

```bash
HTBD_TEST_DATABASE_URL='postgres://htb:change-me@127.0.0.1:5432/htb_test?sslmode=disable' \
  go test ./internal/httpapi -run TestTicketLifecycleOverHTTP
```
