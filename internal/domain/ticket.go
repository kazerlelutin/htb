package domain

import (
	"fmt"
	"regexp"
	"strings"
)

type Role string

const (
	RoleRead  Role = "read"
	RoleWrite Role = "write"
	RoleAdmin Role = "admin"
)

func (r Role) Allows(required Role) bool {
	return map[Role]int{RoleRead: 1, RoleWrite: 2, RoleAdmin: 3}[r] >= map[Role]int{RoleRead: 1, RoleWrite: 2, RoleAdmin: 3}[required]
}

type TicketType string

const (
	UserStory     TicketType = "user_story"
	TechnicalTask TicketType = "technical_task"
	Bug           TicketType = "bug"
	Incident      TicketType = "incident"
)

type Status string

const (
	Open       Status = "open"
	InProgress Status = "in_progress"
	Review     Status = "review"
	Blocked    Status = "blocked"
	Done       Status = "done"
)

var (
	projectKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,19}$`)
	ticketRef  = regexp.MustCompile(`^([A-Z][A-Z0-9_]{1,19})-([1-9][0-9]*)$`)
)

// NormalizeProjectKey makes project identifiers consistent across the CLI and API.
func NormalizeProjectKey(value string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	if key == "" {
		return "", fmt.Errorf("project key is required")
	}
	if !projectKey.MatchString(key) {
		return "", fmt.Errorf("project key must be 2 to 20 characters, start with a letter, and contain only letters, digits, or underscores")
	}
	return key, nil
}

func ParseReference(ref string) (string, int64, error) {
	m := ticketRef.FindStringSubmatch(strings.ToUpper(ref))
	if m == nil {
		return "", 0, fmt.Errorf("invalid ticket reference %q", ref)
	}
	var id int64
	_, err := fmt.Sscan(m[2], &id)
	return m[1], id, err
}

// AggregateStoryStatus makes a parent story deterministic from its technical children.
func AggregateStoryStatus(children []Status) Status {
	if len(children) == 0 {
		return Open
	}
	all := func(want Status) bool {
		for _, status := range children {
			if status != want {
				return false
			}
		}
		return true
	}
	if all(Done) {
		return Done
	}
	for _, status := range children {
		if status == Blocked {
			return Blocked
		}
	}
	if all(Review) {
		return Review
	}
	if all(Open) {
		return Open
	}
	return InProgress
}

func Template(kind TicketType) string {
	switch kind {
	case UserStory:
		return "## Need\nAs a …\nI want …\nSo that …\n\n## Acceptance criteria\n- [ ] …\n"
	case TechnicalTask:
		return "## Context\n\n## Technical approach\n\n## Definition of done\n- [ ] …\n"
	case Bug:
		return "## Observed behavior\n\n## Expected behavior\n\n## Reproduction\n1. …\n\n## Impact\n"
	case Incident:
		return "## Detection\n\n## Affected systems\n\n## Impact\n\n## Containment actions\n\n## Technical details\n"
	default:
		return ""
	}
}
