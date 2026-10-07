Feature: ChatGPT MCP access
  Scenario: A connected person reads only their HTB work
    Given a person has linked their HTB account through Zitadel in ChatGPT
    When they ask ChatGPT to list projects, search a project, or read a ticket
    Then ChatGPT can use the HTB read tools
    And HTB returns only projects and tickets that the person may read

  Scenario: A connected person searches tickets across accessible projects
    Given a person has read access to several HTB projects
    When ChatGPT searches tickets without specifying a project
    Then HTB searches each accessible project with the requested filters
    And HTB returns no tickets from inaccessible projects

  Scenario: ChatGPT discovers authenticated HTB actions
    Given a person has linked their HTB account through Zitadel in ChatGPT
    When ChatGPT starts modern MCP discovery and requests the HTB MCP tool list
    Then each HTB action declares that it requires OAuth
    And the compatibility metadata describes the same OAuth requirement

  Scenario: An unlinked person is asked to sign in
    Given a person has added the HTB MCP server without linking an account
    When ChatGPT invokes an HTB MCP tool
    Then HTB returns an OAuth authentication challenge
    And ChatGPT can start the Zitadel authorization flow
    And ChatGPT registers its OAuth callback with Zitadel dynamically

  Scenario: A chat plans work without changing the board
    Given a person is using the HTB MCP connection
    When they ask for a ticket to be created or changed
    Then the MCP instructions explain that the chat may plan and propose work
    And HTB agents execute work through the existing request and triage process

  Scenario: A connected person submits a confirmed ticket proposal
    Given a person has read access to an HTB project
    When they explicitly confirm a ticket proposal prepared by ChatGPT
    Then ChatGPT submits it as a client request for that project
    And HTB does not create or modify an internal ticket directly
