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

  Scenario: Le créateur archive puis restaure son ticket
    Given un ticket créé par un rédacteur
    When il archive puis restaure ce ticket
    Then le ticket disparaît puis réapparaît dans le travail actif

  Scenario: Seul le créateur ou un administrateur gère le cycle de vie
    Given un ticket créé par un autre rédacteur
    When un simple lecteur tente de l'archiver ou de le supprimer
    Then ces opérations sont refusées

  Scenario: La suppression est protégée quand le ticket est encore lié
    Given un ticket référencé par une tâche, un lien ou une demande client
    When son créateur demande sa suppression
    Then la suppression est refusée
