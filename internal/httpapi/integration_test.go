package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/domain"
	"github.com/kazerlelutin/htb/internal/store"
)

type integrationVerifier struct{ subject string }

func (v integrationVerifier) Verify(context.Context, string) (auth.Principal, error) {
	return auth.Principal{Subject: v.subject, Name: "Integration test"}, nil
}

// TestTicketLifecycleOverHTTP exercises the exact path used by the CLI. It is
// intentionally opt-in because it needs a disposable PostgreSQL database.
func TestTicketLifecycleOverHTTP(t *testing.T) {
	dsn := os.Getenv("HTBD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HTBD_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	data := &store.Store{DB: db}
	if err := data.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	key := "E2E" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	subject := "http-test-" + key
	defer cleanupIntegrationData(t, db, key, subject)

	server := New(data, integrationVerifier{subject: subject}, auth.DeviceConfig{}, "", slog.Default())
	project := requestJSON(t, server, http.MethodPost, "/api/v1/projects", `{"key":"`+strings.ToLower(key)+`","name":"HTTP integration"}`)
	if project.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", project.Code, project.Body.String())
	}
	if !strings.Contains(project.Body.String(), `"key":"`+key+`"`) {
		t.Fatalf("create project should normalize its key: %s", project.Body.String())
	}
	feature := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+key+"/features", `{"key":"newsletter","name":"Newsletter","due_date":"2026-09-30T00:00:00Z"}`)
	if feature.Code != http.StatusCreated {
		t.Fatalf("create feature: %d %s", feature.Code, feature.Body.String())
	}

	created := requestJSON(t, server, http.MethodPost, "/api/v1/tickets", `{"project":"`+key+`","type":"user_story","title":"Ticket avec tags","priority":"high","labels":["site","newsletter"]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create ticket: %d %s", created.Code, created.Body.String())
	}
	var ticket store.Ticket
	if err := json.Unmarshal(created.Body.Bytes(), &ticket); err != nil {
		t.Fatalf("create response must be valid JSON: %v; body=%s", err, created.Body.String())
	}
	if got, want := strings.Join(ticket.Labels, ","), "newsletter,site"; got != want {
		t.Fatalf("create labels: got %q, want %q", got, want)
	}

	shown := requestJSON(t, server, http.MethodGet, "/api/v1/tickets/"+ticket.Ref, "")
	if shown.Code != http.StatusOK {
		t.Fatalf("show ticket: %d %s", shown.Code, shown.Body.String())
	}
	if err := json.Unmarshal(shown.Body.Bytes(), &ticket); err != nil {
		t.Fatalf("show response must contain one JSON document: %v; body=%s", err, shown.Body.String())
	}
	if ticket.Type != domain.UserStory || ticket.Ref == "" {
		t.Fatalf("unexpected ticket: %#v", ticket)
	}

	updated := requestJSON(t, server, http.MethodPatch, "/api/v1/tickets/"+ticket.Ref, `{"expected_version":1,"title":"Titre mis à jour","feature_key":"newsletter"}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update ticket: %d %s", updated.Code, updated.Body.String())
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &ticket); err != nil {
		t.Fatalf("update response must be valid JSON: %v; body=%s", err, updated.Body.String())
	}
	if ticket.Title != "Titre mis à jour" || ticket.Version != 2 || ticket.FeatureKey == nil || *ticket.FeatureKey != "newsletter" {
		t.Fatalf("unexpected update: %#v", ticket)
	}
	linked := requestJSON(t, server, http.MethodGet, "/api/v1/tickets?project="+key+"&feature=newsletter", "")
	if linked.Code != http.StatusOK || !strings.Contains(linked.Body.String(), ticket.Ref) {
		t.Fatalf("list linked tickets: %d %s", linked.Code, linked.Body.String())
	}
}

func requestJSON(t *testing.T, server *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer integration-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func cleanupIntegrationData(t *testing.T, db *sql.DB, key, subject string) {
	t.Helper()
	queries := []string{
		`DELETE FROM activity_events WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM ticket_revisions WHERE ticket_id IN (SELECT id FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1))`,
		`DELETE FROM comments WHERE ticket_id IN (SELECT id FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1))`,
		`DELETE FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM labels WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM features WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM project_memberships WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM projects WHERE key=$1`,
		`DELETE FROM credentials WHERE zitadel_subject=$1`,
		`DELETE FROM users WHERE zitadel_subject=$1`,
	}
	for index, query := range queries {
		argument := any(key)
		if index >= 8 {
			argument = subject
		}
		if _, err := db.Exec(query, argument); err != nil {
			t.Errorf("cleanup integration data: %v", err)
		}
	}
}
