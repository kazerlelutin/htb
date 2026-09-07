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
	if got := archivedLabel(true); got != " (archivé)" {
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
	if got := ticketProgress(ticketView{Type: "user_story", ChildCount: 3, DoneChildren: 2}); got != "2/3 tâches (66%)" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "bug"}); got != "—" {
		t.Fatalf("got %q", got)
	}
}
