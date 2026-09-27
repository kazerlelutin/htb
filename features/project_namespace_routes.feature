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

  Scenario: Consulter un projet namespacé dans le portail client
    Given a client with access to the project "MO5/PROMEAI"
    When the client opens "/portal/projects/MO5/PROMEAI"
    Then the project dashboard is displayed
    And the project is grouped under the "MO5" namespace on the portal home

  Scenario: Mettre à jour une US namespacée avec la CLI
    Given an authenticated user can administer the project "KAZERLELUTIN/HTB"
    And the user story "KAZERLELUTIN/HTB-1" is open
    When the user runs "htb ticket update --version 1 --status done KAZERLELUTIN/HTB-1"
    Then the user story status is "done"

  Scenario: Archiver puis restaurer un projet namespacé
    Given the authenticated user owns the project "ALICE/SITE"
    When the user archives and then restores "ALICE/SITE"
    Then its existing data and memberships remain available
