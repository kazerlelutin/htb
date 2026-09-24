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

// ClientStory is a deliberate public projection. Internal ticket fields and
// technical task content must never be sent to the browser portal.
type ClientStory struct {
	Ref          string `json:"ref"`
	Project      string `json:"project"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Visibility   string `json:"visibility"`
	ChildCount   int    `json:"child_count"`
	DoneChildren int    `json:"done_children"`
	Published    bool   `json:"published"`
}

type ClientStoryPage struct {
	Stories                         []ClientStory
	Page, Total, StoryDone          int
	TaskCount, TaskDone, TotalPages int
}

const (
	ClientStoryDraft     = "draft"
	ClientStoryPublished = "published"
)

type ClientStoryComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

const clientStorySelect = `SELECT p.key || '-' || t.number::text,p.key,t.title,t.description,t.status,t.client_visibility,
	(SELECT count(*) FROM tickets child WHERE child.parent_ticket_id=t.id AND child.type='technical_task'),
	(SELECT count(*) FROM tickets child WHERE child.parent_ticket_id=t.id AND child.type='technical_task' AND child.status='done'),
	FROM tickets t JOIN projects p ON p.id=t.project_id WHERE t.type='user_story'`

func scanClientStory(row scanner) (ClientStory, error) {
	var story ClientStory
	err := row.Scan(&story.Ref, &story.Project, &story.Title, &story.Description, &story.Status, &story.Visibility, &story.ChildCount, &story.DoneChildren)
	story.Published = story.Visibility == ClientStoryPublished
	return story, err
}

// Project administrators can preview drafts. Other members only see stories
// explicitly published to the client portal.
func (s *Store) canPreviewClientStories(ctx context.Context, actor Actor, project string) (bool, error) {
	err := s.authorize(ctx, actor, project, domain.RoleAdmin)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, ErrForbidden) {
		return false, err
	}
	return false, s.authorize(ctx, actor, project, domain.RoleRead)
}

func (s *Store) ListClientStories(ctx context.Context, actor Actor, project string) ([]ClientStory, error) {
	preview, err := s.canPreviewClientStories(ctx, actor, project)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, clientStorySelect+` AND p.key=$1 AND ($2 OR t.client_visibility='published') ORDER BY (t.status='done'),t.number DESC`, strings.ToUpper(project), preview)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stories := []ClientStory{}
	for rows.Next() {
		story, err := scanClientStory(rows)
		if err != nil {
			return nil, err
		}
		stories = append(stories, story)
	}
	return stories, rows.Err()
}

// ListClientStoriesPage returns a bounded, authorized page and summary values
// calculated across every story visible to the current actor.
func (s *Store) ListClientStoriesPage(ctx context.Context, actor Actor, project string, page, perPage int) (ClientStoryPage, error) {
	if page < 1 || perPage < 1 || perPage > 100 {
		return ClientStoryPage{}, fmt.Errorf("invalid client story page")
	}
	preview, err := s.canPreviewClientStories(ctx, actor, project)
	if err != nil {
		return ClientStoryPage{}, err
	}
	project = strings.ToUpper(project)
	result := ClientStoryPage{Page: page}
	err = s.DB.QueryRowContext(ctx, `SELECT
		count(*),
		count(*) FILTER (WHERE t.status='done'),
		COALESCE(sum((SELECT count(*) FROM tickets child WHERE child.parent_ticket_id=t.id AND child.type='technical_task')), 0),
		COALESCE(sum((SELECT count(*) FROM tickets child WHERE child.parent_ticket_id=t.id AND child.type='technical_task' AND child.status='done')), 0)
		FROM tickets t JOIN projects p ON p.id=t.project_id
		WHERE t.type='user_story' AND p.key=$1 AND ($2 OR t.client_visibility='published')`, project, preview).Scan(&result.Total, &result.StoryDone, &result.TaskCount, &result.TaskDone)
	if err != nil {
		return ClientStoryPage{}, err
	}
	result.TotalPages = (result.Total + perPage - 1) / perPage
	if result.TotalPages == 0 {
		result.TotalPages = 1
	}
	if result.Page > result.TotalPages {
		result.Page = result.TotalPages
	}
	offset := (result.Page - 1) * perPage
	rows, err := s.DB.QueryContext(ctx, clientStorySelect+` AND p.key=$1 AND ($2 OR t.client_visibility='published') ORDER BY (t.status='done'),t.number DESC LIMIT $3 OFFSET $4`, project, preview, perPage, offset)
	if err != nil {
		return ClientStoryPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		story, err := scanClientStory(rows)
		if err != nil {
			return ClientStoryPage{}, err
		}
		result.Stories = append(result.Stories, story)
	}
	return result, rows.Err()
}

func (s *Store) GetClientStory(ctx context.Context, actor Actor, ref string) (ClientStory, error) {
	project, number, err := domain.ParseReference(ref)
	if err != nil {
		return ClientStory{}, ErrNotFound
	}
	preview, err := s.canPreviewClientStories(ctx, actor, project)
	if err != nil {
		return ClientStory{}, err
	}
	story, err := scanClientStory(s.DB.QueryRowContext(ctx, clientStorySelect+` AND p.key=$1 AND t.number=$2 AND ($3 OR t.client_visibility='published')`, project, number, preview))
	if errors.Is(err, sql.ErrNoRows) {
		return ClientStory{}, ErrNotFound
	}
	return story, err
}

// SetClientStoryPublished is restricted to project administrators. Existing
// stories are private until the team explicitly publishes them.
func (s *Store) SetClientStoryPublished(ctx context.Context, actor Actor, ref string, published bool) error {
	project, number, err := domain.ParseReference(ref)
	if err != nil {
		return ErrNotFound
	}
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ticketID, projectID int64
	visibility := ClientStoryDraft
	if published {
		visibility = ClientStoryPublished
	}
	err = tx.QueryRowContext(ctx, `UPDATE tickets t SET client_visibility=$3,client_published_at=CASE WHEN $3='published' THEN COALESCE(t.client_published_at,now()) ELSE NULL END
		FROM projects p WHERE p.id=t.project_id AND p.key=$1 AND t.number=$2 AND t.type='user_story'
		RETURNING t.id,p.id`, project, number, visibility).Scan(&ticketID, &projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	action := "client_story_unpublished"
	if published {
		action = "client_story_published"
	}
	if err = writeEvent(ctx, tx, projectID, ticketID, actor.CredentialID, action, map[string]any{"ref": strings.ToUpper(ref)}); err != nil {
		return err
	}
	return tx.Commit()
}

// ListInternalStoryComments returns the ticket discussion to project
// administrators. It is intentionally separate from the client conversation.
func (s *Store) ListInternalStoryComments(ctx context.Context, actor Actor, ref string) ([]Comment, error) {
	if err := s.authorizeInternalStoryComments(ctx, actor, ref); err != nil {
		return nil, err
	}
	return s.Comments(ctx, actor, ref)
}

// AddInternalStoryComment adds a private ticket comment for a project
// administrator, including when the story is still a draft.
func (s *Store) AddInternalStoryComment(ctx context.Context, actor Actor, ref, body string) (Comment, error) {
	if err := s.authorizeInternalStoryComments(ctx, actor, ref); err != nil {
		return Comment{}, err
	}
	return s.AddComment(ctx, actor, ref, body)
}

func (s *Store) authorizeInternalStoryComments(ctx context.Context, actor Actor, ref string) error {
	project, number, err := domain.ParseReference(ref)
	if err != nil {
		return ErrNotFound
	}
	if err := s.authorize(ctx, actor, project, domain.RoleAdmin); err != nil {
		return err
	}
	var exists bool
	err = s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tickets t JOIN projects p ON p.id=t.project_id WHERE p.key=$1 AND t.number=$2 AND t.type='user_story')`, project, number).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListClientStoryComments(ctx context.Context, actor Actor, ref string) ([]ClientStoryComment, error) {
	story, err := s.GetClientStory(ctx, actor, ref)
	if err != nil {
		return nil, err
	}
	if !story.Published {
		return nil, ErrNotFound
	}
	project, number, _ := domain.ParseReference(ref)
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id,c.body,COALESCE(a.name,''),c.created_at
		FROM client_story_comments c JOIN tickets t ON t.id=c.ticket_id JOIN projects p ON p.id=t.project_id
		JOIN credentials a ON a.id=c.author_credential_id
		WHERE p.key=$1 AND t.number=$2 AND t.type='user_story' AND t.client_visibility='published'
		ORDER BY c.created_at,c.id`, project, number)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []ClientStoryComment{}
	for rows.Next() {
		var comment ClientStoryComment
		if err = rows.Scan(&comment.ID, &comment.Body, &comment.Author, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (s *Store) AddClientStoryComment(ctx context.Context, actor Actor, ref, body string) (ClientStoryComment, error) {
	var comment ClientStoryComment
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || len([]rune(trimmed)) > 20000 {
		return comment, fmt.Errorf("%w: comment must contain 1 to 20000 characters", ErrInvalidClientRequest)
	}
	project, number, err := domain.ParseReference(ref)
	if err != nil {
		return comment, ErrNotFound
	}
	if err := s.authorize(ctx, actor, project, domain.RoleRead); err != nil {
		return comment, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return comment, err
	}
	defer tx.Rollback()
	var ticketID, projectID int64
	err = tx.QueryRowContext(ctx, `SELECT t.id,p.id FROM tickets t JOIN projects p ON p.id=t.project_id
		WHERE p.key=$1 AND t.number=$2 AND t.type='user_story' AND t.client_visibility='published' FOR UPDATE OF t`, project, number).Scan(&ticketID, &projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return comment, ErrNotFound
	}
	if err != nil {
		return comment, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT name FROM credentials WHERE id=$1 AND disabled_at IS NULL`, actor.CredentialID).Scan(&comment.Author); err != nil {
		return comment, err
	}
	comment.Body = trimmed
	if err = tx.QueryRowContext(ctx, `INSERT INTO client_story_comments(ticket_id,body,author_credential_id) VALUES($1,$2,$3) RETURNING id,created_at`, ticketID, comment.Body, actor.CredentialID).Scan(&comment.ID, &comment.CreatedAt); err != nil {
		return comment, err
	}
	if err = writeEvent(ctx, tx, projectID, ticketID, actor.CredentialID, "client_story_commented", map[string]any{"comment_id": comment.ID}); err != nil {
		return comment, err
	}
	return comment, tx.Commit()
}
