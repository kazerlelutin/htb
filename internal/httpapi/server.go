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
	store        *store.Store
	verifier     auth.Verifier
	deviceConfig auth.DeviceConfig
	releaseURL   string
	log          *slog.Logger
}

type commandReference struct {
	Group, Syntax, Description string
}

var downloadCommands = []commandReference{
	{"Help", "htb help [COMMAND]", "Show the full local guide. Pass a command path such as 'ticket create' for its options and examples."},
	{"General", "htb version", "Show the installed CLI version when reporting an issue or checking an upgrade."},
	{"Connection", "htb config set-server URL", "Save the HTB server URL used by all subsequent commands on this computer."},
	{"Connection", "htb auth login [--issuer URL --client-id ID --audience ID]", "Open the browser sign-in flow. Normally the server provides the connection settings; the optional flags are only for an advanced manual setup."},
	{"Connection", "htb auth status", "Show whether you are connected, the projects you can access, and the project currently selected."},
	{"Projects", "htb project list", "List projects you can access. The star marks the current project."},
	{"Projects", "htb project create --key KEY --name NAME [--description TEXT]", "Create a project and select it immediately. KEY identifies the project in commands and ticket references such as HTB-1; lowercase input is converted to uppercase."},
	{"Projects", "htb project use KEY", "Select the project used when a command does not include --project."},
	{"Roadmap", "htb feature create --key KEY --name NAME [--project KEY] [--description TEXT] [--due-date YYYY-MM-DD]", "Create a roadmap feature. It uses the current project unless --project is provided; --due-date is optional."},
	{"Tickets", "htb ticket create --title TITLE [--type user_story|technical_task|bug|incident] [--project KEY] [--parent REF] [--related REF] [--feature KEY] [--description TEXT] [--priority low|normal|high|urgent] [--label TAG]", "Create a ticket in the current project by default. A technical task must use --parent with a user-story reference; repeat --label to attach several labels."},
	{"Tickets", "htb ticket list [--project KEY] [--feature KEY] [--tree] [--json|--csv]", "List tickets from the current project. Filter by feature, include child tickets with --tree, or select JSON/CSV for scripts."},
	{"Tickets", "htb ticket show REF", "Display one ticket, including its status, relationships, labels, and current version."},
	{"Tickets", "htb ticket update --version N [--title TITLE] [--description TEXT] [--status open|in_progress|review|blocked|done] [--priority low|normal|high|urgent] [--feature KEY] REF", "Update a ticket safely. Use the version shown by 'htb ticket show REF' so concurrent changes are not overwritten."},
	{"Tickets", "htb ticket comment REF TEXT", "Add a comment to a ticket."},
	{"Tickets", "htb ticket claim REF", "Assign a ticket to yourself and move an open ticket to in progress."},
	{"Tickets", "htb ticket release REF", "Remove your claim from a ticket so another person can take it."},
	{"Tickets", "htb ticket versions REF", "List the saved revisions of a ticket."},
	{"Tickets", "htb ticket restore --version N REF REVISION", "Restore a saved revision only when the ticket is still at version N, preventing accidental overwrites."},
	{"Invitations", "htb invite create [--project KEY] [--role read|write|admin] [--expires-at RFC3339]", "Create a shareable invitation for the current project or --project. Choose the member role and an expiry time."},
	{"Invitations", "htb invite accept CODE", "Accept an invitation code and gain access to its project."},
}

type actorKey struct{}
type principalKey struct{}

func New(s *store.Store, verifier auth.Verifier, deviceConfig auth.DeviceConfig, releaseURL string, log *slog.Logger) *Server {
	return &Server{store: s, verifier: verifier, deviceConfig: deviceConfig, releaseURL: releaseURL, log: log}
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
	fmt.Fprint(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>HTB — Headless Ticket Board</title></head><body><main><h1>Headless Ticket Board</h1><p>A shared work tracker for people, scripts, and agents.</p><p><a href=\"/downloads\">Download HTB and view installation commands</a></p></main></body></html>")
}
func (s *Server) downloads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	url := s.releaseURL
	if url == "" {
		url = "#"
	}
	installationURL := url + "/latest/download/install.sh"
	renderDownloadsPage(w, installationURL, url)
}

func renderDownloadsPage(w http.ResponseWriter, installationURL, releaseURL string) {
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Download HTB</title><style>body{font:16px system-ui,sans-serif;max-width:1100px;margin:2rem auto;padding:0 1rem;line-height:1.5}pre{background:#f4f4f4;padding:1rem;overflow:auto}table{border-collapse:collapse;width:100%%}th,td{border-bottom:1px solid #ddd;padding:.75rem;text-align:left;vertical-align:top}code{white-space:normal;overflow-wrap:anywhere}</style></head><body><main><h1>Download HTB</h1><h2>Install on Linux</h2><p>This command installs the CLI, verifies its checksum, and configures your user PATH:</p><pre>curl -fsSL %s | sh</pre><p>Open a new terminal, then run <code>htb auth login</code>.</p><p><a href="%s">Archives, checksums, and all GitHub releases</a></p><h2>Complete command guide</h2><p>Each command below explains when to use it and how its options affect the result. Run <code>htb help</code> or <code>htb help &lt;command&gt;</code> for the same guidance in the terminal.</p><table><caption>All supported commands</caption><thead><tr><th>Area</th><th>Command</th><th>How to use it</th></tr></thead><tbody>`, htmlText(installationURL), htmlText(releaseURL))
	for _, command := range downloadCommands {
		fmt.Fprintf(w, "<tr><td>%s</td><td><code>%s</code></td><td>%s</td></tr>", htmlText(command.Group), htmlText(command.Syntax), htmlText(command.Description))
	}
	fmt.Fprint(w, "</tbody></table></main></body></html>")
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
	key, err := domain.NormalizeProjectKey(in.Key)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	in.Key = key
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
