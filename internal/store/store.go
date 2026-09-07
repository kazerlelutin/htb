package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazerlelutin/htb"
	"github.com/kazerlelutin/htb/internal/domain"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
)

type Store struct{ DB *sql.DB }
type Actor struct {
	CredentialID int64  `json:"credential_id"`
	UserID       int64  `json:"user_id"`
	Subject      string `json:"subject"`
	Superadmin   bool   `json:"superadmin"`
}
type Project struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
}
type Feature struct {
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}
type Ticket struct {
	ID           int64             `json:"id"`
	Ref          string            `json:"ref"`
	Project      string            `json:"project"`
	Type         domain.TicketType `json:"type"`
	ParentRef    *string           `json:"parent_ref,omitempty"`
	RelatedRef   *string           `json:"related_ref,omitempty"`
	FeatureKey   *string           `json:"feature_key,omitempty"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       domain.Status     `json:"status"`
	Priority     string            `json:"priority"`
	Version      int               `json:"version"`
	Labels       []string          `json:"labels"`
	ChildCount   int               `json:"child_count,omitempty"`
	DoneChildren int               `json:"done_children,omitempty"`
}
type CreateTicket struct {
	Project     string            `json:"project"`
	Type        domain.TicketType `json:"type"`
	ParentRef   string            `json:"parent_ref"`
	RelatedRef  string            `json:"related_ref"`
	FeatureKey  string            `json:"feature_key"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Priority    string            `json:"priority"`
	Labels      []string          `json:"labels"`
}
type UpdateTicket struct {
	Title           *string   `json:"title"`
	Description     *string   `json:"description"`
	Status          *string   `json:"status"`
	Priority        *string   `json:"priority"`
	Labels          *[]string `json:"labels"`
	FeatureKey      *string   `json:"feature_key"`
	ExpectedVersion int       `json:"expected_version"`
}
type Revision struct {
	Version   int             `json:"version"`
	Snapshot  json.RawMessage `json:"snapshot"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"created_at"`
}
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	migrations := []struct {
		version, sql string
	}{
		{"0001_initial", htb.InitialMigration},
		{"0002_feature_due_date", htb.FeatureDueDateMigration},
	}
	for _, migration := range migrations {
		var exists bool
		if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, migration.version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, migration.sql); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, migration.version); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ResolveActor(ctx context.Context, subject, name, email string, super bool) (Actor, error) {
	var actor Actor
	actor.Subject, actor.Superadmin = subject, super
	err := s.DB.QueryRowContext(ctx, `SELECT c.id,c.user_id FROM credentials c WHERE c.zitadel_subject=$1 AND c.disabled_at IS NULL`, subject).Scan(&actor.CredentialID, &actor.UserID)
	if err == nil {
		return actor, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return actor, err
	}
	// A valid Zitadel identity is automatically known to HTB after its first login.
	// It has no project membership until it creates or accepts a project invitation.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return actor, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `INSERT INTO users(zitadel_subject,name,email) VALUES($1,$2,$3) ON CONFLICT(zitadel_subject) DO UPDATE SET name=EXCLUDED.name RETURNING id`, subject, fallback(name, subject), nullIfEmpty(email)).Scan(&actor.UserID); err != nil {
		return actor, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO credentials(user_id,zitadel_subject,name,kind,max_role) VALUES($1,$2,$3,'human','admin') ON CONFLICT(zitadel_subject) DO NOTHING RETURNING id`, actor.UserID, subject, fallback(name, subject)).Scan(&actor.CredentialID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id FROM credentials WHERE zitadel_subject=$1 AND disabled_at IS NULL`, subject).Scan(&actor.CredentialID)
	}
	if err != nil {
		return actor, err
	}
	return actor, tx.Commit()
}

func (s *Store) CreateProject(ctx context.Context, actor Actor, p Project) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var projectID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO projects(key,name,description) VALUES($1,$2,$3) RETURNING id`, strings.ToUpper(p.Key), p.Name, p.Description).Scan(&projectID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,'admin')`, projectID, actor.UserID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListProjects(ctx context.Context, actor Actor) ([]Project, error) {
	query := `SELECT p.key,p.name,p.description,p.archived_at IS NOT NULL FROM projects p`
	args := []any{}
	if !actor.Superadmin {
		query += ` JOIN project_memberships pm ON pm.project_id=p.id WHERE pm.user_id=$1`
		args = []any{actor.UserID}
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err = rows.Scan(&p.Key, &p.Name, &p.Description, &p.Archived); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) CreateFeature(ctx context.Context, actor Actor, project string, f Feature) error {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO features(project_id,key,name,description,due_date) SELECT id,$2,$3,$4,$5 FROM projects WHERE key=$1`, strings.ToUpper(project), f.Key, f.Name, f.Description, f.DueDate)
	return err
}

func (s *Store) CreateTicket(ctx context.Context, actor Actor, in CreateTicket) (Ticket, error) {
	var result Ticket
	if err := s.authorize(ctx, actor, in.Project, domain.RoleWrite); err != nil {
		return result, err
	}
	if in.Type != domain.UserStory && in.Type != domain.TechnicalTask && in.Type != domain.Bug && in.Type != domain.Incident {
		return result, fmt.Errorf("invalid ticket type")
	}
	if strings.TrimSpace(in.Title) == "" || len(in.Title) > 240 {
		return result, fmt.Errorf("title must contain 1 to 240 characters")
	}
	if in.Description == "" {
		in.Description = domain.Template(in.Type)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	projectID, err := projectID(ctx, tx, in.Project)
	if err != nil {
		return result, err
	}
	var parentID, relatedID, featureID any
	if in.Type == domain.TechnicalTask {
		if in.ParentRef == "" {
			return result, fmt.Errorf("technical_task requires parent_ref")
		}
		var typ string
		parentID, typ, err = ticketID(ctx, tx, in.ParentRef, projectID)
		if err != nil {
			return result, err
		}
		if typ != string(domain.UserStory) {
			return result, fmt.Errorf("technical_task parent must be a user_story")
		}
	} else if in.ParentRef != "" {
		return result, fmt.Errorf("only technical_task can have a parent")
	}
	if in.RelatedRef != "" {
		relatedID, _, err = ticketID(ctx, tx, in.RelatedRef, projectID)
		if err != nil {
			return result, err
		}
	}
	if in.FeatureKey != "" {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM features WHERE project_id=$1 AND key=$2 AND archived_at IS NULL`, projectID, in.FeatureKey).Scan(&featureID); err != nil {
			return result, err
		}
	}
	priority := in.Priority
	if priority == "" {
		priority = "normal"
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO tickets(project_id,parent_ticket_id,related_ticket_id,feature_id,type,title,description,priority,created_by_credential_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, projectID, parentID, relatedID, featureID, in.Type, in.Title, in.Description, priority, actor.CredentialID).Scan(&id)
	if err != nil {
		return result, err
	}
	if err = setLabels(ctx, tx, id, projectID, in.Labels); err != nil {
		return result, err
	}
	result, err = getTicket(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if err = writeRevision(ctx, tx, result, actor.CredentialID, "created"); err != nil {
		return result, err
	}
	if err = writeEvent(ctx, tx, projectID, id, actor.CredentialID, "ticket_created", map[string]any{"type": in.Type}); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) GetTicket(ctx context.Context, actor Actor, ref string) (Ticket, error) {
	var result Ticket
	project, _, err := domain.ParseReference(ref)
	if err != nil {
		return result, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return result, err
	}
	project, id, _ := domain.ParseReference(ref)
	row := s.DB.QueryRowContext(ctx, ticketSelect+` WHERE p.key=$1 AND t.id=$2`, project, id)
	return scanTicket(row)
}

func (s *Store) ListTickets(ctx context.Context, actor Actor, project string, parentOnly bool, featureKey string) ([]Ticket, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return nil, err
	}
	query := ticketSelect + ` WHERE p.key=$1`
	arguments := []any{strings.ToUpper(project)}
	if parentOnly {
		query += ` AND t.parent_ticket_id IS NULL`
	}
	if featureKey != "" {
		query += ` AND f.key=$2`
		arguments = append(arguments, featureKey)
	}
	query += ` ORDER BY t.id DESC`
	rows, err := s.DB.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Ticket
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ticket)
	}
	return result, rows.Err()
}

func (s *Store) UpdateTicket(ctx context.Context, actor Actor, ref string, in UpdateTicket) (Ticket, error) {
	var result Ticket
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return result, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleWrite); err != nil {
		return result, err
	}
	if in.ExpectedVersion < 1 {
		return result, fmt.Errorf("expected_version is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	current, err := getTicket(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if current.Project != project {
		return result, ErrNotFound
	}
	if current.Version != in.ExpectedVersion {
		return result, ErrConflict
	}
	if current.Type == domain.UserStory && in.Status != nil {
		return result, fmt.Errorf("user_story status is calculated from its tasks")
	}
	title, description, priority, status := current.Title, current.Description, current.Priority, string(current.Status)
	if in.Title != nil {
		title = *in.Title
	}
	if in.Description != nil {
		description = *in.Description
	}
	if in.Priority != nil {
		priority = *in.Priority
	}
	if in.Status != nil {
		status = *in.Status
	}
	featureKey := ""
	if current.FeatureKey != nil {
		featureKey = *current.FeatureKey
	}
	if in.FeatureKey != nil {
		featureKey = strings.TrimSpace(*in.FeatureKey)
	}
	var featureID any
	if featureKey != "" {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM features WHERE project_id=$1 AND key=$2 AND archived_at IS NULL`, currentProjectID(ctx, tx, id), featureKey).Scan(&featureID); err != nil {
			return result, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tickets SET title=$1,description=$2,priority=$3,status=$4,feature_id=$5,version=version+1,updated_at=now(),closed_at=CASE WHEN $4='done' THEN now() ELSE NULL END WHERE id=$6`, title, description, priority, status, featureID, id); err != nil {
		return result, err
	}
	if in.Labels != nil {
		if err = setLabels(ctx, tx, id, currentProjectID(ctx, tx, id), *in.Labels); err != nil {
			return result, err
		}
	}
	result, err = getTicket(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if err = writeRevision(ctx, tx, result, actor.CredentialID, "updated"); err != nil {
		return result, err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "ticket_updated", map[string]any{"version": result.Version}); err != nil {
		return result, err
	}
	if current.ParentRef != nil {
		if err = refreshStory(ctx, tx, *current.ParentRef, actor.CredentialID); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) Revisions(ctx context.Context, actor Actor, ref string) ([]Revision, error) {
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return nil, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT version,snapshot,reason,created_at FROM ticket_revisions WHERE ticket_id=$1 ORDER BY version DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Revision
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.Version, &r.Snapshot, &r.Reason, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Restore(ctx context.Context, actor Actor, ref string, revision, expectedVersion int) (Ticket, error) {
	var out Ticket
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return out, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleWrite); err != nil {
		return out, err
	}
	if expectedVersion < 1 {
		return out, fmt.Errorf("expected_version is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	current, err := getTicket(ctx, tx, id)
	if err != nil {
		return out, err
	}
	if current.Version != expectedVersion {
		return out, ErrConflict
	}
	var snapshot json.RawMessage
	if err = tx.QueryRowContext(ctx, `SELECT snapshot FROM ticket_revisions WHERE ticket_id=$1 AND version=$2`, id, revision).Scan(&snapshot); err != nil {
		return out, err
	}
	var source Ticket
	if err = json.Unmarshal(snapshot, &source); err != nil {
		return out, err
	}
	status := source.Status
	if current.Type == domain.UserStory {
		status = current.Status
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tickets SET title=$1,description=$2,status=$3,priority=$4,version=version+1,updated_at=now(),closed_at=CASE WHEN $3='done' THEN now() ELSE NULL END WHERE id=$5`, source.Title, source.Description, status, source.Priority, id); err != nil {
		return out, err
	}
	if err = setLabels(ctx, tx, id, currentProjectID(ctx, tx, id), source.Labels); err != nil {
		return out, err
	}
	out, err = getTicket(ctx, tx, id)
	if err != nil {
		return out, err
	}
	if err = writeRevision(ctx, tx, out, actor.CredentialID, fmt.Sprintf("restored from version %d", revision)); err != nil {
		return out, err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "ticket_restored", map[string]any{"revision": revision}); err != nil {
		return out, err
	}
	if current.ParentRef != nil {
		if err = refreshStory(ctx, tx, *current.ParentRef, actor.CredentialID); err != nil {
			return out, err
		}
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) AddComment(ctx context.Context, actor Actor, ref, body string) (Comment, error) {
	var out Comment
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return out, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleWrite); err != nil {
		return out, err
	}
	if len(strings.TrimSpace(body)) == 0 || len(body) > 20000 {
		return out, fmt.Errorf("comment must contain 1 to 20000 characters")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `INSERT INTO comments(ticket_id,body,author_credential_id) VALUES($1,$2,$3) RETURNING id,body,created_at`, id, body, actor.CredentialID).Scan(&out.ID, &out.Body, &out.CreatedAt)
	if err != nil {
		return out, err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "comment_added", map[string]any{"comment_id": out.ID}); err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func (s *Store) Claim(ctx context.Context, actor Actor, ref string) (Ticket, error) {
	var out Ticket
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return out, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleWrite); err != nil {
		return out, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE tickets SET claimed_by_credential_id=$1,claimed_at=now(),status=CASE WHEN status='open' THEN 'in_progress' ELSE status END,version=version+1,updated_at=now() WHERE id=$2 AND claimed_by_credential_id IS NULL`, actor.CredentialID, id)
	if err != nil {
		return out, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return out, err
	}
	if count == 0 {
		return out, ErrConflict
	}
	out, err = getTicket(ctx, tx, id)
	if err != nil {
		return out, err
	}
	if err = writeRevision(ctx, tx, out, actor.CredentialID, "claimed"); err != nil {
		return out, err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "ticket_claimed", map[string]any{}); err != nil {
		return out, err
	}
	if out.ParentRef != nil {
		if err = refreshStory(ctx, tx, *out.ParentRef, actor.CredentialID); err != nil {
			return out, err
		}
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) Release(ctx context.Context, actor Actor, ref string) error {
	project, id, err := domain.ParseReference(ref)
	if err != nil {
		return err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleWrite); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT claimed_by_credential_id FROM tickets WHERE id=$1 FOR UPDATE`, id).Scan(&owner); err != nil {
		return err
	}
	if !owner.Valid {
		return ErrConflict
	}
	if owner.Int64 != actor.CredentialID && !actor.Superadmin {
		if err = s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tickets SET claimed_by_credential_id=NULL,claimed_at=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "ticket_released", map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateInvitation(ctx context.Context, actor Actor, project string, role domain.Role, expires time.Time) (string, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return "", err
	}
	if !role.Allows(domain.RoleRead) || expires.Before(time.Now()) {
		return "", fmt.Errorf("invalid invitation")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(code))
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invitations(project_id,role,code_hash,expires_at,created_by_credential_id) SELECT id,$2,$3,$4,$5 FROM projects WHERE key=$1`, strings.ToUpper(project), role, hash[:], expires, actor.CredentialID)
	return code, err
}

func (s *Store) AcceptInvitation(ctx context.Context, subject, name, email, code string) error {
	hash := sha256.Sum256([]byte(code))
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var projectID, userID int64
	var role string
	err = tx.QueryRowContext(ctx, `SELECT project_id,role FROM invitations WHERE code_hash=$1 AND accepted_at IS NULL AND expires_at>now() FOR UPDATE`, hash[:]).Scan(&projectID, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE zitadel_subject=$1`, subject).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(ctx, `INSERT INTO users(zitadel_subject,name,email) VALUES($1,$2,$3) RETURNING id`, subject, fallback(name, subject), nullIfEmpty(email)).Scan(&userID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO credentials(user_id,zitadel_subject,name,kind,max_role) VALUES($1,$2,$3,'human','admin')`, userID, subject, fallback(name, subject))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT(project_id,user_id) DO UPDATE SET role=EXCLUDED.role`, projectID, userID, role); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE invitations SET accepted_by_user_id=$1,accepted_at=now() WHERE code_hash=$2`, userID, hash[:]); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) authorize(ctx context.Context, actor Actor, project string, required domain.Role) error {
	if actor.Superadmin {
		return nil
	}
	var role, max string
	err := s.DB.QueryRowContext(ctx, `SELECT pm.role,c.max_role FROM credentials c JOIN project_memberships pm ON pm.user_id=c.user_id JOIN projects p ON p.id=pm.project_id WHERE c.id=$1 AND p.key=$2 AND c.disabled_at IS NULL`, actor.CredentialID, strings.ToUpper(project)).Scan(&role, &max)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	effective := domain.Role(role)
	if !domain.Role(max).Allows(effective) {
		effective = domain.Role(max)
	}
	if !effective.Allows(required) {
		return ErrForbidden
	}
	return nil
}

type scanner interface{ Scan(...any) error }

const ticketSelect = `SELECT t.id,p.key,t.type,CASE WHEN pt.id IS NULL THEN NULL ELSE pp.key || '-' || pt.id::text END,CASE WHEN rt.id IS NULL THEN NULL ELSE rtp.key || '-' || rt.id::text END,f.key,t.title,t.description,t.status,t.priority,t.version, COALESCE((SELECT json_agg(l.name ORDER BY l.name) FROM ticket_labels tl JOIN labels l ON l.id=tl.label_id WHERE tl.ticket_id=t.id),'[]'::json), (SELECT count(*) FROM tickets c WHERE c.parent_ticket_id=t.id), (SELECT count(*) FROM tickets c WHERE c.parent_ticket_id=t.id AND c.status='done') FROM tickets t JOIN projects p ON p.id=t.project_id LEFT JOIN tickets pt ON pt.id=t.parent_ticket_id LEFT JOIN projects pp ON pp.id=pt.project_id LEFT JOIN tickets rt ON rt.id=t.related_ticket_id LEFT JOIN projects rtp ON rtp.id=rt.project_id LEFT JOIN features f ON f.id=t.feature_id`

func scanTicket(row scanner) (Ticket, error) {
	var t Ticket
	var parent, related, feature sql.NullString
	var labelsJSON []byte
	var typ, status string
	err := row.Scan(&t.ID, &t.Project, &typ, &parent, &related, &feature, &t.Title, &t.Description, &status, &t.Priority, &t.Version, &labelsJSON, &t.ChildCount, &t.DoneChildren)
	if err != nil {
		return t, err
	}
	t.Ref = fmt.Sprintf("%s-%d", t.Project, t.ID)
	t.Type = domain.TicketType(typ)
	t.Status = domain.Status(status)
	if parent.Valid {
		t.ParentRef = &parent.String
	}
	if related.Valid {
		t.RelatedRef = &related.String
	}
	if feature.Valid {
		t.FeatureKey = &feature.String
	}
	labels, err := decodeTicketLabels(labelsJSON)
	if err != nil {
		return t, err
	}
	t.Labels = labels
	return t, nil
}

func decodeTicketLabels(raw []byte) ([]string, error) {
	var labels []string
	if err := json.Unmarshal(raw, &labels); err != nil {
		return nil, fmt.Errorf("decode ticket labels: %w", err)
	}
	return labels, nil
}
func getTicket(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) (Ticket, error) {
	return scanTicket(q.QueryRowContext(ctx, ticketSelect+` WHERE t.id=$1`, id))
}
func projectID(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, key string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM projects WHERE key=$1`, strings.ToUpper(key)).Scan(&id)
	return id, err
}
func ticketID(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, ref string, project int64) (any, string, error) {
	_, id, err := domain.ParseReference(ref)
	if err != nil {
		return nil, "", err
	}
	var typ string
	err = q.QueryRowContext(ctx, `SELECT id,type FROM tickets WHERE id=$1 AND project_id=$2`, id, project).Scan(&id, &typ)
	return id, typ, err
}
func setLabels(ctx context.Context, tx *sql.Tx, ticketID, projectID int64, labels []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_labels WHERE ticket_id=$1`, ticketID); err != nil {
		return err
	}
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		var id int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO labels(project_id,name) VALUES($1,$2) ON CONFLICT(project_id,name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, projectID, label).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO ticket_labels(ticket_id,label_id) VALUES($1,$2)`, ticketID, id); err != nil {
			return err
		}
	}
	return nil
}
func writeRevision(ctx context.Context, tx *sql.Tx, t Ticket, actor int64, reason string) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ticket_revisions(ticket_id,version,snapshot,author_credential_id,reason) VALUES($1,$2,$3,$4,$5)`, t.ID, t.Version, b, actor, reason)
	return err
}
func writeEvent(ctx context.Context, tx *sql.Tx, project, ticket, actor int64, action string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_events(project_id,ticket_id,action,payload,actor_credential_id) VALUES($1,$2,$3,$4,$5)`, project, ticket, action, b, actor)
	return err
}
func currentProjectID(ctx context.Context, tx *sql.Tx, id int64) int64 {
	var project int64
	_ = tx.QueryRowContext(ctx, `SELECT project_id FROM tickets WHERE id=$1`, id).Scan(&project)
	return project
}
func refreshStory(ctx context.Context, tx *sql.Tx, parentRef string, actor int64) error {
	_, id, err := domain.ParseReference(parentRef)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT status FROM tickets WHERE parent_ticket_id=$1`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	var statuses []domain.Status
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return err
		}
		statuses = append(statuses, domain.Status(s))
	}
	status := domain.AggregateStoryStatus(statuses)
	var old string
	err = tx.QueryRowContext(ctx, `SELECT status FROM tickets WHERE id=$1 FOR UPDATE`, id).Scan(&old)
	if err != nil {
		return err
	}
	if old == string(status) {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tickets SET status=$1,version=version+1,updated_at=now(),closed_at=CASE WHEN $1='done' THEN now() ELSE NULL END WHERE id=$2`, status, id); err != nil {
		return err
	}
	ticket, err := getTicket(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = writeRevision(ctx, tx, ticket, actor, "status recalculated"); err != nil {
		return err
	}
	return writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor, "story_status_recalculated", map[string]any{"status": status})
}
func fallback(v, other string) string {
	if strings.TrimSpace(v) == "" {
		return other
	}
	return v
}
func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
