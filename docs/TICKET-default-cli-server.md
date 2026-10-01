# Ticket — Serveur CLI par défaut

## Problème

La CLI ne connaît aucun serveur tant que la personne n’exécute pas
`htb config set-server https://htboard.xyz`. Ce prérequis est inutile pour la
version hébergée et allonge le parcours de première connexion.

## Solution

Utiliser `https://htboard.xyz` comme serveur par défaut lorsqu’aucune adresse
n’est enregistrée localement. `htb config set-server URL` reste le mécanisme
explicite pour remplacer cette adresse, notamment pour une instance
auto-hébergée.

## Critères d’acceptation

- Une CLI sans fichier de configuration utilise `https://htboard.xyz`.
- Une adresse enregistrée avec `htb config set-server URL` reste prioritaire.
- Le parcours documenté de la version hébergée commence par `htb auth login`.
- La documentation auto-hébergée conserve la commande de changement de
  serveur.

Scénario : `features/cli_experience.feature`.
