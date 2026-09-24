# Ticket — Libellés concis et wordmark du portail

## Problème

Le détail d’un projet répète sa clé au-dessus de son nom. Les libellés
« tâches techniques » et « commentaires internes » ajoutent une précision
inutile dans le portail, et le curseur animé du wordmark n’y est pas affiché.

## Solution

Présenter un projet par son nom, raccourcir les libellés affichés en « tâches »
et « commentaires », et réutiliser le curseur isolé du wordmark sur toutes les
pages du portail. Les droits et les séparations entre commentaires internes et
conversation client restent inchangés.

## Critères d’acceptation

- Le détail du projet ne répète pas sa clé au-dessus de son nom.
- Le portail affiche « tâches » et « commentaires » sans les qualificatifs
  « techniques » et « internes ».
- Chaque page du portail affiche le curseur terminal animé du wordmark, avec
  le repli existant pour la réduction des animations.
- Les contrôles d’accès et les flux de commentaires existants restent
  inchangés.

Scénario : `features/client_stories.feature`.
