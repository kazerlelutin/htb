# Régression du filtre HTTP des tickets

## Problème

Le test HTTP modifie le titre d'un ticket puis cherche l'ancien titre. Il
échoue donc même lorsque le filtre applique correctement tous ses critères.

## Périmètre

- Faire rechercher au test le titre effectivement enregistré après la mise à
  jour.
- Renvoyer une collection JSON vide plutôt que `null` lorsqu'aucun ticket ne
  correspond au filtre.

## Critères d'acceptation

- Les filtres combinés retrouvent le ticket mis à jour.
- Un filtre sans résultat répond `{\"tickets\":[]}`.
