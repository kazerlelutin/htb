package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kazerlelutin/htb"
)

func TestProjectTicketNumbersMigrationBackfillsEachProject(t *testing.T) {
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

	schema := fmt.Sprintf("ticket_numbers_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	}()
	if _, err = db.ExecContext(context.Background(), `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}

	for _, migration := range []string{htb.InitialMigration, htb.FeatureDueDateMigration} {
		if _, err = db.ExecContext(context.Background(), migration); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(context.Background(), `INSERT INTO schema_migrations(version) VALUES ('0001_initial'), ('0002_feature_due_date')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(), `INSERT INTO projects(key,name) VALUES ('HTB','HTB'), ('BENTO','Ben-to')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(context.Background(), `INSERT INTO tickets(project_id,type,title) VALUES (1,'bug','First HTB ticket'), (1,'bug','Second HTB ticket'), (2,'bug','First BENTO ticket')`); err != nil {
		t.Fatal(err)
	}

	if err = (&Store{DB: db}).Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryContext(context.Background(), `SELECT p.key,t.number FROM tickets t JOIN projects p ON p.id=t.project_id ORDER BY t.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var project string
		var number int
		if err = rows.Scan(&project, &number); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s-%d", project, number))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"HTB-1", "HTB-2", "BENTO-1"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("backfilled refs = %v, want %v", got, want)
	}

	for _, assertion := range []struct {
		project string
		want    int
	}{{"HTB", 3}, {"BENTO", 2}} {
		var next int
		if err = db.QueryRowContext(context.Background(), `SELECT next_ticket_number FROM projects WHERE key=$1`, assertion.project).Scan(&next); err != nil {
			t.Fatal(err)
		}
		if next != assertion.want {
			t.Fatalf("next number for %s = %d, want %d", assertion.project, next, assertion.want)
		}
	}
}
