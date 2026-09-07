Feature: Autorisation Zitadel
  Scenario: La CLI ouvre la connexion Zitadel sans demander de paramètres IAM
    Given un serveur HTB configuré avec son client Device Code Zitadel
    When une personne exécute "htb auth login"
    Then la CLI ouvre la page de connexion Zitadel
    And elle affiche l'onboarding de projet après la connexion

  Scenario: Le superadmin est reconnu depuis le claim Zitadel standard
    Given un access token HTB avec le rôle applicatif "superadmin"
    When son claim de rôles Zitadel est au format standard imbriqué
    Then HTB autorise les opérations globales

  Scenario: Une personne invitée reste autorisable sans rôle Zitadel
    Given une invitation HTB valide
    And une identité Zitadel sans rôle applicatif
    When cette identité accepte le code d'invitation dans la CLI
    Then HTB lui attribue le rôle du projet invité
