Feature: ChatGPT MCP access
  Scenario: A connected person reads only their HTB work
    Given a person has linked their HTB account through Zitadel in ChatGPT
    When they ask ChatGPT to list projects, search a project, or read a ticket
    Then ChatGPT can use only the read-only HTB MCP tools
    And HTB returns only projects and tickets that the person may read

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
