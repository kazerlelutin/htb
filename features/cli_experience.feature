Feature: Sorties de la CLI
  Scenario: Créer un projet donne un retour immédiatement exploitable
    Given une personne connectée sans projet courant
    When elle exécute "htb project create --key SITE --name Ben-to"
    Then la CLI confirme la création avec la clé et le nom du projet
    And elle indique que "SITE" est devenu le projet courant

  Scenario: Lister les tickets est lisible dans un terminal
    Given un projet courant contenant des tickets
    When une personne exécute "htb ticket list"
    Then la CLI affiche les références, états, types et titres dans un tableau
