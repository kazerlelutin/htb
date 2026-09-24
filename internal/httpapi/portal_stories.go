package httpapi

import (
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/kazerlelutin/htb/internal/store"
)

type portalStoryRow struct {
	store.ClientStory
	StatusLabel string
	Percent     int
}

type portalStoryCommentView struct {
	Author    string
	Body      template.HTML
	CreatedAt time.Time
}

type portalStoryView struct {
	Ref, Project, Title, StatusLabel string
	Description                      template.HTML
	ChildCount, DoneChildren         int
	Percent                          int
	Comments                         []portalStoryCommentView
	CSRF, Error, DraftComment        string
}

var portalStoryTemplate = template.Must(template.New("portal-story").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a><header class="site-header"><a class="wordmark" href="/portal">&gt;_ HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p><a href="/portal/projects/{{.Project}}">← Retour au projet</a></p><p class="eyebrow">USER STORY {{.Ref}} · {{.StatusLabel}}</p><h1>{{.Title}}</h1>
<section aria-labelledby="progress-title"><h2 id="progress-title">Avancement</h2>{{if .ChildCount}}<label for="story-progress">Tâches techniques terminées : {{.DoneChildren}} / {{.ChildCount}} ({{.Percent}} %)</label><progress id="story-progress" value="{{.DoneChildren}}" max="{{.ChildCount}}">{{.Percent}} %</progress>{{else}}<p>Cette US n’a pas encore de tâche technique liée.</p>{{end}}</section>
<section aria-labelledby="description-title"><h2 id="description-title">Description</h2><div class="portal-markdown">{{.Description}}</div></section>
<section aria-labelledby="conversation-title"><h2 id="conversation-title">Conversation</h2>{{if .Comments}}<ol class="portal-comments">{{range .Comments}}<li><strong>{{.Author}}</strong> <time>{{.CreatedAt.Format "02/01/2006 15:04"}}</time><div class="portal-markdown">{{.Body}}</div></li>{{end}}</ol>{{else}}<p>Aucun commentaire pour le moment.</p>{{end}}</section>
<section aria-labelledby="comment-title"><h2 id="comment-title">Ajouter un commentaire</h2>{{if .Error}}<p class="portal-error" role="alert">{{.Error}}</p>{{end}}<form class="portal-form" action="/portal/stories/{{.Ref}}/comments" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="comment-body">Votre message (Markdown accepté)</label><textarea id="comment-body" name="body" maxlength="20000" rows="5" required>{{.DraftComment}}</textarea><button class="button" type="submit">Publier le commentaire</button></form></section>
</main></body></html>`))

func storyStatusLabel(status string) string {
	switch status {
	case "open":
		return "À faire"
	case "in_progress":
		return "En cours"
	case "review":
		return "En revue"
	case "blocked":
		return "Bloquée"
	case "done":
		return "Terminée"
	default:
		return "État inconnu"
	}
}

func progressPercent(done, total int) int {
	if total <= 0 {
		return 0
	}
	return done * 100 / total
}

func (s *Server) storyView(r *http.Request, ref string) (portalStoryView, error) {
	story, err := s.stories.GetClientStory(r.Context(), actor(r), ref)
	if err != nil {
		return portalStoryView{}, err
	}
	comments, err := s.stories.ListClientStoryComments(r.Context(), actor(r), ref)
	if err != nil {
		return portalStoryView{}, err
	}
	view := portalStoryView{
		Ref: story.Ref, Project: story.Project, Title: story.Title,
		StatusLabel: storyStatusLabel(story.Status), Description: renderPortalMarkdown(story.Description),
		ChildCount: story.ChildCount, DoneChildren: story.DoneChildren, Percent: progressPercent(story.DoneChildren, story.ChildCount), CSRF: s.portalCSRF(r),
	}
	for _, comment := range comments {
		view.Comments = append(view.Comments, portalStoryCommentView{Author: comment.Author, Body: renderPortalMarkdown(comment.Body), CreatedAt: comment.CreatedAt})
	}
	return view, nil
}

func (s *Server) portalStory(w http.ResponseWriter, r *http.Request) {
	view, err := s.storyView(r, r.PathValue("ref"))
	if err != nil {
		s.portalReadError(w, err)
		return
	}
	s.portalHeaders(w)
	if err := portalStoryTemplate.Execute(w, view); err != nil {
		s.log.Error("render client story", "error", err)
	}
}

func (s *Server) portalAddStoryComment(w http.ResponseWriter, r *http.Request) {
	if !s.validPortalForm(w, r) {
		return
	}
	ref := r.PathValue("ref")
	body := r.PostForm.Get("body")
	_, err := s.stories.AddClientStoryComment(r.Context(), actor(r), ref, body)
	if err == nil {
		http.Redirect(w, r, "/portal/stories/"+strings.ToUpper(ref), http.StatusSeeOther)
		return
	}
	if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
		s.portalReadError(w, err)
		return
	}
	if !errors.Is(err, store.ErrInvalidClientRequest) {
		s.portalReadError(w, err)
		return
	}
	view, readErr := s.storyView(r, ref)
	if readErr != nil {
		s.portalReadError(w, readErr)
		return
	}
	view.DraftComment, view.Error = body, "Vérifiez votre commentaire, puis réessayez."
	s.portalHeaders(w)
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := portalStoryTemplate.Execute(w, view); err != nil {
		s.log.Error("render client story", "error", err)
	}
}
