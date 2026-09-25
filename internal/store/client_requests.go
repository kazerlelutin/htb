package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kazerlelutin/htb/internal/domain"
)

type ClientRequest struct {
	ID              int64     `json:"id"`
	Project         string    `json:"project"`
	Title           string    `json:"title"`
	Body            string    `json:"body"`
	Status          string    `json:"status"`
	LinkedTicketRef *string   `json:"linked_ticket_ref,omitempty"`
	SubmittedBy     string    `json:"submitted_by"`
	CreatedAt       time.Time `json:"created_at"`
}

type ClientRequestComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type ClientRequestUpdate struct {
	Status          *string `json:"status"`
	LinkedTicketRef *string `json:"linked_ticket_ref"`
}

var ErrInvalidClientRequest = errors.New("invalid client request")

const clientRequestSelect = `SELECT r.id,p.key,r.title,r.body,r.status,CASE WHEN t.id IS NULL THEN NULL ELSE p.key || '-' || t.number::text END,COALESCE(c.name,''),r.created_at FROM client_requests r JOIN projects p ON p.id=r.project_id JOIN credentials c ON c.id=r.submitted_by_credential_id LEFT JOIN tickets t ON t.id=r.linked_ticket_id`

func scanClientRequest(row scanner) (ClientRequest, error) {
	var item ClientRequest
	var linked sql.NullString
	err := row.Scan(&item.ID, &item.Project, &item.Title, &item.Body, &item.Status, &linked, &item.SubmittedBy, &item.CreatedAt)
	if linked.Valid {
		item.LinkedTicketRef = &linked.String
	}
	return item, err
}

func validClientRequestText(title, body string) error {
	if len(strings.TrimSpace(title)) == 0 || len([]rune(title)) > 240 {
		return fmt.Errorf("%w: title must contain 1 to 240 characters", ErrInvalidClientRequest)
	}
	if len(strings.TrimSpace(body)) == 0 || len([]rune(body)) > 20000 {
		return fmt.Errorf("%w: description must contain 1 to 20000 characters", ErrInvalidClientRequest)
	}
	return nil
}

func (s *Store) CreateClientRequest(ctx context.Context, actor Actor, project, title, body string) (ClientRequest, error) {
	var item ClientRequest
	if err := validClientRequestText(title, body); err != nil {
		return item, err
	}
	if err := s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return item, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	var projectID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE key=$1 AND archived_at IS NULL`, strings.ToUpper(project)).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrForbidden
	}
	if err != nil {
		return item, err
	}
	var id int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO client_requests(project_id,title,body,submitted_by_credential_id) VALUES($1,$2,$3,$4) RETURNING id`, projectID, strings.TrimSpace(title), strings.TrimSpace(body), actor.CredentialID).Scan(&id); err != nil {
		return item, err
	}
	item, err = scanClientRequest(tx.QueryRowContext(ctx, clientRequestSelect+` WHERE r.id=$1`, id))
	if err != nil {
		return item, err
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "client_request_created", map[string]any{"request_id": id}); err != nil {
		return item, err
	}
	return item, tx.Commit()
}

func (s *Store) ListClientRequests(ctx context.Context, actor Actor, project string) ([]ClientRequest, error) {
	if err := s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, clientRequestSelect+` WHERE p.key=$1 ORDER BY r.created_at DESC,r.id DESC`, strings.ToUpper(project))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ClientRequest{}
	for rows.Next() {
		item, err := scanClientRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetClientRequest(ctx context.Context, actor Actor, id int64) (ClientRequest, error) {
	var item ClientRequest
	if id < 1 {
		return item, ErrNotFound
	}
	err := s.DB.QueryRowContext(ctx, `SELECT p.key FROM client_requests r JOIN projects p ON p.id=r.project_id WHERE r.id=$1`, id).Scan(&item.Project)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	if err = s.authorize(ctx, actor, item.Project, domain.RoleRead); err != nil {
		return ClientRequest{}, err
	}
	item, err = scanClientRequest(s.DB.QueryRowContext(ctx, clientRequestSelect+` WHERE r.id=$1`, id))
	return item, err
}

func (s *Store) ListClientRequestComments(ctx context.Context, actor Actor, id int64) ([]ClientRequestComment, error) {
	if _, err := s.GetClientRequest(ctx, actor, id); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id,c.body,COALESCE(a.name,''),c.created_at FROM client_request_comments c JOIN credentials a ON a.id=c.author_credential_id WHERE c.request_id=$1 ORDER BY c.created_at,c.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []ClientRequestComment{}
	for rows.Next() {
		var comment ClientRequestComment
		if err = rows.Scan(&comment.ID, &comment.Body, &comment.Author, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (s *Store) AddClientRequestComment(ctx context.Context, actor Actor, id int64, body string) (ClientRequestComment, error) {
	var comment ClientRequestComment
	if len(strings.TrimSpace(body)) == 0 || len([]rune(body)) > 20000 {
		return comment, fmt.Errorf("%w: comment must contain 1 to 20000 characters", ErrInvalidClientRequest)
	}
	item, err := s.GetClientRequest(ctx, actor, id)
	if err != nil {
		return comment, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return comment, err
	}
	defer tx.Rollback()
	comment.Body = strings.TrimSpace(body)
	if err = tx.QueryRowContext(ctx, `SELECT name FROM credentials WHERE id=$1`, actor.CredentialID).Scan(&comment.Author); err != nil {
		return comment, err
	}
	if err = tx.QueryRowContext(ctx, `INSERT INTO client_request_comments(request_id,body,author_credential_id) VALUES($1,$2,$3) RETURNING id,created_at`, id, comment.Body, actor.CredentialID).Scan(&comment.ID, &comment.CreatedAt); err != nil {
		return comment, err
	}
	projectID, err := projectID(ctx, tx, item.Project)
	if err != nil {
		return comment, err
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "client_request_commented", map[string]any{"request_id": id, "comment_id": comment.ID}); err != nil {
		return comment, err
	}
	return comment, tx.Commit()
}

func (s *Store) UpdateClientRequest(ctx context.Context, actor Actor, id int64, update ClientRequestUpdate) (ClientRequest, error) {
	item, err := s.GetClientRequest(ctx, actor, id)
	if err != nil {
		return item, err
	}
	if err = s.authorize(ctx, actor, item.Project, domain.RoleWrite); err != nil {
		return item, err
	}
	if update.Status == nil && update.LinkedTicketRef == nil {
		return item, fmt.Errorf("%w: status or linked_ticket_ref is required", ErrInvalidClientRequest)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	projectID, err := projectID(ctx, tx, item.Project)
	if err != nil {
		return item, err
	}
	var status string
	var linked sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT status,linked_ticket_id FROM client_requests WHERE id=$1 FOR UPDATE`, id).Scan(&status, &linked); err != nil {
		return item, err
	}
	if update.Status != nil {
		status = *update.Status
	}
	if status != "received" && status != "in_progress" && status != "needs_info" && status != "done" && status != "rejected" {
		return item, fmt.Errorf("%w: invalid status", ErrInvalidClientRequest)
	}
	if update.LinkedTicketRef != nil {
		linked = sql.NullInt64{}
		if *update.LinkedTicketRef != "" {
			linked.Int64, _, err = ticketID(ctx, tx, *update.LinkedTicketRef, projectID)
			if err != nil {
				return item, err
			}
			linked.Valid = true
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE client_requests SET status=$1,linked_ticket_id=$2,updated_at=now() WHERE id=$3`, status, linked, id); err != nil {
		return item, err
	}
	item, err = scanClientRequest(tx.QueryRowContext(ctx, clientRequestSelect+` WHERE r.id=$1`, id))
	if err != nil {
		return item, err
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "client_request_updated", map[string]any{"request_id": id, "status": status, "linked_ticket_ref": item.LinkedTicketRef}); err != nil {
		return item, err
	}
	return item, tx.Commit()
}

// DeleteClientRequest lets its submitter withdraw a request before the team
// starts work, or remove one that was rejected. Linked comments are deleted by
// the database foreign-key constraint.
func (s *Store) DeleteClientRequest(ctx context.Context, actor Actor, id int64) error {
	if id < 1 {
		return ErrNotFound
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var projectID int64
	err = tx.QueryRowContext(ctx, `DELETE FROM client_requests WHERE id=$1 AND submitted_by_credential_id=$2 AND status IN ('received','rejected') RETURNING project_id`, id, actor.CredentialID).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if err = writeProjectEvent(ctx, tx, projectID, actor.CredentialID, "client_request_deleted", map[string]any{"request_id": id}); err != nil {
		return err
	}
	return tx.Commit()
}
