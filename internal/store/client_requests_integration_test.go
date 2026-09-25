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

func TestClientRequestProjectFlow(t *testing.T) {
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
	schema := fmt.Sprintf("client_requests_%d", time.Now().UnixNano())
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
	admin, err := s.ResolveActor(ctx, "owner", "Owner", "owner@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateProject(ctx, admin, Project{Key: "SITE", Name: "Site"}); err != nil {
		t.Fatal(err)
	}
	outsider, err := s.BrowserActor(ctx, "client", "Client", "client@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateClientRequest(ctx, outsider, "SITE", "Subject", "Body"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("uninvited access: %v", err)
	}
	code, err := s.CreateInvitation(ctx, admin, "SITE", domain.RoleRead, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AcceptInvitation(ctx, outsider.Subject, "", "", code); err != nil {
		t.Fatal(err)
	}
	request, err := s.CreateClientRequest(ctx, outsider, "SITE", "Subject", "# Description")
	if err != nil || request.Status != "received" {
		t.Fatalf("create: %+v, %v", request, err)
	}
	items, err := s.ListClientRequests(ctx, outsider, "SITE")
	if err != nil || len(items) != 1 || items[0].ID != request.ID {
		t.Fatalf("list: %+v, %v", items, err)
	}
	if _, err = s.AddClientRequestComment(ctx, outsider, request.ID, "Question"); err != nil {
		t.Fatal(err)
	}
	comments, err := s.ListClientRequestComments(ctx, admin, request.ID)
	if err != nil || len(comments) != 1 || comments[0].Body != "Question" {
		t.Fatalf("comments: %+v, %v", comments, err)
	}
	status := "in_progress"
	if _, err = s.UpdateClientRequest(ctx, outsider, request.ID, ClientRequestUpdate{Status: &status}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read member updated status: %v", err)
	}
	ticket, err := s.CreateTicket(ctx, admin, CreateTicket{Project: "SITE", Type: domain.UserStory, Title: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.UpdateClientRequest(ctx, admin, request.ID, ClientRequestUpdate{Status: &status, LinkedTicketRef: &ticket.Ref})
	if err != nil || updated.Status != status || updated.LinkedTicketRef == nil || *updated.LinkedTicketRef != ticket.Ref {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	if err = s.DeleteClientRequest(ctx, outsider, request.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("client deleted an in-progress request: %v", err)
	}
	rejected := "rejected"
	if _, err = s.UpdateClientRequest(ctx, admin, request.ID, ClientRequestUpdate{Status: &rejected}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err = s.DeleteClientRequest(ctx, outsider, request.ID); err != nil {
		t.Fatalf("delete rejected request: %v", err)
	}
	if _, err = s.GetClientRequest(ctx, outsider, request.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted request is still available: %v", err)
	}
}
