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
	if got := ticketProgress(ticketView{Type: "user_story", ChildCount: 3, DoneChildren: 2}); got != "[██████░░░░] 2/3 tasks (66%)" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "user_story"}); got != "— no tasks" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "bug"}); got != "—" {
		t.Fatalf("got %q", got)
	}
}

func TestProgressBarAndColorsRespectConfiguration(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("HTB_COLOR", "always")
	if got := progressBar(1, 2); got != "[█████░░░░░]" {
		t.Fatalf("uncolored bar = %q", got)
	}
	if got := progressSummary(progressView{Done: 0, Total: 0}, "no tickets"); got != "— no tickets" {
		t.Fatalf("empty progress = %q", got)
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("HTB_COLOR", "always")
	if got := progressBar(1, 2); !strings.Contains(got, ansiSuccess) || !strings.Contains(got, ansiMuted) {
		t.Fatalf("colored bar = %q", got)
	}
	if got := styledStatus("blocked"); !strings.Contains(got, ansiError) {
		t.Fatalf("blocked status = %q", got)
	}
}

func TestProjectStatusViewDecodesAggregate(t *testing.T) {
	var response struct {
		Projects []projectStatusView `json:"projects"`
	}
	if err := json.Unmarshal([]byte(`{"projects":[{"key":"SITE","name":"Site","user_stories":{"total":3,"done":2},"tickets":{"total":5,"done":3},"statuses":{"open":1,"in_progress":1,"review":0,"blocked":0,"done":3}}]}`), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Projects) != 1 || response.Projects[0].Key != "SITE" || response.Projects[0].UserStories.Done != 2 || response.Projects[0].Statuses.InProgress != 1 {
		t.Fatalf("unexpected project status: %#v", response.Projects)
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
		{"project", "list"}, {"project", "status"}, {"project", "use"}, {"project", "create"}, {"project", "members"}, {"project", "member"}, {"feature", "create"},
		{"ticket", "create"}, {"ticket", "list"}, {"ticket", "show"}, {"ticket", "update"}, {"ticket", "comment"}, {"ticket", "claim"}, {"ticket", "release"}, {"ticket", "versions"}, {"ticket", "restore"},
		{"invite", "create"}, {"invite", "accept"}, {"invite", "list"}, {"invite", "revoke"},
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
