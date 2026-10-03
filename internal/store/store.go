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
	ErrNotFound              = errors.New("not found")
	ErrForbidden             = errors.New("forbidden")
	ErrConflict              = errors.New("conflict")
	ErrProjectLimit          = errors.New("project limit reached")
	ErrNamespaceLimit        = errors.New("namespace limit reached")
	ErrNamespaceRequired     = errors.New("namespace must be claimed before it can be used")
	ErrNamespaceReserved     = errors.New("namespace is reserved by another account")
	ErrTicketHasDependencies = errors.New("ticket has dependent records")
)

type Store struct {
	DB                   *sql.DB
	EnforceProjectLimits bool
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Actor struct {
	CredentialID int64  `json:"credential_id"`
	UserID       int64  `json:"user_id"`
	Subject      string `json:"subject"`
	Name         string `json:"-"`
	Superadmin   bool   `json:"superadmin"`
}
type Project struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
}
type Namespace struct {
	Name string `json:"name"`
}
type Progress struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}
type StatusCounts struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Review     int `json:"review"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
}
type ProjectStatus struct {
	Project
	UserStories Progress     `json:"user_stories"`
	Tickets     Progress     `json:"tickets"`
	Statuses    StatusCounts `json:"statuses"`
}
type Feature struct {
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}
type ProjectMember struct {
	UserID int64       `json:"user_id"`
	Name   string      `json:"name"`
	Email  string      `json:"email"`
	Role   domain.Role `json:"role"`
	Owner  bool        `json:"owner"`
}
type Invitation struct {
	ID        int64       `json:"id"`
	Role      domain.Role `json:"role"`
	ExpiresAt time.Time   `json:"expires_at"`
	CreatedAt time.Time   `json:"created_at"`
}
type Ticket struct {
	ID           int64             `json:"id"`
	Number       int64             `json:"-"`
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
	Published    bool              `json:"published"`
	Archived     bool              `json:"archived"`
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
type TicketFilter struct {
	ParentOnly bool
	Archived   bool
	FeatureKey string
	Status     domain.Status
	Priority   string
	Label      string
	Query      string
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
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}
type Activity struct {
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	Actor     string          `json:"actor"`
	CreatedAt time.Time       `json:"created_at"`
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
		{"0003_project_ticket_numbers", htb.ProjectTicketNumbersMigration},
		{"0004_account_project_limits", htb.AccountProjectLimitsMigration},
		{"0005_web_sessions", htb.WebSessionsMigration},
		{"0006_client_requests", htb.ClientRequestsMigration},
		{"0007_client_stories", htb.ClientStoriesMigration},
		{"0008_client_story_visibility", htb.ClientStoryVisibilityMigration},
		{"0009_client_request_rejection", htb.ClientRequestRejectionMigration},
		{"0010_namespaced_project_keys", htb.NamespacedProjectKeysMigration},
		{"0011_namespace_reservations", htb.NamespaceReservationsMigration},
		{"0012_client_story_comment_lifecycle", htb.ClientStoryCommentLifecycleMigration},
		{"0013_ticket_lifecycle", htb.TicketLifecycleMigration},
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
	err := s.DB.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.name FROM credentials c WHERE c.zitadel_subject=$1 AND c.disabled_at IS NULL`, subject).Scan(&actor.CredentialID, &actor.UserID, &actor.Name)
	if err == nil {
		if name = strings.TrimSpace(name); name != "" {
			if _, err = s.DB.ExecContext(ctx, `UPDATE users SET name=$2,email=COALESCE(NULLIF($3,''),email) WHERE id=$1`, actor.UserID, name, strings.TrimSpace(email)); err != nil {
				return actor, err
			}
			if _, err = s.DB.ExecContext(ctx, `UPDATE credentials SET name=$2 WHERE id=$1`, actor.CredentialID, name); err != nil {
				return actor, err
			}
			actor.Name = name
		}
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
	actor.Name = fallback(name, subject)
	return actor, tx.Commit()
}

// BrowserActor records a verified Zitadel identity without granting project
// access. A project remains inaccessible until an invitation is accepted.
func (s *Store) BrowserActor(ctx context.Context, subject, name, email string) (Actor, error) {
	return s.ResolveActor(ctx, subject, name, email, false)
}

// CreateWebSession returns an opaque token. Only its hash is persisted so a
// database disclosure cannot be replayed as a browser session.
func (s *Store) CreateWebSession(ctx context.Context, actor Actor, lifetime time.Duration) (string, error) {
	if actor.CredentialID < 1 || lifetime <= 0 {
		return "", fmt.Errorf("invalid web session")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	_, err := s.DB.ExecContext(ctx, `INSERT INTO web_sessions(token_hash,credential_id,expires_at) VALUES($1,$2,$3)`, hash[:], actor.CredentialID, time.Now().Add(lifetime))
	if err != nil {
		return "", err
	}
	return token, nil
}

// WebSessionActor resolves only an unexpired, non-revoked browser session.
func (s *Store) WebSessionActor(ctx context.Context, token string) (Actor, error) {
	var actor Actor
	if len(token) != 64 {
		return actor, ErrNotFound
	}
	hash := sha256.Sum256([]byte(token))
	err := s.DB.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.zitadel_subject,c.name FROM web_sessions s JOIN credentials c ON c.id=s.credential_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND c.disabled_at IS NULL`, hash[:]).Scan(&actor.CredentialID, &actor.UserID, &actor.Subject, &actor.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return actor, ErrNotFound
	}
	return actor, err
}

// RevokeWebSession invalidates the current browser token without revealing
// whether a supplied token was valid.
func (s *Store) RevokeWebSession(ctx context.Context, token string) error {
	if len(token) != 64 {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	_, err := s.DB.ExecContext(ctx, `UPDATE web_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash[:])
	return err
}

func (s *Store) CreateProject(ctx context.Context, actor Actor, p Project) error {
	key, err := domain.NormalizeProjectKey(p.Key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.authorizeNamespace(ctx, tx, actor, key); err != nil {
		return err
	}
	if s.EnforceProjectLimits {
		if err = s.checkProjectLimit(ctx, tx, actor.UserID); err != nil {
			return err
		}
	}
	var projectID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO projects(key,name,description,owner_user_id) VALUES($1,$2,$3,$4) RETURNING id`, key, p.Name, p.Description, actor.UserID).Scan(&projectID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_memberships(project_id,user_id,role) VALUES($1,$2,'admin')`, projectID, actor.UserID); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateProjectKey changes the key of an existing project.
// The new key must be valid and unique.
func (s *Store) UpdateProjectKey(ctx context.Context, actor Actor, oldKey string, newKey string) error {
	normalized, err := domain.NormalizeProjectKey(newKey)
	if err != nil {
		return err
	}
	if err := s.authorize(ctx, actor, oldKey, domain.RoleAdmin); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.authorizeNamespace(ctx, tx, actor, normalized); err != nil {
		return err
	}
	// Check uniqueness
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE key=$1)`, normalized).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrConflict
	}
	// Update the key
	_, err = tx.ExecContext(ctx, `UPDATE projects SET key=$1 WHERE key=$2`, normalized, strings.ToUpper(oldKey))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// SetProjectArchived preserves a project's data while removing or restoring it
// from active work. Only its owner (or a superadmin) can change this state.
func (s *Store) SetProjectArchived(ctx context.Context, actor Actor, project string, archived bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, ownerID, err := projectAdministration(ctx, tx, project)
	if err != nil {
		return err
	}
	if !actor.Superadmin && actor.UserID != ownerID {
		return ErrForbidden
	}
	if archived {
		_, err = tx.ExecContext(ctx, `UPDATE projects SET archived_at=COALESCE(archived_at,now()) WHERE id=$1`, projectID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE projects SET archived_at=NULL WHERE id=$1`, projectID)
	}
	if err != nil {
		return err
	}
	action := "project_restored"
	if archived {
		action = "project_archived"
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, action, map[string]any{"project": strings.ToUpper(project)}); err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimNamespace reserves a namespace prefix for the authenticated account.
// Repeating a claim by its owner is safe and idempotent.
func (s *Store) ClaimNamespace(ctx context.Context, actor Actor, name string) (Namespace, error) {
	normalized, err := domain.NormalizeNamespace(name)
	if err != nil {
		return Namespace{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Namespace{}, err
	}
	defer tx.Rollback()
	var ownerID int64
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id FROM namespaces WHERE name=$1 FOR UPDATE`, normalized).Scan(&ownerID)
	if err == nil {
		if ownerID != actor.UserID {
			return Namespace{}, ErrNamespaceReserved
		}
		return Namespace{Name: normalized}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Namespace{}, err
	}
	if err = s.checkNamespaceLimit(ctx, tx, actor.UserID); err != nil {
		return Namespace{}, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO namespaces(name,owner_user_id) VALUES($1,$2) ON CONFLICT (name) DO NOTHING RETURNING owner_user_id`, normalized, actor.UserID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(ctx, `SELECT owner_user_id FROM namespaces WHERE name=$1`, normalized).Scan(&ownerID); err != nil {
			return Namespace{}, err
		}
		if ownerID != actor.UserID {
			return Namespace{}, ErrNamespaceReserved
		}
	} else if err != nil {
		return Namespace{}, err
	}
	if err = tx.Commit(); err != nil {
		return Namespace{}, err
	}
	return Namespace{Name: normalized}, nil
}

func (s *Store) ListNamespaces(ctx context.Context, actor Actor) ([]Namespace, error) {
	query, args := `SELECT name FROM namespaces`, []any{}
	if !actor.Superadmin {
		query += ` WHERE owner_user_id=$1`
		args = append(args, actor.UserID)
	}
	query += ` ORDER BY name`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var namespaces []Namespace
	for rows.Next() {
		var namespace Namespace
		if err = rows.Scan(&namespace.Name); err != nil {
			return nil, err
		}
		namespaces = append(namespaces, namespace)
	}
	return namespaces, rows.Err()
}

func (s *Store) authorizeNamespace(ctx context.Context, tx *sql.Tx, actor Actor, projectKey string) error {
	namespace, _ := domain.SplitProjectKey(projectKey)
	if namespace == "" {
		return nil
	}
	var ownerID int64
	err := tx.QueryRowContext(ctx, `SELECT owner_user_id FROM namespaces WHERE name=$1`, namespace).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNamespaceRequired
	}
	if err != nil {
		return err
	}
	if ownerID != actor.UserID {
		return ErrNamespaceReserved
	}
	return nil
}

// checkProjectLimit serializes project creation for one account, so concurrent
// requests cannot both pass the quota check.
func (s *Store) checkProjectLimit(ctx context.Context, tx *sql.Tx, userID int64) error {
	if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
		return err
	}
	var limit sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT max_projects FROM plans
		WHERE code=COALESCE((SELECT plan_code FROM user_plans WHERE user_id=$1), 'community')
	`, userID).Scan(&limit)
	if err != nil {
		return err
	}
	if !limit.Valid {
		return nil
	}
	var owned int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE owner_user_id=$1 AND archived_at IS NULL`, userID).Scan(&owned); err != nil {
		return err
	}
	if owned >= limit.Int64 {
		return ErrProjectLimit
	}
	return nil
}

func (s *Store) checkNamespaceLimit(ctx context.Context, tx *sql.Tx, userID int64) error {
	if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
		return err
	}
	var limit sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT max_namespaces FROM plans
		WHERE code=COALESCE((SELECT plan_code FROM user_plans WHERE user_id=$1), 'community')
	`, userID).Scan(&limit)
	if err != nil {
		return err
	}
	if !limit.Valid {
		return nil
	}
	var owned int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM namespaces WHERE owner_user_id=$1`, userID).Scan(&owned); err != nil {
		return err
	}
	if owned >= limit.Int64 {
		return ErrNamespaceLimit
	}
	return nil
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

// ListProjectStatuses returns each accessible project's ticket totals in one
// query, so the CLI can render a portfolio view without an N+1 request loop.
func (s *Store) ListProjectStatuses(ctx context.Context, actor Actor) ([]ProjectStatus, error) {
	query := `SELECT p.key,p.name,p.description,p.archived_at IS NOT NULL,
		count(t.id) FILTER (WHERE t.type='user_story'),
		count(t.id) FILTER (WHERE t.type='user_story' AND t.status='done'),
		count(t.id), count(t.id) FILTER (WHERE t.status='done'),
		count(t.id) FILTER (WHERE t.status='open'),
		count(t.id) FILTER (WHERE t.status='in_progress'),
		count(t.id) FILTER (WHERE t.status='review'),
		count(t.id) FILTER (WHERE t.status='blocked')
		FROM projects p LEFT JOIN tickets t ON t.project_id=p.id AND t.archived_at IS NULL`
	args := []any{}
	if !actor.Superadmin {
		query += ` JOIN project_memberships pm ON pm.project_id=p.id WHERE pm.user_id=$1`
		args = append(args, actor.UserID)
	}
	query += ` GROUP BY p.id,p.key,p.name,p.description,p.archived_at ORDER BY p.key`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []ProjectStatus
	for rows.Next() {
		var project ProjectStatus
		if err = rows.Scan(
			&project.Key, &project.Name, &project.Description, &project.Archived,
			&project.UserStories.Total, &project.UserStories.Done,
			&project.Tickets.Total, &project.Tickets.Done,
			&project.Statuses.Open, &project.Statuses.InProgress,
			&project.Statuses.Review, &project.Statuses.Blocked,
		); err != nil {
			return nil, err
		}
		project.Statuses.Done = project.Tickets.Done
		projects = append(projects, project)
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
	var id, number int64
	err = tx.QueryRowContext(ctx, `UPDATE projects SET next_ticket_number=next_ticket_number+1 WHERE id=$1 RETURNING next_ticket_number-1`, projectID).Scan(&number)
	if err != nil {
		return result, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO tickets(project_id,number,parent_ticket_id,related_ticket_id,feature_id,type,title,description,priority,created_by_credential_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, projectID, number, parentID, relatedID, featureID, in.Type, in.Title, in.Description, priority, actor.CredentialID).Scan(&id)
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
	project, number, _ := domain.ParseReference(ref)
	row := s.DB.QueryRowContext(ctx, ticketSelect+` WHERE p.key=$1 AND t.number=$2`, project, number)
	result, err = scanTicket(row)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	return result, err
}

func (s *Store) ListTickets(ctx context.Context, actor Actor, project string, filter TicketFilter) ([]Ticket, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return nil, err
	}
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, fmt.Errorf("invalid ticket status")
	}
	if filter.Priority != "" && !validPriority(filter.Priority) {
		return nil, fmt.Errorf("invalid ticket priority")
	}
	query := ticketSelect + ` WHERE p.key=$1 AND (t.archived_at IS NOT NULL)=$2`
	arguments := []any{strings.ToUpper(project), filter.Archived}
	nextArgument := func(value any) string {
		arguments = append(arguments, value)
		return fmt.Sprintf("$%d", len(arguments))
	}
	if filter.ParentOnly {
		query += ` AND t.parent_ticket_id IS NULL`
	}
	if filter.FeatureKey != "" {
		query += ` AND f.key=` + nextArgument(filter.FeatureKey)
	}
	if filter.Status != "" {
		query += ` AND t.status=` + nextArgument(filter.Status)
	}
	if filter.Priority != "" {
		query += ` AND t.priority=` + nextArgument(filter.Priority)
	}
	if filter.Label != "" {
		query += ` AND EXISTS (SELECT 1 FROM ticket_labels tl JOIN labels l ON l.id=tl.label_id WHERE tl.ticket_id=t.id AND l.name=` + nextArgument(filter.Label) + `)`
	}
	if filter.Query != "" {
		query += ` AND (t.title ILIKE '%' || ` + nextArgument(filter.Query) + ` || '%' OR t.description ILIKE '%' || ` + nextArgument(filter.Query) + ` || '%')`
	}
	query += ` ORDER BY t.number DESC`
	rows, err := s.DB.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ticket)
	}
	return result, rows.Err()
}

func validStatus(status domain.Status) bool {
	return status == domain.Open || status == domain.InProgress || status == domain.Review || status == domain.Blocked || status == domain.Done
}

func validPriority(priority string) bool {
	return priority == "low" || priority == "normal" || priority == "high" || priority == "urgent"
}

func (s *Store) UpdateTicket(ctx context.Context, actor Actor, ref string, in UpdateTicket) (Ticket, error) {
	var result Ticket
	project, _, err := domain.ParseReference(ref)
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
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return result, err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return result, err
	}
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

// SetTicketArchived removes or restores a ticket from active work. Its creator
// and project administrators can perform the operation.
func (s *Store) SetTicketArchived(ctx context.Context, actor Actor, ref string, archived bool) error {
	project, _, err := domain.ParseReference(ref)
	if err != nil {
		return err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return err
	}
	var creator int64
	var parentRef sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT t.created_by_credential_id,CASE WHEN parent.id IS NULL THEN NULL ELSE p.key || '-' || parent.number::text END FROM tickets t LEFT JOIN tickets parent ON parent.id=t.parent_ticket_id LEFT JOIN projects p ON p.id=parent.project_id WHERE t.id=$1 FOR UPDATE OF t`, id).Scan(&creator, &parentRef)
	if err != nil {
		return err
	}
	if !actor.Superadmin && creator != actor.CredentialID {
		if err = authorize(ctx, tx, actor, project, domain.RoleAdmin); err != nil {
			return err
		}
	}
	if archived {
		_, err = tx.ExecContext(ctx, `UPDATE tickets SET archived_at=COALESCE(archived_at,now()),updated_at=now() WHERE id=$1`, id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE tickets SET archived_at=NULL,updated_at=now() WHERE id=$1`, id)
	}
	if err != nil {
		return err
	}
	action := "ticket_restored"
	if archived {
		action = "ticket_archived"
	}
	if err = writeEvent(ctx, tx, projectID, id, actor.CredentialID, action, map[string]any{"ref": strings.ToUpper(ref)}); err != nil {
		return err
	}
	if parentRef.Valid {
		if err = refreshStory(ctx, tx, parentRef.String, actor.CredentialID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteTicket permanently removes a ticket that is no longer referenced. The
// project audit trail retains a deletion event without retaining ticket data.
func (s *Store) DeleteTicket(ctx context.Context, actor Actor, ref string) error {
	project, _, err := domain.ParseReference(ref)
	if err != nil {
		return err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return err
	}
	var creator int64
	if err = tx.QueryRowContext(ctx, `SELECT created_by_credential_id FROM tickets WHERE id=$1 FOR UPDATE`, id).Scan(&creator); err != nil {
		return err
	}
	if !actor.Superadmin && creator != actor.CredentialID {
		if err = authorize(ctx, tx, actor, project, domain.RoleAdmin); err != nil {
			return err
		}
	}
	var hasDependencies bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets WHERE parent_ticket_id=$1 OR related_ticket_id=$1) OR EXISTS(SELECT 1 FROM client_requests WHERE linked_ticket_id=$1)`, id).Scan(&hasDependencies); err != nil {
		return err
	}
	if hasDependencies {
		return ErrTicketHasDependencies
	}
	for _, table := range []string{"comments", "ticket_revisions", "activity_events"} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE ticket_id=$1`, id); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM tickets WHERE id=$1`, id); err != nil {
		return err
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "ticket_deleted", map[string]any{"ref": strings.ToUpper(ref)}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Revisions(ctx context.Context, actor Actor, ref string) ([]Revision, error) {
	project, _, err := domain.ParseReference(ref)
	if err != nil {
		return nil, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return nil, err
	}
	projectID, err := projectID(ctx, s.DB, project)
	if err != nil {
		return nil, err
	}
	id, _, err := ticketID(ctx, s.DB, ref, projectID)
	if err != nil {
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
	project, _, err := domain.ParseReference(ref)
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
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return out, err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return out, err
	}
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
	project, _, err := domain.ParseReference(ref)
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
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return out, err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return out, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO comments(ticket_id,body,author_credential_id) VALUES($1,$2,$3) RETURNING id,body,created_at`, id, body, actor.CredentialID).Scan(&out.ID, &out.Body, &out.CreatedAt)
	if err != nil {
		return out, err
	}
	if err = writeEvent(ctx, tx, currentProjectID(ctx, tx, id), id, actor.CredentialID, "comment_added", map[string]any{"comment_id": out.ID}); err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func (s *Store) Comments(ctx context.Context, actor Actor, ref string) ([]Comment, error) {
	id, err := s.ticketForRead(ctx, actor, ref)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id,c.body,COALESCE(credential.name,''),c.created_at FROM comments c LEFT JOIN credentials credential ON credential.id=c.author_credential_id WHERE c.ticket_id=$1 ORDER BY c.created_at,c.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		if err = rows.Scan(&comment.ID, &comment.Body, &comment.Author, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (s *Store) Activity(ctx context.Context, actor Actor, ref string) ([]Activity, error) {
	id, err := s.ticketForRead(ctx, actor, ref)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT event.action,event.payload,COALESCE(credential.name,''),event.created_at FROM activity_events event LEFT JOIN credentials credential ON credential.id=event.actor_credential_id WHERE event.ticket_id=$1 ORDER BY event.created_at,event.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activity := []Activity{}
	for rows.Next() {
		var item Activity
		if err = rows.Scan(&item.Action, &item.Payload, &item.Actor, &item.CreatedAt); err != nil {
			return nil, err
		}
		activity = append(activity, item)
	}
	return activity, rows.Err()
}

func (s *Store) ticketForRead(ctx context.Context, actor Actor, ref string) (int64, error) {
	project, _, err := domain.ParseReference(ref)
	if err != nil {
		return 0, err
	}
	if err = s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return 0, err
	}
	projectID, err := projectID(ctx, s.DB, project)
	if err != nil {
		return 0, err
	}
	id, _, err := ticketID(ctx, s.DB, ref, projectID)
	return id, err
}

func (s *Store) Claim(ctx context.Context, actor Actor, ref string) (Ticket, error) {
	var out Ticket
	project, _, err := domain.ParseReference(ref)
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
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return out, err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return out, err
	}
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
	project, _, err := domain.ParseReference(ref)
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
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return err
	}
	id, _, err := ticketID(ctx, tx, ref, projectID)
	if err != nil {
		return err
	}
	var owner sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT claimed_by_credential_id FROM tickets WHERE id=$1 FOR UPDATE`, id).Scan(&owner); err != nil {
		return err
	}
	if !owner.Valid {
		return ErrConflict
	}
	if owner.Int64 != actor.CredentialID && !actor.Superadmin {
		if err = authorize(ctx, tx, actor, project, domain.RoleAdmin); err != nil {
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

func (s *Store) ListProjectMembers(ctx context.Context, actor Actor, project string) ([]ProjectMember, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT u.id,u.name,COALESCE(u.email,''),pm.role,p.owner_user_id=u.id FROM project_memberships pm JOIN projects p ON p.id=pm.project_id JOIN users u ON u.id=pm.user_id WHERE p.key=$1 ORDER BY p.owner_user_id=u.id DESC,u.name,u.id`, strings.ToUpper(project))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []ProjectMember{}
	for rows.Next() {
		var member ProjectMember
		var role string
		if err = rows.Scan(&member.UserID, &member.Name, &member.Email, &role, &member.Owner); err != nil {
			return nil, err
		}
		member.Role = domain.Role(role)
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Store) UpdateProjectMemberRole(ctx context.Context, actor Actor, project string, userID int64, role domain.Role) error {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	if !role.Allows(domain.RoleRead) {
		return fmt.Errorf("invalid member role")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, ownerID, err := projectAdministration(ctx, tx, project)
	if err != nil {
		return err
	}
	if userID == ownerID {
		return fmt.Errorf("the project owner role cannot be changed")
	}
	result, err := tx.ExecContext(ctx, `UPDATE project_memberships SET role=$1 WHERE project_id=$2 AND user_id=$3`, role, projectID, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "member_role_updated", map[string]any{"user_id": userID, "role": role}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RemoveProjectMember(ctx context.Context, actor Actor, project string, userID int64) error {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, ownerID, err := projectAdministration(ctx, tx, project)
	if err != nil {
		return err
	}
	if userID == ownerID {
		return fmt.Errorf("the project owner cannot be removed")
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM project_memberships WHERE project_id=$1 AND user_id=$2`, projectID, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "member_removed", map[string]any{"user_id": userID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListPendingInvitations(ctx context.Context, actor Actor, project string) ([]Invitation, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT i.id,i.role,i.expires_at,i.created_at FROM invitations i JOIN projects p ON p.id=i.project_id WHERE p.key=$1 AND i.accepted_at IS NULL AND i.expires_at>now() ORDER BY i.expires_at,i.id`, strings.ToUpper(project))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invitations := []Invitation{}
	for rows.Next() {
		var invitation Invitation
		var role string
		if err = rows.Scan(&invitation.ID, &role, &invitation.ExpiresAt, &invitation.CreatedAt); err != nil {
			return nil, err
		}
		invitation.Role = domain.Role(role)
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *Store) RevokeInvitation(ctx context.Context, actor Actor, project string, invitationID int64) error {
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID, _, err := projectAdministration(ctx, tx, project)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM invitations WHERE id=$1 AND project_id=$2 AND accepted_at IS NULL AND expires_at>now()`, invitationID, projectID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "invitation_revoked", map[string]any{"invitation_id": invitationID}); err != nil {
		return err
	}
	return tx.Commit()
}

func projectAdministration(ctx context.Context, tx *sql.Tx, project string) (int64, int64, error) {
	var projectID, ownerID int64
	err := tx.QueryRowContext(ctx, `SELECT id,owner_user_id FROM projects WHERE key=$1 FOR UPDATE`, strings.ToUpper(project)).Scan(&projectID, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	return projectID, ownerID, err
}

func (s *Store) authorize(ctx context.Context, actor Actor, project string, required domain.Role) error {
	return authorize(ctx, s.DB, actor, project, required)
}

func authorize(ctx context.Context, queryer rowQuerier, actor Actor, project string, required domain.Role) error {
	if actor.Superadmin {
		return nil
	}
	var role, max string
	err := queryer.QueryRowContext(ctx, `SELECT pm.role,c.max_role FROM credentials c JOIN project_memberships pm ON pm.user_id=c.user_id JOIN projects p ON p.id=pm.project_id WHERE c.id=$1 AND p.key=$2 AND c.disabled_at IS NULL`, actor.CredentialID, strings.ToUpper(project)).Scan(&role, &max)
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

const ticketSelect = `SELECT t.id,t.number,p.key,t.type,CASE WHEN pt.id IS NULL THEN NULL ELSE pp.key || '-' || pt.number::text END,CASE WHEN rt.id IS NULL THEN NULL ELSE rtp.key || '-' || rt.number::text END,f.key,t.title,t.description,t.status,t.priority,t.version, COALESCE((SELECT json_agg(l.name ORDER BY l.name) FROM ticket_labels tl JOIN labels l ON l.id=tl.label_id WHERE tl.ticket_id=t.id),'[]'::json), (SELECT count(*) FROM tickets c WHERE c.parent_ticket_id=t.id AND c.archived_at IS NULL), (SELECT count(*) FROM tickets c WHERE c.parent_ticket_id=t.id AND c.status='done' AND c.archived_at IS NULL), t.client_visibility='published',t.archived_at IS NOT NULL FROM tickets t JOIN projects p ON p.id=t.project_id LEFT JOIN tickets pt ON pt.id=t.parent_ticket_id LEFT JOIN projects pp ON pp.id=pt.project_id LEFT JOIN tickets rt ON rt.id=t.related_ticket_id LEFT JOIN projects rtp ON rtp.id=rt.project_id LEFT JOIN features f ON f.id=t.feature_id`

func scanTicket(row scanner) (Ticket, error) {
	var t Ticket
	var parent, related, feature sql.NullString
	var labelsJSON []byte
	var typ, status string
	err := row.Scan(&t.ID, &t.Number, &t.Project, &typ, &parent, &related, &feature, &t.Title, &t.Description, &status, &t.Priority, &t.Version, &labelsJSON, &t.ChildCount, &t.DoneChildren, &t.Published, &t.Archived)
	if err != nil {
		return t, err
	}
	t.Ref = fmt.Sprintf("%s-%d", t.Project, t.Number)
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
}, ref string, projectID int64) (int64, string, error) {
	project, number, err := domain.ParseReference(ref)
	if err != nil {
		return 0, "", err
	}
	var id int64
	var typ string
	err = q.QueryRowContext(ctx, `SELECT t.id,t.type FROM tickets t JOIN projects p ON p.id=t.project_id WHERE t.project_id=$1 AND p.key=$2 AND t.number=$3`, projectID, project, number).Scan(&id, &typ)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
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
func writeProjectEvent(ctx context.Context, tx *sql.Tx, project, actor int64, action string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_events(project_id,ticket_id,action,payload,actor_credential_id) VALUES($1,NULL,$2,$3,$4)`, project, action, b, actor)
	return err
}
func currentProjectID(ctx context.Context, tx *sql.Tx, id int64) int64 {
	var project int64
	_ = tx.QueryRowContext(ctx, `SELECT project_id FROM tickets WHERE id=$1`, id).Scan(&project)
	return project
}
func refreshStory(ctx context.Context, tx *sql.Tx, parentRef string, actor int64) error {
	project, _, err := domain.ParseReference(parentRef)
	if err != nil {
		return err
	}
	projectID, err := projectID(ctx, tx, project)
	if err != nil {
		return err
	}
	id, _, err := ticketID(ctx, tx, parentRef, projectID)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT status FROM tickets WHERE parent_ticket_id=$1 AND archived_at IS NULL`, id)
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
