package main

import (
	"strings"
	"testing"
)

func TestRequestCommandRejectsInvalidIdentifiersAndArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"show", "zero"}, {"show", "0"}, {"comment", "1"}, {"status", "1"}, {"link", "1"}} {
		if err := requestCommand(args); err == nil {
			t.Fatalf("expected usage error for %v", args)
		}
	}
	if !strings.Contains(helpText([]string{"request"}), "request link ID TICKET-REF") {
		t.Fatal("request help is incomplete")
	}
}
