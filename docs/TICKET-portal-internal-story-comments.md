# Ticket — Commentaires internes depuis le détail d’une US

## Problème

Un administrateur qui prévisualise une US brouillon dans le portail ne peut pas
ajouter de commentaire au ticket depuis sa page de détail.

## Solution

Afficher la discussion interne et son formulaire sur `/portal/stories/{ref}`
pour les administrateurs. Réutiliser les commentaires de ticket existants et
les garder séparés de la conversation visible par les clients.

## Critères d’acceptation

- Un administrateur peut commenter une US, même non publiée, depuis son détail
  dans le portail.
- La soumission nécessite le jeton CSRF de la session.
- Les membres non administrateurs ne peuvent ni lire ni ajouter ces
  commentaires internes.
- La conversation client des US publiées reste distincte.

Scénario : `features/client_stories.feature`.
