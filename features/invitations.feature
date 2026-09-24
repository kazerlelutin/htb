Feature: Invitation à un projet
  Scenario: Une invitation utilise une expiration par défaut
    Given un administrateur crée une invitation sans date d’expiration
    When HTB enregistre l’invitation
    Then l’invitation expire sept jours après sa création

  Scenario: Une invitation attribue les droits du projet
    Given une invitation "write" à usage unique pour le projet "SITE"
    When une identité Zitadel accepte le code
    Then elle devient membre "write" du projet "SITE"

  Scenario: Un administrateur gère les accès du projet
    Given un projet "SITE" avec son propriétaire, un membre et une invitation active
    When l’administrateur consulte les membres et invitations du projet
    Then il voit les identifiants nécessaires pour modifier les accès
    And les codes d’invitation secrets ne sont pas affichés
    When il modifie le rôle du membre, le retire, ou révoque l’invitation
    Then les accès concernés ne sont plus disponibles

  Scenario: Le propriétaire reste protégé
    Given un administrateur consulte les membres du projet "SITE"
    When il essaie de retirer ou rétrograder le propriétaire
    Then HTB refuse l’opération
