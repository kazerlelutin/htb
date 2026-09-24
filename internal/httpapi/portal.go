package httpapi

import (
	"html/template"
	"net/http"
)

var portalTemplate = template.Must(template.New("portal").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Mes projets — HTB</title><link rel="stylesheet" href="/assets/public.css"></head><body>
<a class="skip-link" href="#main">Aller au contenu</a>
<header class="site-header"><a class="wordmark" href="/" aria-label="Accueil HTB">&gt;_ HTB</a><form action="/logout" method="post"><button class="link-button" type="submit">Se déconnecter</button></form></header>
<main id="main" class="portal"><p class="eyebrow">ESPACE CLIENT</p><h1>Mes projets</h1>
{{if .}}<p>Les projets auxquels vous avez accès.</p><ul class="portal-projects">{{range .}}<li><strong>{{.Name}}</strong><span>{{.Key}}</span></li>{{end}}</ul>{{else}}<p>Aucun projet ne vous est encore attribué. Contactez la personne qui vous a invité.</p>{{end}}
</main></body></html>`))

func (s *Server) portalProjects(w http.ResponseWriter, r *http.Request) {
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
	if err := portalTemplate.Execute(w, projects); err != nil {
		s.log.Error("render browser projects", "error", err)
	}
}
