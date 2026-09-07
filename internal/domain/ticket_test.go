package domain

import "testing"

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
}

func TestTemplateIsSpecificToTicketType(t *testing.T) {
	if got := Template(Incident); got == "" || got == Template(Bug) {
		t.Fatalf("incident template should be present and distinct")
	}
}
