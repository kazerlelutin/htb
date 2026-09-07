package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeviceScopesRequestHTBAudienceAndRoles(t *testing.T) {
	scopes := deviceScopes("123456")
	for _, expected := range []string{
		"urn:zitadel:iam:org:project:id:123456:aud",
		"urn:zitadel:iam:org:projects:roles",
	} {
		if !strings.Contains(scopes, expected) {
			t.Fatalf("missing %q in %q", expected, scopes)
		}
	}
}

func TestArchivedLabel(t *testing.T) {
	if got := archivedLabel(true); got != " (archived)" {
		t.Fatalf("got %q", got)
	}
	if got := archivedLabel(false); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestHelpRecognizesShortAndLongFlags(t *testing.T) {
	if !isHelpFlag("-h") || !isHelpFlag("--help") || isHelpFlag("help") {
		t.Fatal("help flags are not recognized correctly")
	}
}

func TestTicketViewDecodesRelationshipFields(t *testing.T) {
	var ticket ticketView
	if err := json.Unmarshal([]byte(`{"ref":"SITE-4","parent_ref":"SITE-3","feature_key":"newsletter"}`), &ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.ParentRef == nil || *ticket.ParentRef != "SITE-3" || ticket.FeatureKey == nil || *ticket.FeatureKey != "newsletter" {
		t.Fatalf("unexpected ticket view: %#v", ticket)
	}
}

func TestTicketProgress(t *testing.T) {
	if got := ticketProgress(ticketView{Type: "user_story", ChildCount: 3, DoneChildren: 2}); got != "2/3 tasks (66%)" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "bug"}); got != "—" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectCreationValidationExplainsAndNormalizesKey(t *testing.T) {
	key, err := validateProjectCreation(" htb ", "HTB")
	if err != nil || key != "HTB" {
		t.Fatalf("got key=%q err=%v", key, err)
	}
	for _, input := range []string{"", "htb-key"} {
		if _, err := validateProjectCreation(input, "HTB"); err == nil || !strings.Contains(err.Error(), "project key") || !strings.Contains(err.Error(), "HTB-1") {
			t.Fatalf("input %q returned %v", input, err)
		}
	}
	if _, err := validateProjectCreation("HTB", " "); err == nil || err.Error() != "project name is required" {
		t.Fatalf("unexpected name validation error: %v", err)
	}
}

func TestHelpIsDetailedForEveryCommand(t *testing.T) {
	commands := [][]string{
		{"version"}, {"config", "set-server"}, {"auth", "login"}, {"auth", "status"},
		{"project", "list"}, {"project", "use"}, {"project", "create"}, {"feature", "create"},
		{"ticket", "create"}, {"ticket", "list"}, {"ticket", "show"}, {"ticket", "update"}, {"ticket", "comment"}, {"ticket", "claim"}, {"ticket", "release"}, {"ticket", "versions"}, {"ticket", "restore"},
		{"invite", "create"}, {"invite", "accept"},
	}
	for _, command := range commands {
		help := helpText(command)
		if !strings.Contains(help, "Usage:") || strings.Contains(help, "Help is unavailable") {
			t.Fatalf("command %q has incomplete help: %s", command, help)
		}
	}
	if help := helpText([]string{"project", "create"}); !strings.Contains(help, "normalized to uppercase") || !strings.Contains(help, "HTB-1") {
		t.Fatalf("project help lacks key explanation: %s", help)
	}
}

func TestAPIErrorUsesStructuredMessage(t *testing.T) {
	err := apiError("422 Unprocessable Entity", []byte(`{"error":{"message":"project key is required"}}`))
	if err.Error() != "project key is required" {
		t.Fatalf("got %q", err)
	}
	if err := apiError("500 Internal Server Error", []byte("not json")); err.Error() != "request failed: 500 Internal Server Error" {
		t.Fatalf("got %q", err)
	}
}
