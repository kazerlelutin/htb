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
	projectKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,19}(/[A-Z][A-Z0-9_]{1,19})?$`)
	ticketRef  = regexp.MustCompile(`^([A-Z][A-Z0-9_]{1,19}(/[A-Z][A-Z0-9_]{1,19})?)-([1-9][0-9]*)$`)
)

// NormalizeProjectKey makes project identifiers consistent across the CLI and API.
func NormalizeProjectKey(value string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	if key == "" {
		return "", fmt.Errorf("project key is required")
	}
	if !projectKey.MatchString(key) {
		return "", fmt.Errorf("project key must be 2 to 20 characters, start with a letter, and contain only letters, digits, or underscores; optionally followed by / and another key")
	}
	if len(key) > 40 {
		return "", fmt.Errorf("project key cannot exceed 40 characters")
	}
	return key, nil
}

func ParseReference(ref string) (string, int64, error) {
	m := ticketRef.FindStringSubmatch(strings.ToUpper(ref))
	if m == nil {
		return "", 0, fmt.Errorf("invalid ticket reference %q", ref)
	}
	var id int64
	_, err := fmt.Sscan(m[3], &id)
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

// SplitProjectKey splits a project key into namespace and short key.
// If the key contains a slash, the part before slash is the namespace,
// the part after slash is the short key.
// If there is no slash, namespace is empty and shortKey is the entire key.
func SplitProjectKey(key string) (namespace, shortKey string) {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return "", key
}

// ShortKey returns the part of the project key after the slash, or the whole key if there is no slash.
func ShortKey(key string) string {
	_, short := SplitProjectKey(key)
	return short
}

// SuggestNamespace converts a user name to a valid namespace slug.
// It keeps only letters, digits, and underscores, converts to uppercase,
// and limits length to 20 characters.
func SuggestNamespace(name string) string {
	var result []rune
	for _, r := range strings.ToUpper(strings.TrimSpace(name)) {
		if len(result) >= 20 {
			break
		}
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result = append(result, r)
		} else if r == ' ' || r == '-' || r == '@' || r == '.' || r == '+' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "USER"
	}
	if result[0] < 'A' || result[0] > 'Z' {
		result = append([]rune("USER_"), result...)
		if len(result) > 20 {
			result = result[:20]
		}
	}
	return string(result)
}
