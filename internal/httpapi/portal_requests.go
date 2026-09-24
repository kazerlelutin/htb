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
	Counts                                     map[string]int
}

type portalCommentView struct {
	Author    string
	Body      template.HTML
	CreatedAt time.Time
}

type portalRequestView struct {
	ID                          int64
	Project, Title, StatusLabel string
	Body                        template.HTML
	Comments                    []portalCommentView
	CSRF, Error, DraftComment   string
}

var portalProjectTemplate = template.Must(template.New("portal-project").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Name}} — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a><header class="site-header"><a class="wordmark" href="/portal">&gt;_ HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p><a href="/portal">← Mes projets</a></p><p class="eyebrow">{{.Key}}</p><h1>{{.Name}}</h1>
<section aria-labelledby="status-title"><h2 id="status-title">Suivi</h2><p>Reçues : {{index .Counts "received"}} · En cours : {{index .Counts "in_progress"}} · Besoin d’information : {{index .Counts "needs_info"}} · Terminées : {{index .Counts "done"}}</p></section>
<section aria-labelledby="requests-title"><h2 id="requests-title">Demandes</h2>{{if .Requests}}<ul class="portal-requests">{{range .Requests}}<li><a href="/portal/requests/{{.ID}}"><strong>{{.Title}}</strong><span>{{.StatusLabel}}</span></a></li>{{end}}</ul>{{else}}<p>Aucune demande pour le moment.</p>{{end}}</section>
<section aria-labelledby="new-request-title"><h2 id="new-request-title">Proposer une demande</h2><p>Décrivez votre besoin en quelques mots. Vous pourrez suivre la conversation ici.</p>
{{if .Error}}<p class="portal-error" role="alert">{{.Error}}</p>{{end}}
<form class="portal-form" action="/portal/projects/{{.Key}}/requests" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="request-title">Sujet</label><input id="request-title" name="title" maxlength="240" required value="{{.Title}}"><label for="request-body">Description (Markdown accepté)</label><textarea id="request-body" name="body" maxlength="20000" rows="7" required>{{.Description}}</textarea><button class="button" type="submit">Envoyer la demande</button></form></section>
</main></body></html>`))

var portalRequestTemplate = template.Must(template.New("portal-request").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a><header class="site-header"><a class="wordmark" href="/portal">&gt;_ HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p><a href="/portal/projects/{{.Project}}">← Retour au projet</a></p><p class="eyebrow">DEMANDE #{{.ID}} · {{.StatusLabel}}</p><h1>{{.Title}}</h1><div class="portal-markdown">{{.Body}}</div>
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

func (s *Server) projectView(r *http.Request, key string) (portalProjectView, error) {
	view := portalProjectView{Key: key, CSRF: s.portalCSRF(r), Counts: map[string]int{}}
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
	for _, item := range items {
		view.Counts[item.Status]++
		view.Requests = append(view.Requests, portalRequestRow{ID: item.ID, Title: item.Title, StatusLabel: clientStatusLabel(item.Status), CreatedAt: item.CreatedAt})
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

func (s *Server) portalReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Demande ou projet introuvable.", http.StatusNotFound)
		return
	}
	s.log.Error("client portal", "error", err)
	http.Error(w, "Le portail est momentanément indisponible.", http.StatusInternalServerError)
}

func clientStatusLabel(status string) string {
	switch status {
	case "received":
		return "Reçue"
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
