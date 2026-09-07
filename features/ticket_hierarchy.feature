Feature: Découpage d'une user story
  Scenario: Le statut d'une US reflète ses tâches techniques
    Given une US "SITE-12" avec deux tâches techniques ouvertes
    When une tâche est en cours et l'autre est terminée
    Then le statut de l'US est "in_progress"

  Scenario: Une tâche ne peut pas appartenir à un ticket non-US
    Given un bug "SITE-13"
    When un rédacteur crée une tâche technique sous "SITE-13"
    Then la création est refusée

  Scenario: Une user story peut exister sans fonctionnalité produit
    Given un projet sans fonctionnalité définie
    When un rédacteur crée une user story
    Then la création est acceptée
