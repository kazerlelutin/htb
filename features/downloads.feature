Feature: Download and command pages
  Scenario: The download page focuses on installation
    When a visitor opens "/downloads"
    Then the page is in English
    And it provides installation instructions
    And it links to the command guide

  Scenario: The command page documents every CLI command
    When a visitor opens "/commands"
    Then the page is in English
    And it explains when to use every command and its important options
    And each command syntax is displayed in its own readable entry
    And a visitor can copy a complete command

  Scenario: The public page avoids irrelevant server implementation details
    When a visitor opens "/commands"
    Then the page does not display the server version
    And it describes sign-in without naming the identity provider
