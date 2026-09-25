package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/kazerlelutin/htb/internal/domain"
	"github.com/kazerlelutin/htb/internal/store"
)

type portalRequestRow struct {
	ID          int64
	Title       string
	StatusLabel string
	CreatedAt   time.Time
}

type portalProjectView struct {
	Name, Key, CSRF, Error, Title, Description string
	Requests                                   []portalRequestRow
	Stories                                    []portalStoryRow
	StoryCount, StoryDone, TaskCount, TaskDone int
	StoryPercent, TaskPercent                  int
	StoryPage, StoryPages                      int
	StoryPrevious, StoryNext                   string
}

type portalCommentView struct {
	Author    string
	Body      template.HTML
	CreatedAt time.Time
}

type portalRequestView struct {
	ID                                 int64
	Project, Title, StatusLabel        string
	Body                               template.HTML
	Comments                           []portalCommentView
	CSRF, Error, DraftComment          string
	RelatedStoryRef, RelatedStoryTitle string
}

var portalProjectTemplate = template.Must(template.New("portal-project").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Name}} — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a><header class="site-header"><a class="wordmark" href="/portal"><span aria-hidden="true">&gt;</span><span class="cursor" aria-hidden="true">_</span> HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p><a href="/portal">← Mes projets</a></p><h1>{{.Name}}</h1>
<section aria-labelledby="progress-title"><h2 id="progress-title">Avancement du projet</h2>{{if .StoryCount}}<label for="stories-progress">US terminées : {{.StoryDone}} / {{.StoryCount}} ({{.StoryPercent}} %)</label><progress id="stories-progress" value="{{.StoryDone}}" max="{{.StoryCount}}">{{.StoryPercent}} %</progress>{{if .TaskCount}}<label for="tasks-progress">Tâches terminées : {{.TaskDone}} / {{.TaskCount}} ({{.TaskPercent}} %)</label><progress id="tasks-progress" value="{{.TaskDone}}" max="{{.TaskCount}}">{{.TaskPercent}} %</progress>{{else}}<p>Les US visibles n’ont pas encore de tâches liées.</p>{{end}}{{else}}<p>Aucune US visible pour le moment.</p>{{end}}</section>
<section aria-labelledby="stories-title"><h2 id="stories-title">User stories</h2>{{if .Stories}}<ul class="portal-requests">{{range .Stories}}<li><a href="/portal/stories/{{.Ref}}"><strong>{{.Title}}</strong><span>{{.Ref}}&nbsp;·&nbsp;{{.StatusLabel}}{{if not .Published}}&nbsp;·&nbsp;Brouillon{{end}}</span></a>{{if .ChildCount}}<label for="story-{{.Ref}}">Tâches terminées : {{.DoneChildren}} / {{.ChildCount}} ({{.Percent}} %)</label><progress id="story-{{.Ref}}" value="{{.DoneChildren}}" max="{{.ChildCount}}">{{.Percent}} %</progress>{{else}}<span>Aucune tâche liée</span>{{end}}</li>{{end}}</ul>{{if gt .StoryPages 1}}<nav class="portal-pagination" aria-label="Pagination des user stories">{{if .StoryPrevious}}<a href="{{.StoryPrevious}}" rel="prev">← Précédente</a>{{end}}<span aria-current="page">Page {{.StoryPage}} sur {{.StoryPages}}</span>{{if .StoryNext}}<a href="{{.StoryNext}}" rel="next">Suivante →</a>{{end}}</nav>{{end}}{{else}}<p>Aucune US visible pour ce projet.</p>{{end}}</section>
<section aria-labelledby="requests-title"><h2 id="requests-title">Demandes proposées</h2>{{if .Requests}}<ul class="portal-requests">{{range .Requests}}<li><a href="/portal/requests/{{.ID}}"><strong>{{.Title}}</strong><span>{{.StatusLabel}}</span></a><form class="portal-request-delete" action="/portal/requests/{{.ID}}/delete" method="post"><input type="hidden" name="csrf" value="{{$.CSRF}}"><button class="link-button" type="submit">Supprimer <span class="visually-hidden">la demande « {{.Title}} »</span></button></form></li>{{end}}</ul>{{else}}<p>Aucune demande en attente ou rejetée.</p>{{end}}</section>
<section aria-labelledby="new-request-title"><h2 id="new-request-title">Proposer une demande</h2><p>Décrivez votre besoin en quelques mots. Vous pourrez suivre la conversation ici.</p>
{{if .Error}}<p class="portal-error" role="alert">{{.Error}}</p>{{end}}
<form class="portal-form" action="/portal/projects/{{.Key}}/requests" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="request-title">Sujet</label><input id="request-title" name="title" maxlength="240" required value="{{.Title}}"><label for="request-body">Description (Markdown accepté)</label><textarea id="request-body" name="body" maxlength="20000" rows="7" required>{{.Description}}</textarea><button class="button" type="submit">Envoyer la demande</button></form></section>
</main></body></html>`))

var portalRequestTemplate = template.Must(template.New("portal-request").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a><header class="site-header"><a class="wordmark" href="/portal"><span aria-hidden="true">&gt;</span><span class="cursor" aria-hidden="true">_</span> HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p><a href="/portal/projects/{{.Project}}">← Retour au projet</a></p><p class="eyebrow">DEMANDE #{{.ID}} · {{.StatusLabel}}</p><h1>{{.Title}}</h1><div class="portal-markdown">{{.Body}}</div>{{if .RelatedStoryRef}}<p>Cette demande est suivie dans l’<a href="/portal/stories/{{.RelatedStoryRef}}">US {{.RelatedStoryRef}} — {{.RelatedStoryTitle}}</a>.</p>{{end}}
<section aria-labelledby="conversation-title"><h2 id="conversation-title">Conversation</h2>{{if .Comments}}<ol class="portal-comments">{{range .Comments}}<li><strong>{{.Author}}</strong> <time>{{.CreatedAt.Format "02/01/2006 15:04"}}</time><div class="portal-markdown">{{.Body}}</div></li>{{end}}</ol>{{else}}<p>Aucun commentaire pour le moment.</p>{{end}}</section>
<section aria-labelledby="comment-title"><h2 id="comment-title">Ajouter un commentaire</h2>{{if .Error}}<p class="portal-error" role="alert">{{.Error}}</p>{{end}}<form class="portal-form" action="/portal/requests/{{.ID}}/comments" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="comment-body">Votre message (Markdown accepté)</label><textarea id="comment-body" name="body" maxlength="20000" rows="5" required>{{.DraftComment}}</textarea><button class="button" type="submit">Publier le commentaire</button></form></section>
</main></body></html>`))

func (s *Server) portalCSRF(r *http.Request) string {
	token := r.Context().Value(browserSessionTokenKey{}).(string)
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write([]byte("portal-form:" + token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) validPortalForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formulaire invalide ou trop long.", http.StatusBadRequest)
		return false
	}
	provided, err := base64.RawURLEncoding.DecodeString(r.PostForm.Get("csrf"))
	if err != nil {
		http.Error(w, "Session de formulaire expirée. Rechargez la page.", http.StatusForbidden)
		return false
	}
	expected, _ := base64.RawURLEncoding.DecodeString(s.portalCSRF(r))
	if !hmac.Equal(provided, expected) {
		http.Error(w, "Session de formulaire expirée. Rechargez la page.", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) portalHeaders(w http.ResponseWriter) {
	s.publicHeaders(w)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

const portalStoriesPerPage = 20

func requestedStoryPage(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func (s *Server) projectView(r *http.Request, key string) (portalProjectView, error) {
	view := portalProjectView{Key: key, CSRF: s.portalCSRF(r)}
	projects, err := s.sessions.ListProjects(r.Context(), actor(r))
	if err != nil {
		return view, err
	}
	allowed := false
	for _, project := range projects {
		if project.Key == key {
			view.Name, allowed = project.Name, true
			break
		}
	}
	if !allowed {
		return view, store.ErrForbidden
	}
	items, err := s.requests.ListClientRequests(r.Context(), actor(r), key)
	if err != nil {
		return view, err
	}
	storyPage, err := s.stories.ListClientStoriesPage(r.Context(), actor(r), key, requestedStoryPage(r), portalStoriesPerPage)
	if err != nil {
		return view, err
	}
	view.StoryPage, view.StoryPages = storyPage.Page, storyPage.TotalPages
	view.StoryCount, view.StoryDone = storyPage.Total, storyPage.StoryDone
	view.TaskCount, view.TaskDone = storyPage.TaskCount, storyPage.TaskDone
	if storyPage.Page > 1 {
		view.StoryPrevious = "/portal/projects/" + key + "?page=" + strconv.Itoa(storyPage.Page-1)
	}
	if storyPage.Page < storyPage.TotalPages {
		view.StoryNext = "/portal/projects/" + key + "?page=" + strconv.Itoa(storyPage.Page+1)
	}
	for _, story := range storyPage.Stories {
		view.Stories = append(view.Stories, portalStoryRow{ClientStory: story, StatusLabel: storyStatusLabel(story.Status), Percent: progressPercent(story.DoneChildren, story.ChildCount)})
	}
	view.StoryPercent = progressPercent(view.StoryDone, view.StoryCount)
	view.TaskPercent = progressPercent(view.TaskDone, view.TaskCount)
	for _, item := range items {
		if item.Status == "received" || item.Status == "rejected" {
			view.Requests = append(view.Requests, portalRequestRow{ID: item.ID, Title: item.Title, StatusLabel: clientStatusLabel(item.Status), CreatedAt: item.CreatedAt})
		}
	}
	return view, nil
}

func (s *Server) portalProject(w http.ResponseWriter, r *http.Request) {
	key, err := domain.NormalizeProjectKey(r.PathValue("project"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	view, err := s.projectView(r, key)
	if err != nil {
		s.portalReadError(w, err)
		return
	}
	s.portalHeaders(w)
	_ = portalProjectTemplate.Execute(w, view)
}

func (s *Server) portalCreateRequest(w http.ResponseWriter, r *http.Request) {
	if !s.validPortalForm(w, r) {
		return
	}
	key, err := domain.NormalizeProjectKey(r.PathValue("project"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	title, body := r.PostForm.Get("title"), r.PostForm.Get("body")
	item, err := s.requests.CreateClientRequest(r.Context(), actor(r), key, title, body)
	if err == nil {
		http.Redirect(w, r, "/portal/requests/"+strconv.FormatInt(item.ID, 10), http.StatusSeeOther)
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		http.Error(w, "Accès au projet refusé.", http.StatusForbidden)
		return
	}
	if !errors.Is(err, store.ErrInvalidClientRequest) {
		s.portalReadError(w, err)
		return
	}
	view, readErr := s.projectView(r, key)
	if readErr != nil {
		s.portalReadError(w, readErr)
		return
	}
	view.Title, view.Description, view.Error = title, body, "Vérifiez le sujet et la description, puis réessayez."
	s.portalHeaders(w)
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = portalProjectTemplate.Execute(w, view)
}

func (s *Server) requestView(r *http.Request, id int64) (portalRequestView, error) {
	view := portalRequestView{CSRF: s.portalCSRF(r)}
	item, err := s.requests.GetClientRequest(r.Context(), actor(r), id)
	if err != nil {
		return view, err
	}
	comments, err := s.requests.ListClientRequestComments(r.Context(), actor(r), id)
	if err != nil {
		return view, err
	}
	view.ID, view.Project, view.Title, view.StatusLabel, view.Body = item.ID, item.Project, item.Title, clientStatusLabel(item.Status), renderPortalMarkdown(item.Body)
	if item.LinkedTicketRef != nil {
		story, storyErr := s.stories.GetClientStory(r.Context(), actor(r), *item.LinkedTicketRef)
		switch {
		case storyErr == nil:
			view.RelatedStoryRef, view.RelatedStoryTitle = story.Ref, story.Title
		case errors.Is(storyErr, store.ErrNotFound):
			// A technical or unpublished ticket remains private.
		default:
			return view, storyErr
		}
	}
	for _, comment := range comments {
		view.Comments = append(view.Comments, portalCommentView{Author: comment.Author, Body: renderPortalMarkdown(comment.Body), CreatedAt: comment.CreatedAt})
	}
	return view, nil
}

func (s *Server) portalRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	view, err := s.requestView(r, id)
	if err != nil {
		s.portalReadError(w, err)
		return
	}
	s.portalHeaders(w)
	_ = portalRequestTemplate.Execute(w, view)
}

func (s *Server) portalAddComment(w http.ResponseWriter, r *http.Request) {
	if !s.validPortalForm(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	body := r.PostForm.Get("body")
	_, err = s.requests.AddClientRequestComment(r.Context(), actor(r), id, body)
	if err == nil {
		http.Redirect(w, r, "/portal/requests/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
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
	view, readErr := s.requestView(r, id)
	if readErr != nil {
		s.portalReadError(w, readErr)
		return
	}
	view.DraftComment, view.Error = body, "Vérifiez votre commentaire, puis réessayez."
	s.portalHeaders(w)
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = portalRequestTemplate.Execute(w, view)
}

func (s *Server) portalDeleteRequest(w http.ResponseWriter, r *http.Request) {
	if !s.validPortalForm(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	item, err := s.requests.GetClientRequest(r.Context(), actor(r), id)
	if err != nil {
		s.portalReadError(w, err)
		return
	}
	if err = s.requests.DeleteClientRequest(r.Context(), actor(r), id); err != nil {
		if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
			http.Error(w, "Cette demande ne peut pas être supprimée.", http.StatusForbidden)
			return
		}
		s.portalReadError(w, err)
		return
	}
	http.Redirect(w, r, "/portal/projects/"+item.Project, http.StatusSeeOther)
}

func (s *Server) portalReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Contenu ou projet introuvable.", http.StatusNotFound)
		return
	}
	s.log.Error("client portal", "error", err)
	http.Error(w, "Le portail est momentanément indisponible.", http.StatusInternalServerError)
}

func clientStatusLabel(status string) string {
	switch status {
	case "received":
		return "En attente"
	case "rejected":
		return "Rejetée"
	case "in_progress":
		return "En cours"
	case "needs_info":
		return "Besoin d’information"
	case "done":
		return "Terminée"
	default:
		return "Reçue"
	}
}
