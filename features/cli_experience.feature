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

  Scenario: A person narrows daily work with combined filters
    Given a current project containing tickets with distinct statuses, priorities, labels, and descriptions
    When they run "htb ticket list --status blocked --priority urgent --label production --query checkout"
    Then the CLI displays only tickets matching every filter
    And the same filters can be used with JSON and CSV output

  Scenario: Reading a ticket keeps its conversation and changes visible
    Given a ticket with comments and traceable changes
    When a person runs "htb ticket comments SITE-1" or "htb ticket activity SITE-1"
    Then the CLI displays the author and timestamp of each item

  Scenario: A user story shows progress as a bar
    Given a user story with two completed technical tasks out of three
    When a person runs "htb ticket list"
    Then the CLI displays a progress bar and "2/3 tasks (66%)" for that user story

  Scenario: Project status gives a portfolio view
    Given a person can access projects with tickets in several statuses
    When they run "htb project status"
    Then the CLI displays a user-story bar, a ticket bar, and each status count for every accessible project

  Scenario: Console colors can be disabled
    Given a terminal with "NO_COLOR=1"
    When a person runs "htb project status"
    Then the CLI does not emit ANSI color sequences

  Scenario: Ticket references start at one in every project
    Given an existing ticket in project "HTB"
    When a person creates the first ticket in project "SITE"
    Then its reference is "SITE-1"
