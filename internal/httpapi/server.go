package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/domain"
	"github.com/kazerlelutin/htb/internal/store"
)

type Server struct {
	store               *store.Store
	verifier            auth.Verifier
	deviceConfig        auth.DeviceConfig
	version, releaseURL string
	log                 *slog.Logger
}
type actorKey struct{}
type principalKey struct{}

func New(s *store.Store, verifier auth.Verifier, deviceConfig auth.DeviceConfig, version, releaseURL string, log *slog.Logger) *Server {
	return &Server{store: s, verifier: verifier, deviceConfig: deviceConfig, version: version, releaseURL: releaseURL, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /downloads", s.downloads)
	mux.HandleFunc("GET /auth/device-config", s.deviceConfiguration)
	mux.Handle("/api/v1/", s.authenticated(http.HandlerFunc(s.api)))
	return s.logging(mux)
}
func (s *Server) deviceConfiguration(w http.ResponseWriter, r *http.Request) {
	if s.deviceConfig.Issuer == "" || s.deviceConfig.ClientID == "" || s.deviceConfig.Audience == "" {
		writeError(w, http.StatusServiceUnavailable, "device_login_unavailable", "CLI login is not configured", nil)
		return
	}
	writeJSON(w, http.StatusOK, s.deviceConfig)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "Database is unavailable", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<!doctype html><html lang=\"fr\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>HTB — Headless Ticket Board</title></head><body><main><h1>Headless Ticket Board</h1><p>Un registre de travail partagé pour humains, scripts et agents.</p><p><a href=\"/downloads\">Télécharger HTB et voir les commandes d’installation</a></p></main></body></html>")
}
func (s *Server) downloads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	url := s.releaseURL
	if url == "" {
		url = "#"
	}
	fmt.Fprintf(w, `<!doctype html><html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Téléchargements HTB</title><style>body{font:16px system-ui,sans-serif;max-width:900px;margin:2rem auto;padding:0 1rem;line-height:1.5}pre{background:#f4f4f4;padding:1rem;overflow:auto}table{border-collapse:collapse;width:100%%}th,td{border-bottom:1px solid #ddd;padding:.55rem;text-align:left}code{white-space:nowrap}</style></head><body><main><h1>Téléchargements HTB</h1><p>Version serveur : <strong>%s</strong></p><h2>Installation Linux</h2><p>Installe la CLI dans le PATH utilisateur, sans sudo :</p><pre>mkdir -p "$HOME/.local/bin"
curl -fsSL %s/latest/download/htb_linux_amd64.tar.gz | tar -xz -C "$HOME/.local/bin" htb
export PATH="$HOME/.local/bin:$PATH"
htb version</pre><p>Ajoute <code>export PATH="$HOME/.local/bin:$PATH"</code> à <code>~/.bashrc</code> pour conserver ce réglage.</p><p><a href="%s">Archives, checksums et toutes les versions GitHub</a></p><h2>Commandes essentielles</h2><p>La CLI embarque l'aide complète : <code>htb help</code> ou <code>htb help ticket create</code>.</p><table><thead><tr><th>Action</th><th>Commande</th></tr></thead><tbody><tr><td>État de connexion</td><td><code>htb auth status</code></td></tr><tr><td>Changer de projet</td><td><code>htb project use SITE</code></td></tr><tr><td>Créer une fonctionnalité</td><td><code>htb feature create --key newsletter --name Newsletter --due-date 2026-09-30</code></td></tr><tr><td>Créer une US</td><td><code>htb ticket create --type user_story --title "Titre"</code></td></tr><tr><td>Créer une tâche</td><td><code>htb ticket create --type technical_task --parent SITE-2 --title "Titre"</code></td></tr><tr><td>Voir les tickets</td><td><code>htb ticket list</code></td></tr><tr><td>Filtrer par fonctionnalité</td><td><code>htb ticket list --feature newsletter</code></td></tr><tr><td>Mettre à jour</td><td><code>htb ticket update --version 2 --status in_progress SITE-3</code></td></tr></tbody></table></main></body></html>`, htmlText(s.version), htmlText(url), htmlText(url))
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := auth.Bearer(r)
		if err != nil {
			writeError(w, 401, "unauthenticated", "Authentication is required", nil)
			return
		}
		p, err := s.verifier.Verify(r.Context(), raw)
		if err != nil {
			s.log.Warn("Zitadel token verification failed", "error", err)
			writeError(w, 401, "invalid_token", "Token is invalid or expired", nil)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, p)
		actor, err := s.store.ResolveActor(ctx, p.Subject, p.Name, p.Email, p.Superadmin)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, actorKey{}, actor)))
	})
}
func actor(r *http.Request) store.Actor { return r.Context().Value(actorKey{}).(store.Actor) }
func principal(r *http.Request) auth.Principal {
	return r.Context().Value(principalKey{}).(auth.Principal)
}

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	switch {
	case r.Method == "GET" && path == "me":
		writeJSON(w, 200, actor(r))
	case r.Method == "POST" && path == "projects":
		s.createProject(w, r)
	case r.Method == "GET" && path == "projects":
		s.listProjects(w, r)
	case r.Method == "POST" && strings.HasPrefix(path, "projects/") && strings.HasSuffix(path, "/features"):
		s.createFeature(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "projects/"), "/features"))
	case r.Method == "GET" && path == "tickets":
		s.listTickets(w, r)
	case r.Method == "POST" && path == "tickets":
		s.createTicket(w, r)
	case strings.HasPrefix(path, "tickets/"):
		s.ticket(w, r, strings.TrimPrefix(path, "tickets/"))
	case r.Method == "POST" && path == "invitations":
		s.createInvitation(w, r)
	case r.Method == "POST" && strings.HasPrefix(path, "invitations/") && strings.HasSuffix(path, "/accept"):
		s.acceptInvitation(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "invitations/"), "/accept"))
	default:
		writeError(w, 404, "not_found", "Route not found", nil)
	}
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var in store.Project
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.CreateProject(r.Context(), actor(r), in); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 201, in)
}
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context(), actor(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"projects": projects})
}
func (s *Server) createFeature(w http.ResponseWriter, r *http.Request, project string) {
	var in store.Feature
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.CreateFeature(r.Context(), actor(r), project, in); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 201, in)
}
func (s *Server) listTickets(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeError(w, 400, "invalid_request", "project is required", nil)
		return
	}
	items, err := s.store.ListTickets(r.Context(), actor(r), project, r.URL.Query().Get("tree") != "true", r.URL.Query().Get("feature"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"tickets": items})
}
func (s *Server) createTicket(w http.ResponseWriter, r *http.Request) {
	var in store.CreateTicket
	if !decode(w, r, &in) {
		return
	}
	ticket, err := s.store.CreateTicket(r.Context(), actor(r), in)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 201, ticket)
}
func (s *Server) ticket(w http.ResponseWriter, r *http.Request, tail string) {
	parts := strings.Split(tail, "/")
	ref := parts[0]
	if len(parts) == 4 && parts[1] == "versions" && parts[3] == "restore" && r.Method == "POST" {
		revision, err := strconv.Atoi(parts[2])
		if err != nil {
			writeError(w, 400, "invalid_request", "Revision must be numeric", nil)
			return
		}
		var in struct {
			ExpectedVersion int `json:"expected_version"`
		}
		if !decode(w, r, &in) {
			return
		}
		ticket, err := s.store.Restore(r.Context(), actor(r), ref, revision, in.ExpectedVersion)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, ticket)
		return
	}
	if len(parts) == 2 && parts[1] == "comments" && r.Method == "POST" {
		var in struct {
			Body string `json:"body"`
		}
		if !decode(w, r, &in) {
			return
		}
		comment, err := s.store.AddComment(r.Context(), actor(r), ref, in.Body)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 201, comment)
		return
	}
	if len(parts) == 2 && parts[1] == "claim" && r.Method == "POST" {
		ticket, err := s.store.Claim(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, ticket)
		return
	}
	if len(parts) == 2 && parts[1] == "release" && r.Method == "POST" {
		if err := s.store.Release(r.Context(), actor(r), ref); err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "released"})
		return
	}
	if len(parts) == 2 && parts[1] == "versions" && r.Method == "GET" {
		versions, err := s.store.Revisions(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"versions": versions})
		return
	}
	switch r.Method {
	case "GET":
		if len(parts) != 1 {
			break
		}
		ticket, err := s.store.GetTicket(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, ticket)
		return
	case "PATCH":
		if len(parts) != 1 {
			break
		}
		var in store.UpdateTicket
		if !decode(w, r, &in) {
			return
		}
		ticket, err := s.store.UpdateTicket(r.Context(), actor(r), ref, in)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		w.Header().Set("ETag", strconv.Itoa(ticket.Version))
		writeJSON(w, 200, ticket)
		return
	default:
	}
	writeError(w, 404, "not_found", "Route not found", nil)
}
func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Project   string      `json:"project"`
		Role      domain.Role `json:"role"`
		ExpiresAt time.Time   `json:"expires_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	code, err := s.store.CreateInvitation(r.Context(), actor(r), in.Project, in.Role, in.ExpiresAt)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"code": code})
}
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request, code string) {
	p := principal(r)
	if err := s.store.AcceptInvitation(r.Context(), p.Subject, p.Name, p.Email, code); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "accepted"})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, 400, "invalid_json", "Request body must contain valid JSON", nil)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "details": details}})
}
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrForbidden):
		writeError(w, 403, "forbidden", "You do not have permission for this operation", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, 404, "not_found", "Resource not found", nil)
	case errors.Is(err, store.ErrConflict):
		writeError(w, 409, "version_conflict", "Ticket has changed; fetch it and retry", nil)
	default:
		writeError(w, 422, "invalid_request", err.Error(), nil)
	}
}
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
func htmlText(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", "\"", "&quot;").Replace(v)
}
