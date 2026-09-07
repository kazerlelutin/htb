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

var ticketRef = regexp.MustCompile(`^([A-Z][A-Z0-9_]{1,19})-([1-9][0-9]*)$`)

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
		return "## Besoin\nEn tant que …\nJe veux …\nAfin de …\n\n## Critères d’acceptation\n- [ ] …\n"
	case TechnicalTask:
		return "## Contexte\n\n## Approche technique\n\n## Définition de fini\n- [ ] …\n"
	case Bug:
		return "## Constat\n\n## Résultat attendu\n\n## Reproduction\n1. …\n\n## Impact\n"
	case Incident:
		return "## Détection\n\n## Systèmes affectés\n\n## Impact\n\n## Mesures de confinement\n\n## Éléments techniques\n"
	default:
		return ""
	}
}
