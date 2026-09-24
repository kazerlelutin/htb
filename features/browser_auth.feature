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

  Scenario: The public site names both browser access options
    Given HTB is configured with a Zitadel web application using PKCE
    When a visitor opens the public HTB site
    Then the login link reads "Se connecter / S’inscrire" in French
    And the same link reads "Sign in / Sign up" in English

  Scenario: A signed-in visitor sees their account on the public site
    Given a visitor has a valid browser session with a name
    When they open the public HTB site
    Then the header shows their name and an avatar initial linked to the portal
    And the header offers a sign-out action
    And an expired session shows the sign-in link instead

  Scenario: A signed-in visitor has no display name
    Given a visitor has a valid browser session whose stored name is their technical identifier
    When they open the public HTB site in French
    Then the header links to the portal as "Mon tableau de bord"
    And their technical identifier is not displayed
    And the header offers a sign-out action

  Scenario: Account controls stay beside the language selector
    Given HTB is configured with browser login
    When a visitor opens the public HTB site on desktop or mobile
    Then the sign-in link appears beside the language selector on the right
    And after sign-in the account and sign-out controls take the same place
    And the main navigation remains accessible without horizontal overflow

  Scenario: A self-hosted portal returns to its own HTTPS domain
    Given HTBD_PUBLIC_URL is "https://tickets.example.org"
    And the ZITADEL Web application allows "https://tickets.example.org/auth/callback"
    When a client signs in through ZITADEL
    Then ZITADEL returns the browser to the HTB callback on that domain
    And HTB opens the client portal after validating the login
