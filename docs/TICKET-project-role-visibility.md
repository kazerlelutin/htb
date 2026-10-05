# Ticket — Afficher le rôle d’accès au projet

## Problème

Une personne peut accéder à un projet sans savoir si son rôle est `read`,
`write` ou `admin`. Cette ambiguïté rend notamment un refus de publication
difficile à interpréter.

## Solution

Exposer le rôle effectif dans les listes de projets et d’avancement de l’API,
et l’afficher dans la CLI ainsi que dans le portail. Ce changement est
informatif : les règles d’autorisation restent inchangées.

## Critères d’acceptation

- La CLI affiche le rôle pour chaque projet dans `project list` et
  `project status`.
- Le portail affiche le rôle sur l’accueil et sur la page du projet.
- Les rôles restent limités à `read`, `write` et `admin` ; aucune donnée de
  membre supplémentaire n’est exposée.

Scénario : `features/client_stories.feature`.
