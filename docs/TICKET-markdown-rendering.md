# Ticket #23 — Lecture Markdown dans la CLI

Suivi GitHub : [#23](https://github.com/kazerlelutin/htb/issues/23).

## Problème

Les modèles de descriptions de tickets sont déjà écrits en Markdown, mais la
CLI les affiche littéralement. La lecture de critères d’acceptation, de listes
de suivi, de liens et de blocs de code est donc inutilement difficile.

## Périmètre

Formater le Markdown à la lecture dans `htb ticket show` et
`htb ticket comments`, sans changer le contenu enregistré ni les réponses de
l’API.

## Critères d’acceptation

- Les titres, listes, cases à cocher, citations, liens, emphases et blocs de
  code sont lisibles dans un terminal.
- Les contenus restent accessibles lorsque les couleurs sont désactivées.
- Une séquence de contrôle présente dans un ticket ou un commentaire ne peut
  pas modifier le terminal de la personne qui le lit.
- Les sorties structurées existantes ne changent pas.

## Hors périmètre

Édition Markdown, prévisualisation dans un navigateur, nouvelles routes HTTP,
et modification du format stocké.
