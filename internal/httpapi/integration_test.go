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
	"strconv"
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
	otherKey := "ALT" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	limitKey := "LIM" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	blockedKey := "NO" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	subject := "http-test-" + key
	memberSubject := "http-member-" + key
	defer cleanupIntegrationData(t, db, []string{key, otherKey, limitKey, blockedKey}, []string{subject, memberSubject})

	server := New(data, integrationVerifier{subject: subject}, auth.DeviceConfig{}, "", slog.Default())
	project := requestJSON(t, server, http.MethodPost, "/api/v1/projects", `{"key":"`+strings.ToLower(key)+`","name":"HTTP integration"}`)
	if project.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", project.Code, project.Body.String())
	}
	if !strings.Contains(project.Body.String(), `"key":"`+key+`"`) {
		t.Fatalf("create project should normalize its key: %s", project.Body.String())
	}
	members := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+key+"/members", "")
	if members.Code != http.StatusOK || !strings.Contains(members.Body.String(), `"owner":true`) {
		t.Fatalf("list project members: %d %s", members.Code, members.Body.String())
	}
	var roster struct {
		Members []store.ProjectMember `json:"members"`
	}
	if err := json.Unmarshal(members.Body.Bytes(), &roster); err != nil || len(roster.Members) != 1 || !roster.Members[0].Owner {
		t.Fatalf("decode owner roster: %#v, %v", roster, err)
	}
	if response := requestJSON(t, server, http.MethodPatch, "/api/v1/projects/"+key+"/members/"+strconv.FormatInt(roster.Members[0].UserID, 10), `{"role":"write"}`); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("demote owner: %d %s", response.Code, response.Body.String())
	}
	memberInvitation := requestJSON(t, server, http.MethodPost, "/api/v1/invitations", `{"project":"`+key+`","role":"read","expires_at":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	if memberInvitation.Code != http.StatusCreated {
		t.Fatalf("create member invitation: %d %s", memberInvitation.Code, memberInvitation.Body.String())
	}
	var invitationCode struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(memberInvitation.Body.Bytes(), &invitationCode); err != nil || invitationCode.Code == "" {
		t.Fatalf("decode member invitation: %v %s", err, memberInvitation.Body.String())
	}
	if err := data.AcceptInvitation(ctx, memberSubject, "HTTP member", "member@example.test", invitationCode.Code); err != nil {
		t.Fatalf("accept member invitation: %v", err)
	}
	members = requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+key+"/members", "")
	if err := json.Unmarshal(members.Body.Bytes(), &roster); err != nil || len(roster.Members) != 2 {
		t.Fatalf("list accepted member: %v %s", err, members.Body.String())
	}
	memberID := roster.Members[1].UserID
	if response := requestJSON(t, server, http.MethodPatch, "/api/v1/projects/"+key+"/members/"+strconv.FormatInt(memberID, 10), `{"role":"write"}`); response.Code != http.StatusNoContent {
		t.Fatalf("update member role: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodDelete, "/api/v1/projects/"+key+"/members/"+strconv.FormatInt(memberID, 10), ""); response.Code != http.StatusNoContent {
		t.Fatalf("remove member: %d %s", response.Code, response.Body.String())
	}
	pending := requestJSON(t, server, http.MethodPost, "/api/v1/invitations", `{"project":"`+key+`","role":"read","expires_at":"`+time.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	if pending.Code != http.StatusCreated {
		t.Fatalf("create pending invitation: %d %s", pending.Code, pending.Body.String())
	}
	invitations := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+key+"/invitations", "")
	var pendingList struct {
		Invitations []store.Invitation `json:"invitations"`
	}
	if invitations.Code != http.StatusOK || strings.Contains(invitations.Body.String(), `"code"`) || json.Unmarshal(invitations.Body.Bytes(), &pendingList) != nil || len(pendingList.Invitations) != 1 {
		t.Fatalf("list pending invitations: %d %s", invitations.Code, invitations.Body.String())
	}
	if response := requestJSON(t, server, http.MethodDelete, "/api/v1/projects/"+key+"/invitations/"+strconv.FormatInt(pendingList.Invitations[0].ID, 10), ""); response.Code != http.StatusNoContent {
		t.Fatalf("revoke invitation: %d %s", response.Code, response.Body.String())
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
	if want := key + "-1"; ticket.Ref != want {
		t.Fatalf("first ticket ref = %q, want %q", ticket.Ref, want)
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
	filtered := requestJSON(t, server, http.MethodGet, "/api/v1/tickets?project="+key+"&status=open&priority=high&label=newsletter&query=avec+tags", "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), ticket.Ref) {
		t.Fatalf("list filtered tickets: %d %s", filtered.Code, filtered.Body.String())
	}
	if response := requestJSON(t, server, http.MethodGet, "/api/v1/tickets?project="+key+"&status=unknown", ""); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status filter: %d %s", response.Code, response.Body.String())
	}

	otherProject := requestJSON(t, server, http.MethodPost, "/api/v1/projects", `{"key":"`+strings.ToLower(otherKey)+`","name":"Second HTTP integration"}`)
	if otherProject.Code != http.StatusCreated {
		t.Fatalf("create second project: %d %s", otherProject.Code, otherProject.Body.String())
	}
	otherTicket := requestJSON(t, server, http.MethodPost, "/api/v1/tickets", `{"project":"`+otherKey+`","type":"user_story","title":"First ticket in another project"}`)
	if otherTicket.Code != http.StatusCreated {
		t.Fatalf("create second-project ticket: %d %s", otherTicket.Code, otherTicket.Body.String())
	}
	var other store.Ticket
	if err := json.Unmarshal(otherTicket.Body.Bytes(), &other); err != nil {
		t.Fatalf("decode second-project ticket: %v", err)
	}
	if want := otherKey + "-1"; other.Ref != want {
		t.Fatalf("first ticket in second project ref = %q, want %q", other.Ref, want)
	}
	shownOther := requestJSON(t, server, http.MethodGet, "/api/v1/tickets/"+other.Ref, "")
	if shownOther.Code != http.StatusOK || !strings.Contains(shownOther.Body.String(), other.Ref) {
		t.Fatalf("show second-project ticket: %d %s", shownOther.Code, shownOther.Body.String())
	}
	childResponse := requestJSON(t, server, http.MethodPost, "/api/v1/tickets", `{"project":"`+otherKey+`","type":"technical_task","parent_ref":"`+other.Ref+`","title":"Child ticket"}`)
	if childResponse.Code != http.StatusCreated {
		t.Fatalf("create child ticket: %d %s", childResponse.Code, childResponse.Body.String())
	}
	var child store.Ticket
	if err := json.Unmarshal(childResponse.Body.Bytes(), &child); err != nil {
		t.Fatalf("decode child ticket: %v", err)
	}
	if want := otherKey + "-2"; child.Ref != want || child.ParentRef == nil || *child.ParentRef != other.Ref {
		t.Fatalf("unexpected child ticket: %#v", child)
	}
	if response := requestJSON(t, server, http.MethodPost, "/api/v1/tickets/"+child.Ref+"/comments", `{"body":"A comment"}`); response.Code != http.StatusCreated {
		t.Fatalf("comment on child ticket: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodGet, "/api/v1/tickets/"+child.Ref+"/comments", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "A comment") || !strings.Contains(response.Body.String(), "Integration test") {
		t.Fatalf("list child comments: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodGet, "/api/v1/tickets/"+child.Ref+"/activity", ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "comment_added") {
		t.Fatalf("list child activity: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodGet, "/api/v1/tickets/"+child.Ref+"/versions", ""); response.Code != http.StatusOK {
		t.Fatalf("list child revisions: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodPost, "/api/v1/tickets/"+child.Ref+"/claim", `{}`); response.Code != http.StatusOK {
		t.Fatalf("claim child ticket: %d %s", response.Code, response.Body.String())
	}
	updatedChild := requestJSON(t, server, http.MethodPatch, "/api/v1/tickets/"+child.Ref, `{"expected_version":2,"status":"done"}`)
	if updatedChild.Code != http.StatusOK {
		t.Fatalf("update child ticket: %d %s", updatedChild.Code, updatedChild.Body.String())
	}
	statuses := requestJSON(t, server, http.MethodGet, "/api/v1/projects/status", "")
	if statuses.Code != http.StatusOK {
		t.Fatalf("project status: %d %s", statuses.Code, statuses.Body.String())
	}
	var portfolio struct {
		Projects []store.ProjectStatus `json:"projects"`
	}
	if err := json.Unmarshal(statuses.Body.Bytes(), &portfolio); err != nil {
		t.Fatalf("decode project status: %v", err)
	}
	var otherStatus *store.ProjectStatus
	for i := range portfolio.Projects {
		if portfolio.Projects[i].Key == otherKey {
			otherStatus = &portfolio.Projects[i]
			break
		}
	}
	if otherStatus == nil || otherStatus.UserStories.Total != 1 || otherStatus.UserStories.Done != 1 || otherStatus.Tickets.Total != 2 || otherStatus.Tickets.Done != 2 || otherStatus.Statuses.Done != 2 {
		t.Fatalf("unexpected project status: %#v", otherStatus)
	}
	if response := requestJSON(t, server, http.MethodPost, "/api/v1/tickets/"+child.Ref+"/versions/1/restore", `{"expected_version":3}`); response.Code != http.StatusOK {
		t.Fatalf("restore child ticket: %d %s", response.Code, response.Body.String())
	}
	if response := requestJSON(t, server, http.MethodPost, "/api/v1/tickets/"+child.Ref+"/release", `{}`); response.Code != http.StatusOK {
		t.Fatalf("release child ticket: %d %s", response.Code, response.Body.String())
	}

	// The first two projects were created while enforcement was disabled. The
	// Community plan allows three owned active projects when enforcement starts.
	if response := requestJSON(t, server, http.MethodPost, "/api/v1/projects", `{"key":"`+limitKey+`","name":"Quota limit"}`); response.Code != http.StatusCreated {
		t.Fatalf("create project up to quota: %d %s", response.Code, response.Body.String())
	}
	data.EnforceProjectLimits = true
	blocked := requestJSON(t, server, http.MethodPost, "/api/v1/projects", `{"key":"`+blockedKey+`","name":"Over quota"}`)
	if blocked.Code != http.StatusForbidden || !strings.Contains(blocked.Body.String(), `"project_limit_reached"`) {
		t.Fatalf("project limit response: %d %s", blocked.Code, blocked.Body.String())
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

func cleanupIntegrationData(t *testing.T, db *sql.DB, keys, subjects []string) {
	t.Helper()
	queries := []string{
		`DELETE FROM activity_events WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM invitations WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM ticket_revisions WHERE ticket_id IN (SELECT id FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1))`,
		`DELETE FROM comments WHERE ticket_id IN (SELECT id FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1))`,
		`DELETE FROM tickets WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM labels WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM features WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM project_memberships WHERE project_id IN (SELECT id FROM projects WHERE key=$1)`,
		`DELETE FROM projects WHERE key=$1`,
	}
	for _, key := range keys {
		for _, query := range queries {
			if _, err := db.Exec(query, key); err != nil {
				t.Errorf("cleanup integration data: %v", err)
			}
		}
	}
	for _, subject := range subjects {
		for _, query := range []string{
			`DELETE FROM credentials WHERE zitadel_subject=$1`,
			`DELETE FROM users WHERE zitadel_subject=$1`,
		} {
			if _, err := db.Exec(query, subject); err != nil {
				t.Errorf("cleanup integration data: %v", err)
			}
		}
	}
}
