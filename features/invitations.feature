Feature: Invitation à un projet
  Scenario: Une invitation attribue les droits du projet
    Given une invitation "write" à usage unique pour le projet "SITE"
    When une identité Zitadel accepte le code
    Then elle devient membre "write" du projet "SITE"
