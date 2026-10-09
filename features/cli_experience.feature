Feature: CLI experience
  Scenario: The hosted server is ready without configuration
    Given a person has installed the CLI without a local configuration file
    When they run "htb auth login"
    Then the CLI requests its connection settings from "https://htboard.xyz"
    And they only need "htb config set-server URL" to use another server

  Scenario: A Windows user can install the CLI from PowerShell
    Given a release contains the Windows x86_64 CLI archive and checksums
    When they run the PowerShell installer
    Then it verifies the archive before installing htb.exe for the current user
    And htb is available in the current and future PowerShell sessions

  Scenario: A person updates the CLI on their operating system
    Given a newer CLI release for their operating system and architecture
    When they run "htb update"
    Then the CLI downloads the matching archive and verifies its SHA-256 checksum
    And it replaces the installed CLI, after the command exits on Windows
    And it displays the latest release summary and a link to the complete release notes

  Scenario: An unsupported operating system is explained before downloading
    Given the current operating system and architecture have no HTB release
    When a person runs "htb update"
    Then the CLI explains that automatic updates are unavailable for that platform

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

  Scenario: A person searches all accessible projects from the CLI
    Given a person can read several projects containing tickets
    When they run "htb ticket list --all-projects --query checkout"
    Then the CLI displays only matching tickets from projects they can access
    And the result identifies the project in every ticket reference

  Scenario: A person searches one explicit project from the CLI
    Given a person can read several projects containing tickets
    When they run "htb ticket list --project SITE --query checkout"
    Then the CLI searches only the "SITE" project
    And it rejects using "--project" and "--all-projects" together

  Scenario: JSON ticket listings report client publication
    Given a project contains a published user story and a private ticket
    When they run "htb ticket list --json"
    Then every ticket includes a "published" boolean
    And the published user story has "published" set to true

  Scenario: Reading a ticket keeps its conversation and changes visible
    Given a ticket with comments and traceable changes
    When a person runs "htb ticket comments SITE-1" or "htb ticket activity SITE-1"
    Then the CLI displays the author and timestamp of each item

  Scenario: Reading ticket content formats Markdown safely
    Given a ticket description or comment containing headings, lists, links, and code
    When a person runs "htb ticket show SITE-1" or "htb ticket comments SITE-1"
    Then the CLI displays the Markdown as readable terminal text
    And control sequences supplied in the content cannot affect the terminal

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
