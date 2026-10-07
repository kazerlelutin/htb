# Ticket — Recherche de tickets et d’US

## Problème

La CLI sait filtrer les tickets du projet courant, mais ne permet pas de
retrouver une expression dans plusieurs projets accessibles. Le portail ne
propose pas de recherche parmi les user stories visibles.

## Solution

- Ajouter une recherche textuelle par titre et description sur le portail du
  projet. Elle conserve les règles de visibilité existantes : les clients ne
  voient que les US publiées, tandis qu’un administrateur peut aussi retrouver
  ses brouillons.
- Ajouter `htb ticket list --all-projects` pour lancer les filtres existants,
  notamment `--query`, sur tous les projets accessibles. Sans cette option,
  la CLI conserve la recherche par projet courant ou par `--project KEY`.

## Critères d’acceptation

- Le portail recherche les US visibles du projet ouvert, par titre ou
  description, et conserve la requête entre les pages.
- Les résultats du portail ne révèlent ni tickets techniques ni US privées à
  un membre non administrateur.
- `htb ticket list --all-projects --query TEXTE` ne retourne que les tickets
  correspondant à TEXTE dans les projets accessibles.
- L’outil MCP `htb_list_tickets` accepte un projet facultatif et recherche tous
  les projets accessibles lorsqu’il est absent.
- `--project` et `--all-projects` sont incompatibles et la CLI l’explique.
- Les formats JSON et CSV reçoivent le même ensemble de résultats que la vue
  terminal.

## Scénarios

`features/client_stories.feature` et `features/cli_experience.feature`.
