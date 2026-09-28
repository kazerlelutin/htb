# Rendu Markdown dans le portail web

## Problème

Certaines descriptions de tickets, notamment celles saisies par ligne de
commande, contiennent leurs sauts de ligne sous forme de sequences `\n`.
Le portail les affichait littéralement et ne pouvait donc pas interpréter les
titres, listes et cases Markdown.

## Périmètre

Normaliser uniquement les fins de ligne encodées avant le rendu Markdown du
portail. Le contenu reste échappé et les liens conservent leur liste blanche
de schémas HTTP(S).

## Critères d’acceptation

- Une description avec `\n` affiche ses paragraphes, titres et listes.
- Le Markdown déjà écrit avec de vrais sauts de ligne reste inchangé.
- Du HTML ou une URL dangereuse fournis par une personne ne sont pas rendus
  comme du contenu actif.
