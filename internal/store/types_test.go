package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/domain"
)

func TestCreateProjectRejectsInvalidKeyBeforeDatabaseAccess(t *testing.T) {
	err := (&Store{}).CreateProject(context.Background(), Actor{}, Project{Key: "not-valid", Name: "Example"})
	if err == nil || !strings.Contains(err.Error(), "project key") {
		t.Fatalf("got %v", err)
	}
}

func TestTicketSerializesDistinctRelationshipFields(t *testing.T) {
	parent, related, feature := "SITE-1", "SITE-2", "onboarding"
	b, err := json.Marshal(Ticket{ParentRef: &parent, RelatedRef: &related, FeatureKey: &feature, Title: "Titre", Description: "Description"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"parent_ref", "related_ref", "feature_key"} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("missing %s from %s", key, b)
		}
	}
	for _, key := range []string{"\"title\":\"Titre\"", "\"description\":\"Description\""} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("missing %s from %s", key, b)
		}
	}
}

func TestTicketLabelJSONDecodes(t *testing.T) {
	labels, err := decodeTicketLabels([]byte(`["newsletter","site"]`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(labels, ","), "newsletter,site"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTicketPayloadDecodesSnakeCaseFields(t *testing.T) {
	var create CreateTicket
	if err := json.Unmarshal([]byte(`{"project":"SITE","type":"technical_task","parent_ref":"SITE-1","feature_key":"newsletter"}`), &create); err != nil {
		t.Fatal(err)
	}
	if create.ParentRef != "SITE-1" || create.FeatureKey != "newsletter" {
		t.Fatalf("unexpected creation payload: %#v", create)
	}
	var update UpdateTicket
	if err := json.Unmarshal([]byte(`{"expected_version":3,"status":"in_progress"}`), &update); err != nil {
		t.Fatal(err)
	}
	if update.ExpectedVersion != 3 || update.Status == nil || *update.Status != "in_progress" {
		t.Fatalf("unexpected update payload: %#v", update)
	}
}

type valuesScanner struct{ values []any }

func (s valuesScanner) Scan(destinations ...any) error {
	if len(destinations) != len(s.values) {
		return errors.New("unexpected scan destination count")
	}
	for index, value := range s.values {
		switch destination := destinations[index].(type) {
		case *int64:
			*destination = value.(int64)
		case *int:
			*destination = value.(int)
		case *string:
			*destination = value.(string)
		case *sql.NullString:
			if value != nil {
				destination.String, destination.Valid = value.(string), true
			}
		case *[]byte:
			*destination = []byte(value.(string))
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func TestScanTicketDecodesLabelsAndFullRelations(t *testing.T) {
	ticket, err := scanTicket(valuesScanner{values: []any{
		int64(2), "SITE", string(domain.TechnicalTask), "SITE-1", "SITE-7", "newsletter",
		"Créer l'endpoint", "Description", string(domain.Open), "normal", 1, `["newsletter","site"]`, 0, 0,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Ref != "SITE-2" || ticket.ParentRef == nil || *ticket.ParentRef != "SITE-1" || ticket.RelatedRef == nil || *ticket.RelatedRef != "SITE-7" {
		t.Fatalf("unexpected relations: %#v", ticket)
	}
	if got, want := strings.Join(ticket.Labels, ","), "newsletter,site"; got != want {
		t.Fatalf("got labels %q, want %q", got, want)
	}
}
