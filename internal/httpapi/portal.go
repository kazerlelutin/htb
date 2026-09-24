package httpapi

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	"github.com/kazerlelutin/htb/internal/store"
)

type portalHomeView struct {
	Projects []store.Project
	CSRF     string
	Error    string
}

var portalTemplate = template.Must(template.New("portal").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Mes projets — HTB</title><link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a>
<header class="site-header"><a class="wordmark" href="/" aria-label="Accueil HTB">&gt;_ HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p class="eyebrow">ESPACE CLIENT</p><h1>Mes projets</h1>
{{if .Projects}}<p>Les projets auxquels vous avez accès.</p><ul class="portal-projects">{{range .Projects}}<li><a href="/portal/projects/{{.Key}}"><strong>{{.Name}}</strong><span>{{.Key}}</span></a></li>{{end}}</ul>{{else}}<p>Aucun projet ne vous est encore attribué.</p>{{end}}
<section aria-labelledby="join-title"><h2 id="join-title">Rejoindre un projet</h2><p>Vous avez reçu un code d’invitation ? Saisissez-le ici.</p>{{if .Error}}<p class="portal-error" role="alert">{{.Error}}</p>{{end}}<form class="portal-form" action="/portal/invitations" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="invitation-code">Code d’invitation</label><input id="invitation-code" name="code" autocomplete="off" required><button class="button" type="submit">Rejoindre le projet</button></form></section>
</main></body></html>`))

func (s *Server) portalProjects(w http.ResponseWriter, r *http.Request) {
	s.renderPortalProjects(w, r, "", http.StatusOK)
}

func (s *Server) renderPortalProjects(w http.ResponseWriter, r *http.Request, message string, status int) {
	projects, err := s.sessions.ListProjects(r.Context(), actor(r))
	if err != nil {
		s.log.Error("render browser projects", "error", err)
		http.Error(w, "Unable to display projects", http.StatusInternalServerError)
		return
	}
	s.publicHeaders(w)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := portalTemplate.Execute(w, portalHomeView{Projects: projects, CSRF: s.portalCSRF(r), Error: message}); err != nil {
		s.log.Error("render browser projects", "error", err)
	}
}

func (s *Server) portalAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	if !s.validPortalForm(w, r) {
		return
	}
	code := strings.TrimSpace(r.PostForm.Get("code"))
	if code == "" || len(code) > 128 {
		s.renderPortalProjects(w, r, "Saisissez un code d’invitation valide.", http.StatusUnprocessableEntity)
		return
	}
	err := s.sessions.AcceptInvitation(r.Context(), actor(r).Subject, "", "", code)
	if errors.Is(err, store.ErrNotFound) {
		s.renderPortalProjects(w, r, "Ce code est invalide, expiré ou déjà utilisé.", http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		s.log.Error("accept browser invitation", "error", err)
		http.Error(w, "Impossible de rejoindre le projet pour le moment.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/portal", http.StatusSeeOther)
}
