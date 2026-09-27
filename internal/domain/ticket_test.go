package domain

import "testing"

func TestNormalizeProjectKey(t *testing.T) {
	key, err := NormalizeProjectKey(" htb_2 ")
	if err != nil || key != "HTB_2" {
		t.Fatalf("got key=%q err=%v", key, err)
	}
	// Test with namespace
	key, err = NormalizeProjectKey("alice/site")
	if err != nil || key != "ALICE/SITE" {
		t.Fatalf("got key=%q err=%v", key, err)
	}
	for _, value := range []string{"", "A", "HTB-key", "1HTB", "THIS_PROJECT_KEY_IS_TOO_LONG", "ALICE/SITE/EXTRA", "ALICE/", "/SITE", "ALICE/site-extra"} {
		if _, err := NormalizeProjectKey(value); err == nil {
			t.Fatalf("%q should be invalid", value)
		}
	}
}

func TestAggregateStoryStatus(t *testing.T) {
	cases := []struct {
		name     string
		children []Status
		want     Status
	}{
		{"empty", nil, Open}, {"done", []Status{Done, Done}, Done},
		{"blocked wins", []Status{Done, Blocked}, Blocked},
		{"review", []Status{Review, Review}, Review}, {"open", []Status{Open, Open}, Open},
		{"mixed", []Status{Open, Review}, InProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AggregateStoryStatus(tc.children); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseReference(t *testing.T) {
	project, id, err := ParseReference("site-42")
	if err != nil || project != "SITE" || id != 42 {
		t.Fatalf("unexpected parse: %q %d %v", project, id, err)
	}
	project, id, err = ParseReference("alice/site-42")
	if err != nil || project != "ALICE/SITE" || id != 42 {
		t.Fatalf("unexpected parse with namespace: %q %d %v", project, id, err)
	}
}

func TestTemplateIsSpecificToTicketType(t *testing.T) {
	if got := Template(Incident); got == "" || got == Template(Bug) {
		t.Fatalf("incident template should be present and distinct")
	}
}

func TestSplitProjectKey(t *testing.T) {
	ns, short := SplitProjectKey("SITE")
	if ns != "" || short != "SITE" {
		t.Fatalf("no namespace: got ns=%q short=%q", ns, short)
	}
	ns, short = SplitProjectKey("ALICE/SITE")
	if ns != "ALICE" || short != "SITE" {
		t.Fatalf("with namespace: got ns=%q short=%q", ns, short)
	}
	ns, short = SplitProjectKey("ALICE/SITE/EXTRA")
	if ns != "ALICE" || short != "SITE/EXTRA" {
		t.Fatalf("multiple slashes: got ns=%q short=%q", ns, short)
	}
}

func TestSuggestNamespace(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"Alice", "ALICE"},
		{"Alice Smith", "ALICE_SMITH"},
		{"alice-smith", "ALICE_SMITH"},
		{"alice_smith", "ALICE_SMITH"},
		{"Alice123", "ALICE123"},
		{"Alice@Example", "ALICE_EXAMPLE"},
		{"123 Alice", "USER_123_ALICE"},
		{"@Alice", "USER__ALICE"},
		{"12345678901234567890", "USER_123456789012345"},
		{"", "USER"},
		{"A very long name that exceeds twenty characters", "A_VERY_LONG_NAME_THA"},
	}
	for _, tc := range cases {
		got := SuggestNamespace(tc.input)
		if got != tc.want {
			t.Errorf("SuggestNamespace(%q) = %q, want %q", tc.input, got, tc.want)
		}
		if _, err := NormalizeProjectKey(got); err != nil {
			t.Errorf("SuggestNamespace(%q) returned invalid namespace %q: %v", tc.input, got, err)
		}
	}
}
