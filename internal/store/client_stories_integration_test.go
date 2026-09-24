package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kazerlelutin/htb/internal/domain"
)

func TestClientStoryPublicationAndPublicConversation(t *testing.T) {
	dsn := os.Getenv("HTBD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HTBD_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("client_stories_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, dropErr := db.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`); dropErr != nil {
			t.Errorf("drop test schema: %v", dropErr)
		}
	}()
	if _, err = db.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	s := &Store{DB: db}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	admin, err := s.ResolveActor(ctx, "story-owner", "Owner", "owner@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateProject(ctx, admin, Project{Key: "SITE", Name: "Site"}); err != nil {
		t.Fatal(err)
	}
	client, err := s.BrowserActor(ctx, "story-client", "Client", "client@example.test")
	if err != nil {
		t.Fatal(err)
	}
	code, err := s.CreateInvitation(ctx, admin, "SITE", domain.RoleRead, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AcceptInvitation(ctx, client.Subject, "", "", code); err != nil {
		t.Fatal(err)
	}
	story, err := s.CreateTicket(ctx, admin, CreateTicket{Project: "SITE", Type: domain.UserStory, Title: "Exporter les données", Description: "Besoin client"})
	if err != nil {
		t.Fatal(err)
	}
	technical, err := s.CreateTicket(ctx, admin, CreateTicket{Project: "SITE", Type: domain.TechnicalTask, ParentRef: story.Ref, Title: "Secret: endpoint interne"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddComment(ctx, admin, story.Ref, "Note technique privée"); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListClientStories(ctx, client, "SITE")
	if err != nil || len(items) != 0 {
		t.Fatalf("unpublished story visible: %+v, %v", items, err)
	}
	adminItems, err := s.ListClientStories(ctx, admin, "SITE")
	if err != nil || len(adminItems) != 1 || adminItems[0].Ref != story.Ref || adminItems[0].Published || adminItems[0].Visibility != ClientStoryDraft {
		t.Fatalf("administrator cannot preview draft: %+v, %v", adminItems, err)
	}
	if draft, err := s.GetClientStory(ctx, admin, story.Ref); err != nil || draft.Published || draft.Visibility != ClientStoryDraft {
		t.Fatalf("administrator draft detail: %+v, %v", draft, err)
	}
	internalComments, err := s.ListInternalStoryComments(ctx, admin, story.Ref)
	if err != nil || len(internalComments) != 1 || internalComments[0].Body != "Note technique privée" {
		t.Fatalf("administrator draft internal comments: %+v, %v", internalComments, err)
	}
	if _, err := s.ListClientStoryComments(ctx, admin, story.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft conversation should remain closed: %v", err)
	}
	if _, err = s.GetClientStory(ctx, client, story.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unpublished detail: %v", err)
	}
	if err = s.SetClientStoryPublished(ctx, client, story.Ref, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("client published story: %v", err)
	}
	if err = s.SetClientStoryPublished(ctx, admin, technical.Ref, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("technical ticket published: %v", err)
	}
	if err = s.SetClientStoryPublished(ctx, admin, story.Ref, true); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListClientStories(ctx, client, "SITE")
	if err != nil || len(items) != 1 || items[0].Ref != story.Ref || items[0].ChildCount != 1 || items[0].DoneChildren != 0 || !items[0].Published || items[0].Visibility != ClientStoryPublished {
		t.Fatalf("published story projection: %+v, %v", items, err)
	}
	if items[0].Title == technical.Title || items[0].Description == "Note technique privée" {
		t.Fatalf("internal content leaked in projection: %+v", items[0])
	}
	comments, err := s.ListClientStoryComments(ctx, client, story.Ref)
	if err != nil || len(comments) != 0 {
		t.Fatalf("internal comment leaked: %+v, %v", comments, err)
	}
	if _, err := s.ListInternalStoryComments(ctx, client, story.Ref); !errors.Is(err, ErrForbidden) {
		t.Fatalf("client read internal comments: %v", err)
	}
	if _, err := s.AddInternalStoryComment(ctx, client, story.Ref, "Secret"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("client added internal comment: %v", err)
	}
	if _, err = s.AddClientStoryComment(ctx, client, story.Ref, "Question client"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddClientStoryComment(ctx, admin, story.Ref, "Réponse équipe"); err != nil {
		t.Fatal(err)
	}
	comments, err = s.ListClientStoryComments(ctx, client, story.Ref)
	if err != nil || len(comments) != 2 || comments[0].Body != "Question client" || comments[1].Body != "Réponse équipe" {
		t.Fatalf("public conversation: %+v, %v", comments, err)
	}
	completed := string(domain.Done)
	if _, err = s.UpdateTicket(ctx, admin, technical.Ref, UpdateTicket{Status: &completed, ExpectedVersion: technical.Version}); err != nil {
		t.Fatal(err)
	}
	visible, err := s.GetClientStory(ctx, client, story.Ref)
	if err != nil || visible.ChildCount != 1 || visible.DoneChildren != 1 || visible.Status != string(domain.Done) {
		t.Fatalf("derived progress: %+v, %v", visible, err)
	}
	if err = s.SetClientStoryPublished(ctx, admin, story.Ref, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetClientStory(ctx, client, story.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unpublished detail remained visible: %v", err)
	}
	if draft, err := s.GetClientStory(ctx, admin, story.Ref); err != nil || draft.Published || draft.Visibility != ClientStoryDraft {
		t.Fatalf("administrator lost unpublished story: %+v, %v", draft, err)
	}
	if _, err = s.AddClientStoryComment(ctx, client, story.Ref, "Hidden"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("comment on hidden story: %v", err)
	}
}
