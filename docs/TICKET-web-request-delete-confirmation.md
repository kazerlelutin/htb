# Ticket — Confirmation de suppression d’une demande depuis le portail

## Problème

Le bouton de suppression d’une demande du portail client envoyait immédiatement
la suppression. Une erreur de clic pouvait donc retirer définitivement une
demande encore en attente ou rejetée.

## Périmètre

Ajouter une étape de confirmation dans le parcours web des demandes client,
sans modifier les règles d’autorisation ni les autres interfaces.

## Critères d’acceptation

- Le bouton « Supprimer » ouvre une page de confirmation qui identifie la
  demande concernée et indique que l’action est irréversible.
- La demande n’est pas supprimée lors de l’ouverture de cette page ni lorsque
  la personne annule.
- Seule la confirmation explicite, protégée par le jeton anti-CSRF existant
  et un jeton de confirmation signé pour la demande, exécute la suppression.
- Les demandes prises en charge ne peuvent pas accéder à cette confirmation.

## Scénario

Voir `features/client_requests.feature`.
