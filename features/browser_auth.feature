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
    When they request their projects through the client route
    Then HTB returns only their authorized project names
    And the browser session cannot access the internal API
