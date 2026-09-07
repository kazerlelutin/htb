# HTB — Headless Ticket Board

HTB est un tableau de tickets pour terminal. Zitadel gère entièrement les
comptes, l'inscription, la connexion et les mots de passe. HTB ne voit qu'une
identité Zitadel déjà connectée et gère les projets et leurs droits.

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
htb project use SITE     # change le projet courant
htb ticket list          # utilise le projet courant
```

Un projet courant est conservé dans le fichier de configuration local de la
CLI. Les commandes acceptent encore `--project` pour agir explicitement sur un
autre projet.

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
| Gérer le projet | `htb project list`, `htb project create --key KEY --name NAME`, `htb project use KEY` |
| Organiser la roadmap | `htb feature create --key KEY --name NAME --due-date YYYY-MM-DD` |
| Créer un ticket | `htb ticket create --title TITRE --type user_story` |
| Créer une tâche d'US | `htb ticket create --type technical_task --parent SITE-1 --title TITRE` |
| Voir les tickets | `htb ticket list`, `htb ticket show SITE-1` |
| Filtrer par fonctionnalité | `htb ticket list --feature newsletter` |
| Mettre à jour | `htb ticket update --version 2 --status in_progress SITE-2` |
| Collaborer | `htb ticket comment SITE-2 "Texte"`, `htb ticket claim SITE-2`, `htb ticket release SITE-2` |
| Historique | `htb ticket versions SITE-2`, `htb ticket restore --version 3 SITE-2 1` |
| Invitations | `htb invite create --role write`, `htb invite accept CODE` |

La sortie est conçue pour le terminal. Pour automatiser une liste, utiliser
`htb ticket list --json` ou `htb ticket list --csv`.

Les US affichent leur progression à partir de leurs tâches techniques, par
exemple `2/3 tâches (66%)`. Les états sont colorés automatiquement dans un
terminal ; définir `NO_COLOR=1` ou `HTB_COLOR=never` pour les désactiver, et
`HTB_COLOR=always` pour les forcer.

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

Reporter ensuite les trois valeurs relevées dans la configuration CapRover :

```dotenv
HTBD_ZITADEL_ISSUER=https://<votre-instance-zitadel>
HTBD_ZITADEL_AUDIENCE=<Project ID de HTB>
HTBD_ZITADEL_DEVICE_CLIENT_ID=<Client ID de HTB CLI>
```

## Configurer et lancer HTB

```bash
cp .env.example .env
# renseigner PostgreSQL, Zitadel et le Client ID HTB CLI
set -a && source .env && set +a
go run ./cmd/htbd
```

Les migrations sont appliquées au démarrage sur une base PostgreSQL vide. La
CLI ne demande jamais l'issuer, l'audience ou le Client ID : elle les récupère
depuis `https://tickets.example.org/auth/device-config`.

## Déploiement CapRover

Le dépôt contient le `Dockerfile` et `captain-definition`. Dans CapRover,
définir les variables de `.env.example` avec les valeurs de production. Aucune
URL de redirection publique HTB, secret de session ni client Web Zitadel n'est
nécessaire. La Redirect URI `htb://oauth/callback` reste un paramètre interne
requis par l'application native Zitadel ; HTB ne l'expose pas. Ne jamais
exposer les clés de comptes de service.

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
