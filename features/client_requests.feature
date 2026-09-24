Feature: Espace client de demandes
  Scenario: Une nouvelle personne rejoint un projet sans CLI
    Given une personne possède une identité Zitadel avec email vérifié
    And un administrateur lui a transmis un code d’invitation au projet SITE
    When elle se connecte au portail puis saisit le code
    Then elle accède au projet SITE sans que HTB ne lui demande de mot de passe

  Scenario: Le client propose et suit une demande
    Given un membre read du projet SITE est connecté au portail
    When il propose une demande avec sujet et description Markdown
    Then la demande est enregistrée avec l’état « reçue »
    And il peut la retrouver dans la liste du projet et voir sa conversation
    And il peut y ajouter un commentaire

  Scenario: L’équipe qualifie la demande sans exposer le ticket interne
    Given une demande client du projet SITE
    When un membre write la passe en cours et la rattache à SITE-12
    Then le lien est visible dans la CLI de l’équipe
    And le portail client n’affiche ni la priorité, ni le responsable, ni le ticket interne

  Scenario: Le projet et les formulaires restent protégés
    Given un membre du projet SITE sans accès au projet SECRET
    When il ouvre une demande du projet SECRET
    Then HTB ne révèle pas cette demande
    When il envoie un formulaire sans jeton anti-CSRF valide
    Then HTB refuse l’action
