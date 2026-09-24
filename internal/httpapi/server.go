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
	sessions     browserSessionStore
	requests     clientRequestStore
	stories      clientStoryStore
	verifier     auth.Verifier
	browserLogin auth.BrowserLogin
	stateKey     []byte
	deviceConfig auth.DeviceConfig
	releaseURL   string
	publicURL    string
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
	{"Projects", "htb project status", "Show progress for every accessible project, including user stories, all tickets, and their status breakdown."},
	{"Projects", "htb project create --key KEY --name NAME [--description TEXT]", "Create a project and select it immediately. KEY identifies the project in commands and ticket references such as HTB-1; lowercase input is converted to uppercase."},
	{"Projects", "htb project use KEY", "Select the project used when a command does not include --project."},
	{"Projects", "htb project members [--project KEY]", "List project members. Administrators can use member IDs to change roles or remove access."},
	{"Projects", "htb project member set-role --user ID --role read|write|admin [--project KEY]", "Change a non-owner member role. The project owner cannot be demoted."},
	{"Projects", "htb project member remove --user ID [--project KEY]", "Remove a non-owner member from a project."},
	{"Roadmap", "htb feature create --key KEY --name NAME [--project KEY] [--description TEXT] [--due-date YYYY-MM-DD]", "Create a roadmap feature. It uses the current project unless --project is provided; --due-date is optional."},
	{"Tickets", "htb ticket create --title TITLE [--type user_story|technical_task|bug|incident] [--project KEY] [--parent REF] [--related REF] [--feature KEY] [--description TEXT] [--priority low|normal|high|urgent] [--label TAG]", "Create a ticket in the current project by default. A technical task must use --parent with a user-story reference; repeat --label to attach several labels."},
	{"Tickets", "htb ticket list [--project KEY] [--feature KEY] [--status STATUS] [--priority PRIORITY] [--label LABEL] [--query TEXT] [--tree] [--json|--csv]", "List tickets from the current project. Combine feature, status, priority, label, and text filters; include child tickets with --tree or select JSON/CSV for scripts."},
	{"Tickets", "htb ticket show REF", "Display one ticket, including its status, relationships, labels, and current version."},
	{"Tickets", "htb ticket update --version N [--title TITLE] [--description TEXT] [--status open|in_progress|review|blocked|done] [--priority low|normal|high|urgent] [--feature KEY] REF", "Update a ticket safely. Use the version shown by 'htb ticket show REF' so concurrent changes are not overwritten."},
	{"Tickets", "htb ticket comment REF TEXT", "Add a comment to a ticket."},
	{"Tickets", "htb ticket comments REF", "Read ticket comments with their author and timestamp."},
	{"Client stories", "htb ticket publish REF | htb ticket unpublish REF", "Make a user story visible to clients or hide it again; project admin only."},
	{"Client stories", "htb ticket client-comments REF", "Read the public conversation on a published user story."},
	{"Client stories", "htb ticket client-comment REF TEXT", "Reply to clients without exposing internal ticket comments."},
	{"Tickets", "htb ticket activity REF", "Read the ticket audit activity."},
	{"Tickets", "htb ticket claim REF", "Assign a ticket to yourself and move an open ticket to in progress."},
	{"Tickets", "htb ticket release REF", "Remove your claim from a ticket so another person can take it."},
	{"Tickets", "htb ticket versions REF", "List the saved revisions of a ticket."},
	{"Tickets", "htb ticket restore --version N REF REVISION", "Restore a saved revision only when the ticket is still at version N, preventing accidental overwrites."},
	{"Invitations", "htb invite create [--project KEY] [--role read|write|admin] [--expires-at RFC3339]", "Create a shareable invitation for the current project or --project. Choose the member role and an expiry time; it defaults to 7 days."},
	{"Invitations", "htb invite accept CODE", "Accept an invitation code and gain access to its project."},
	{"Invitations", "htb invite list [--project KEY]", "List active invitations without exposing their codes."},
	{"Invitations", "htb invite revoke ID [--project KEY]", "Revoke an active invitation."},
}

type actorKey struct{}
type principalKey struct{}

const defaultInvitationLifetime = 7 * 24 * time.Hour

type browserSessionStore interface {
	BrowserActor(context.Context, string, string, string) (store.Actor, error)
	CreateWebSession(context.Context, store.Actor, time.Duration) (string, error)
	WebSessionActor(context.Context, string) (store.Actor, error)
	RevokeWebSession(context.Context, string) error
	ListProjects(context.Context, store.Actor) ([]store.Project, error)
	AcceptInvitation(context.Context, string, string, string, string) error
}

type clientRequestStore interface {
	CreateClientRequest(context.Context, store.Actor, string, string, string) (store.ClientRequest, error)
	ListClientRequests(context.Context, store.Actor, string) ([]store.ClientRequest, error)
	GetClientRequest(context.Context, store.Actor, int64) (store.ClientRequest, error)
	ListClientRequestComments(context.Context, store.Actor, int64) ([]store.ClientRequestComment, error)
	AddClientRequestComment(context.Context, store.Actor, int64, string) (store.ClientRequestComment, error)
	UpdateClientRequest(context.Context, store.Actor, int64, store.ClientRequestUpdate) (store.ClientRequest, error)
}

type clientStoryStore interface {
	ListClientStories(context.Context, store.Actor, string) ([]store.ClientStory, error)
	GetClientStory(context.Context, store.Actor, string) (store.ClientStory, error)
	SetClientStoryPublished(context.Context, store.Actor, string, bool) error
	ListInternalStoryComments(context.Context, store.Actor, string) ([]store.Comment, error)
	AddInternalStoryComment(context.Context, store.Actor, string, string) (store.Comment, error)
	ListClientStoryComments(context.Context, store.Actor, string) ([]store.ClientStoryComment, error)
	AddClientStoryComment(context.Context, store.Actor, string, string) (store.ClientStoryComment, error)
}

func New(s *store.Store, verifier auth.Verifier, deviceConfig auth.DeviceConfig, releaseURL string, log *slog.Logger) *Server {
	return &Server{store: s, sessions: s, requests: s, stories: s, verifier: verifier, deviceConfig: deviceConfig, releaseURL: releaseURL, publicURL: "https://htb.ben-to.fr", log: log}
}

// SetBrowserLogin enables the web entry point backed by Zitadel.
// The key signs the transient PKCE state cookie; it must be at least 32 bytes.
func (s *Server) SetBrowserLogin(login auth.BrowserLogin, stateKey []byte) error {
	if login == nil || len(stateKey) < 32 {
		return fmt.Errorf("browser login requires an authenticator and a state key of at least 32 bytes")
	}
	s.browserLogin = login
	s.stateKey = append([]byte(nil), stateKey...)
	return nil
}

// SetPublicURL configures canonical URLs displayed on public pages.
func (s *Server) SetPublicURL(value string) {
	if value != "" {
		s.publicURL = strings.TrimRight(value, "/")
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /downloads", s.downloads)
	mux.HandleFunc("GET /commands", s.commands)
	mux.HandleFunc("GET /mentions-legales", s.legalNotice)
	mux.HandleFunc("GET /cgu", s.terms)
	mux.HandleFunc("GET /privacy", s.privacy)
	mux.HandleFunc("GET /assets/public.css", s.publicStyles)
	mux.HandleFunc("GET /assets/public.js", s.publicScript)
	mux.HandleFunc("GET /favicon.svg", s.favicon)
	mux.HandleFunc("GET /login", s.login)
	mux.HandleFunc("GET /auth/callback", s.browserCallback)
	mux.HandleFunc("POST /logout", s.logout)
	mux.Handle("GET /portal", s.browserAuthenticated(http.HandlerFunc(s.portalProjects)))
	mux.Handle("POST /portal/invitations", s.browserAuthenticated(http.HandlerFunc(s.portalAcceptInvitation)))
	mux.Handle("GET /portal/api/projects", s.browserAuthenticated(http.HandlerFunc(s.browserProjects)))
	mux.Handle("GET /portal/projects/{project}", s.browserAuthenticated(http.HandlerFunc(s.portalProject)))
	mux.Handle("GET /portal/stories/{ref}", s.browserAuthenticated(http.HandlerFunc(s.portalStory)))
	mux.Handle("POST /portal/stories/{ref}/ticket-comments", s.browserAuthenticated(http.HandlerFunc(s.portalAddInternalStoryComment)))
	mux.Handle("POST /portal/stories/{ref}/comments", s.browserAuthenticated(http.HandlerFunc(s.portalAddStoryComment)))
	mux.Handle("POST /portal/projects/{project}/requests", s.browserAuthenticated(http.HandlerFunc(s.portalCreateRequest)))
	mux.Handle("GET /portal/requests/{id}", s.browserAuthenticated(http.HandlerFunc(s.portalRequest)))
	mux.Handle("POST /portal/requests/{id}/comments", s.browserAuthenticated(http.HandlerFunc(s.portalAddComment)))
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
func (s *Server) downloads(w http.ResponseWriter, r *http.Request) {
	url := s.releaseURL
	if url == "" {
		url = "#"
	}
	installationURL := url + "/latest/download/install.sh"
	language := publicLanguage(r)
	s.renderPublicPage(w, r, localized(language, "Télécharger HTB", "Download HTB"), localized(language, "Installer la CLI HTB pour Linux.", "Install the HTB CLI for Linux."), downloadsBody(language, installationURL, url))
}

func (s *Server) commands(w http.ResponseWriter, r *http.Request) {
	language := publicLanguage(r)
	s.renderPublicPage(w, r, localized(language, "Commandes HTB", "HTB commands"), localized(language, "Guide de référence des commandes de la CLI HTB.", "Reference guide for HTB CLI commands."), commandsBody(language))
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := auth.Bearer(r)
		if err == nil {
			p, verifyErr := s.verifier.Verify(r.Context(), raw)
			if verifyErr != nil {
				s.log.Warn("Zitadel token verification failed", "error", verifyErr)
				writeError(w, 401, "invalid_token", "Token is invalid or expired", nil)
				return
			}
			ctx := context.WithValue(r.Context(), principalKey{}, p)
			actor, resolveErr := s.store.ResolveActor(ctx, p.Subject, p.Name, p.Email, p.Superadmin)
			if resolveErr != nil {
				writeStoreError(w, resolveErr)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, actorKey{}, actor)))
			return
		}
		writeError(w, 401, "unauthenticated", "Authentication is required", nil)
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
	case r.Method == "GET" && path == "projects/status":
		s.listProjectStatuses(w, r)
	case r.Method == "POST" && strings.HasPrefix(path, "projects/") && strings.HasSuffix(path, "/features"):
		s.createFeature(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "projects/"), "/features"))
	case strings.HasPrefix(path, "projects/"):
		s.projectAdministration(w, r, strings.TrimPrefix(path, "projects/"))
	case r.Method == "GET" && path == "tickets":
		s.listTickets(w, r)
	case r.Method == "GET" && path == "client-requests":
		s.listInternalClientRequests(w, r)
	case r.Method == "GET" && strings.HasPrefix(path, "client-requests/"):
		s.getInternalClientRequest(w, r, strings.TrimPrefix(path, "client-requests/"))
	case r.Method == "POST" && strings.HasPrefix(path, "client-requests/"):
		s.commentInternalClientRequest(w, r, strings.TrimPrefix(path, "client-requests/"))
	case r.Method == "PATCH" && strings.HasPrefix(path, "client-requests/"):
		s.updateInternalClientRequest(w, r, strings.TrimPrefix(path, "client-requests/"))
	case r.Method == "POST" && path == "tickets":
		s.createTicket(w, r)
	case strings.HasPrefix(path, "client-stories/"):
		s.clientStoryAPI(w, r, strings.TrimPrefix(path, "client-stories/"))
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
func (s *Server) listProjectStatuses(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjectStatuses(r.Context(), actor(r))
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
func (s *Server) projectAdministration(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet {
		members, err := s.store.ListProjectMembers(r.Context(), actor(r), parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
		return
	}
	if len(parts) == 3 && parts[1] == "members" && (r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
		userID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || userID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "Member ID must be numeric", nil)
			return
		}
		if r.Method == http.MethodPatch {
			var in struct {
				Role domain.Role `json:"role"`
			}
			if !decode(w, r, &in) {
				return
			}
			err = s.store.UpdateProjectMemberRole(r.Context(), actor(r), parts[0], userID, in.Role)
		} else {
			err = s.store.RemoveProjectMember(r.Context(), actor(r), parts[0], userID)
		}
		if err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "invitations" && r.Method == http.MethodGet {
		invitations, err := s.store.ListPendingInvitations(r.Context(), actor(r), parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitations": invitations})
		return
	}
	if len(parts) == 3 && parts[1] == "invitations" && r.Method == http.MethodDelete {
		invitationID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || invitationID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invitation ID must be numeric", nil)
			return
		}
		if err = s.store.RevokeInvitation(r.Context(), actor(r), parts[0], invitationID); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
}
func (s *Server) listTickets(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeError(w, 400, "invalid_request", "project is required", nil)
		return
	}
	filter := store.TicketFilter{
		ParentOnly: r.URL.Query().Get("tree") != "true",
		FeatureKey: r.URL.Query().Get("feature"),
		Status:     domain.Status(r.URL.Query().Get("status")),
		Priority:   r.URL.Query().Get("priority"),
		Label:      r.URL.Query().Get("label"),
		Query:      r.URL.Query().Get("query"),
	}
	items, err := s.store.ListTickets(r.Context(), actor(r), project, filter)
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
	if len(parts) == 2 && parts[1] == "comments" && r.Method == "GET" {
		comments, err := s.store.Comments(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"comments": comments})
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
	if len(parts) == 2 && parts[1] == "activity" && r.Method == "GET" {
		activity, err := s.store.Activity(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"activity": activity})
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
		ExpiresAt *time.Time  `json:"expires_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	code, err := s.store.CreateInvitation(r.Context(), actor(r), in.Project, in.Role, invitationExpiry(in.ExpiresAt, time.Now()))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"code": code})
}

func invitationExpiry(expiresAt *time.Time, now time.Time) time.Time {
	if expiresAt == nil {
		return now.Add(defaultInvitationLifetime)
	}
	return *expiresAt
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
	case errors.Is(err, store.ErrProjectLimit):
		writeError(w, 403, "project_limit_reached", "Your account has reached its project limit", nil)
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
