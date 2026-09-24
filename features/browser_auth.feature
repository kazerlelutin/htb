Feature: Client browser authentication
  Scenario: A client starts a browser sign-in through Zitadel
    Given HTB is configured with a Zitadel web application using PKCE
    When a client opens the sign-in URL
    Then HTB redirects them to Zitadel without exposing an access token
    And the callback can only complete with its signed, short-lived state

  Scenario: A client session is protected and revocable
    Given a client has completed the Zitadel sign-in flow
    When HTB creates their browser session
    Then the browser receives only an opaque secure cookie valid for 30 days
    And logging out revokes the stored session

  Scenario: Browser access preserves project authorization
    Given a client has a browser session and membership in one project
    When they open the client space
    Then HTB shows only their authorized project names
    And the browser session cannot access the internal API

  Scenario: An unconfigured Web client does not expose browser login
    Given the ZITADEL Web client ID is left empty in the example configuration
    When a visitor opens the public HTB site
    Then the client portal entry point is not displayed

  Scenario: A self-hosted portal returns to its own HTTPS domain
    Given HTBD_PUBLIC_URL is "https://tickets.example.org"
    And the ZITADEL Web application allows "https://tickets.example.org/auth/callback"
    When a client signs in through ZITADEL
    Then ZITADEL returns the browser to the HTB callback on that domain
    And HTB opens the client portal after validating the login
