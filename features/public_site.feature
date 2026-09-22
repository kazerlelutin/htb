Feature: Site public HTB
  Scenario: Une visite sans choix ne déclenche pas la mesure d’audience
    Given une personne visite une page publique HTB pour la première fois
    When la page est affichée
    Then le bandeau de préférence de mesure est visible
    And le script analytics.ben-to.fr n’est pas chargé

  Scenario: Une personne accepte la mesure d’audience
    Given une personne voit le bandeau de préférence de mesure
    When elle choisit "Accepter"
    Then son choix est mémorisé dans son navigateur
    And le script analytics.ben-to.fr est chargé avec l’identifiant HTB

  Scenario: Une personne refuse la mesure d’audience
    Given une personne voit le bandeau de préférence de mesure
    When elle choisit "Refuser"
    Then le script analytics.ben-to.fr n’est pas chargé
    And elle peut modifier ce choix depuis le pied de page

  Scenario: Le site explique le fonctionnement d’HTB
    Given une personne consulte la page d’accueil
    Then elle voit un exemple de commandes pour créer un projet et un ticket
    And elle comprend qu’HTB est utilisable par les personnes, scripts et agents
    And elle comprend que les invitations, droits de projet et changements sont gérés de façon explicite et traçable
    And elle peut consulter les CGU, les mentions légales et la politique de confidentialité

  Scenario: Le site affiche une identité terminal reconnaissable
    Given une personne consulte une page publique HTB
    Then son navigateur charge le favicon HTB au format SVG
    And le wordmark affiche le curseur terminal clignotant
    And le curseur reste fixe lorsque la personne préfère réduire les animations

  Scenario: Les futures limites de projets restent inactives par défaut
    Given le contrôle HTBD_ENFORCE_PROJECT_LIMITS vaut "false"
    When une personne crée un projet
    Then le serveur ne limite pas la création selon son offre

  Scenario: Les futures limites s’appliquent au propriétaire lorsque le contrôle est actif
    Given le contrôle HTBD_ENFORCE_PROJECT_LIMITS vaut "true"
    And une personne possède déjà le nombre maximal de projets de son offre
    When elle crée un projet supplémentaire
    Then le serveur refuse la création avec le code "project_limit_reached"
