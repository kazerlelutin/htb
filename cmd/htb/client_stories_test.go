package main

import (
	"strings"
	"testing"
)

func TestClientStoryCommandsValidateReferencesAndArguments(t *testing.T) {
	if path, err := clientStoryPath("site-12"); err != nil || path != "/api/v1/client-stories/SITE-12" {
		t.Fatalf("story path: %q, %v", path, err)
	}
	for _, ref := range []string{"", "SITE-0", "SITE-12/publication"} {
		if _, err := clientStoryPath(ref); err == nil {
			t.Errorf("accepted invalid reference %q", ref)
		}
	}
	if err := ticketPublication(nil, true); err == nil {
		t.Fatal("publish accepted missing reference")
	}
	if err := ticketClientComment([]string{"SITE-12"}); err == nil {
		t.Fatal("client comment accepted missing text")
	}
	for _, command := range [][]string{{"ticket", "publish"}, {"ticket", "client-comments"}, {"ticket", "client-comment"}} {
		if !strings.Contains(helpText(command), "Usage:") {
			t.Errorf("help missing for %q", command)
		}
	}
}
