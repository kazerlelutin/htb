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

func TestTicketArchiveAndDeletionPermissions(t *testing.T) {
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
	schema := fmt.Sprintf("ticket_lifecycle_%d", time.Now().UnixNano())
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
	admin, err := s.ResolveActor(ctx, "ticket-admin", "Admin", "admin@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateProject(ctx, admin, Project{Key: "SITE", Name: "Site"}); err != nil {
		t.Fatal(err)
	}
	creator, err := s.ResolveActor(ctx, "ticket-creator", "Creator", "creator@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := s.ResolveActor(ctx, "ticket-reader", "Reader", "reader@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, invitee := range []struct {
		actor Actor
		role  domain.Role
	}{
		{creator, domain.RoleWrite},
		{reader, domain.RoleRead},
	} {
		code, inviteErr := s.CreateInvitation(ctx, admin, "SITE", invitee.role, time.Now().Add(time.Hour))
		if inviteErr != nil {
			t.Fatal(inviteErr)
		}
		if inviteErr = s.AcceptInvitation(ctx, invitee.actor.Subject, invitee.actor.Name, "", code); inviteErr != nil {
			t.Fatal(inviteErr)
		}
	}

	ticket, err := s.CreateTicket(ctx, creator, CreateTicket{Project: "SITE", Type: domain.Bug, Title: "Typo"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetTicketArchived(ctx, reader, ticket.Ref, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reader archived another ticket: %v", err)
	}
	if err = s.SetTicketArchived(ctx, creator, ticket.Ref, true); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListTickets(ctx, creator, "SITE", TicketFilter{})
	if err != nil || len(active) != 0 {
		t.Fatalf("archived ticket remained active: %#v, %v", active, err)
	}
	archived, err := s.ListTickets(ctx, creator, "SITE", TicketFilter{Archived: true})
	if err != nil || len(archived) != 1 || !archived[0].Archived {
		t.Fatalf("archived ticket missing from archive: %#v, %v", archived, err)
	}
	if err = s.SetTicketArchived(ctx, admin, ticket.Ref, false); err != nil {
		t.Fatalf("administrator could not restore ticket: %v", err)
	}
	if err = s.DeleteTicket(ctx, reader, ticket.Ref); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reader deleted another ticket: %v", err)
	}
	if err = s.DeleteTicket(ctx, creator, ticket.Ref); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetTicket(ctx, admin, ticket.Ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted ticket remained readable: %v", err)
	}

	story, err := s.CreateTicket(ctx, creator, CreateTicket{Project: "SITE", Type: domain.UserStory, Title: "Duplicate story"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTicket(ctx, creator, CreateTicket{Project: "SITE", Type: domain.TechnicalTask, ParentRef: story.Ref, Title: "Child"}); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteTicket(ctx, admin, story.Ref); !errors.Is(err, ErrTicketHasDependencies) {
		t.Fatalf("deleted referenced ticket: %v", err)
	}
}
