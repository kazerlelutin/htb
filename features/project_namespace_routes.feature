Feature: Administration des projets namespacés
  Les clés de projet peuvent porter un namespace et doivent rester utilisables
  dans toutes les routes d'administration.

  Scenario: Renommer un projet déjà namespacé
    Given a project "ALICE/SITE" administered by the authenticated user
    When the user renames it to "BOB/SITE"
    Then the rename succeeds
    And the user can list the members of "BOB/SITE"

  Scenario: Suggérer un namespace à partir d'un nom qui commence par un chiffre
    Given an authenticated user named "123 Alice"
    When the user creates a project without a namespace
    Then the suggested namespace is a valid project-key prefix

  Scenario: Filtrer un namespace sans respecter la casse
    Given an accessible project in the "ALICE" namespace
    When the user runs project status with namespace "alice"
    Then the project is included in the status output
