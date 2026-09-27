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
