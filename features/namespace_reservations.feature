Feature: Réservation des namespaces
  Les namespaces évitent qu'un autre compte utilise le préfixe d'un projet.

  Scenario: Revendiquer un namespace avant de créer un projet
    Given an authenticated user with no reserved namespace
    When the user claims "ALICE"
    And creates the project "ALICE/SITE"
    Then the project is created

  Scenario: Refuser un namespace appartenant à un autre compte
    Given the namespace "ALICE" is reserved by another account
    When the authenticated user creates "ALICE/SITE"
    Then the project creation is forbidden

  Scenario: Limiter les namespaces selon l'offre
    Given an account whose plan permits one namespace
    And the account has reserved "ALICE"
    When the account claims "BOB"
    Then the claim is refused because the namespace limit is reached
