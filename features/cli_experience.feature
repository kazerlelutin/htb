Feature: CLI experience
  Scenario: Creating a project gives immediately useful feedback
    Given a connected person without a current project
    When they run "htb project create --key SITE --name Ben-to"
    Then the CLI confirms creation with the project key and name
    And it says that "SITE" is now the current project

  Scenario: A lowercase project key is accepted predictably
    Given a connected person without a current project
    When they run "htb project create --key htb --name HTB"
    Then the CLI creates the project with key "HTB"
    And it says that "HTB" is now the current project

  Scenario: A missing project key is explained before a request is made
    When a person runs "htb project create --name HTB"
    Then the CLI says that a project key is required
    And it explains the project-key format and its use in ticket references

  Scenario: Project-creation help explains project keys
    When a person runs "htb project create --help"
    Then the CLI explains project keys with an "HTB-1" example

  Scenario: Listing tickets is readable in a terminal
    Given a current project containing tickets
    When a person runs "htb ticket list"
    Then the CLI displays references, statuses, types, and titles in a table
