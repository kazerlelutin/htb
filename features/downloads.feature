Feature: Downloads page
  Scenario: The public page documents every CLI command
    When a visitor opens "/downloads"
    Then the page is in English
    And it provides installation instructions
    And it explains when to use every command and its important options

  Scenario: The public page avoids irrelevant server implementation details
    When a visitor opens "/downloads"
    Then the page does not display the server version
    And it describes sign-in without naming the identity provider
