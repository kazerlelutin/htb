Feature: User stories visible aux clients
  Scenario: Une US reste privée jusqu’à sa publication
    Given une US et ses tâches techniques existent dans un projet
    When une personne du projet ouvre le portail client
    Then elle ne voit pas cette US
    When un administrateur publie l’US avec la CLI
    Then la personne voit l’US, son titre et sa description
    And elle ne voit ni les titres des tâches techniques ni les commentaires internes

  Scenario: L’avancement d’une US vient de ses tâches techniques
    Given une US publiée possède trois tâches techniques dont deux terminées
    When une personne ouvre l’US dans le portail
    Then elle voit « 2 tâches terminées sur 3 » et une barre de progression à 66 %
    And le tableau de bord du projet agrège la progression des US publiées

  Scenario: Une US sans tâche n’annonce pas un faux pourcentage
    Given une US publiée n’a aucune tâche technique liée
    When une personne ouvre l’US dans le portail
    Then elle lit que l’US n’a pas encore de tâche liée

  Scenario: La conversation client reste distincte des notes internes
    Given une US publiée possède un commentaire interne dans la CLI
    When un client commente cette US depuis le portail
    Then sa réponse apparaît dans la conversation client
    And le commentaire interne n’apparaît pas dans le portail
    And l’équipe peut lire et répondre à la conversation client depuis la CLI

  Scenario: Une demande peut mener à une US publiée
    Given une demande client est liée à une US publiée
    When le client ouvre sa demande
    Then il peut rejoindre l’US sans voir les tickets techniques

  Scenario: Une US masquée n’est plus consultable
    Given une US était publiée dans le portail
    When un administrateur la masque avec la CLI
    Then le portail ne montre plus l’US et refuse les nouveaux commentaires
