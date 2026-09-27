Feature: Exploitation sûre du service
  Scenario: Une erreur PostgreSQL inattendue ne fuit pas dans l’API
    Given a storage operation fails with an unexpected database error
    When an API client performs the operation
    Then the response is a generic server error without database details

  Scenario: Le proxy limite les rafales par adresse IP
    Given the HTB reverse proxy applies its request limit
    When one address sends more requests than the configured burst
    Then excess requests receive HTTP 429
    And a normal request succeeds again after the limit recovers
