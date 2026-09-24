package store

import (
	"strings"
	"testing"
)

func TestClientStorySelectDoesNotEndColumnsWithComma(t *testing.T) {
	if strings.Contains(clientStorySelect, ",\n\tFROM") {
		t.Fatalf("client story SELECT has a trailing comma before FROM: %s", clientStorySelect)
	}
}
