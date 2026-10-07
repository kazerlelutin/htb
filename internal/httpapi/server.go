package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/domain"
	"github.com/kazerlelutin/htb/internal/store"
)

type Server struct {
	store        *store.Store
	mcp          mcpStore
	mcpVerifier  auth.Verifier
	mcpEnabled   bool
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
	{"Connection", "htb config set-server URL", "Change the HTB server URL used by all subsequent commands. The default is https://htboard.xyz."},
	{"Connection", "htb auth login [--issuer URL --client-id ID --audience ID]", "Open the browser sign-in flow. Normally the server provides the connection settings; the optional flags are only for an advanced manual setup."},
	{"Connection", "htb auth status", "Show whether you are connected, the projects you can access, and the project currently selected."},
	{"Projects", "htb project list", "List projects you can access. The star marks the current project."},
	{"Projects", "htb project status [--namespace NAMESPACE]", "Show progress for every accessible project, including user stories, all tickets, and their status breakdown. Use --namespace to filter by namespace; projects are grouped automatically when multiple namespaces exist."},
	{"Projects", "htb project create --key KEY --name NAME [--description TEXT]", "Create a project and select it immediately. KEY identifies the project in commands and ticket references such as HTB-1 or ALICE/SITE-1; lowercase input is converted to uppercase. An optional namespace prefix (NAMESPACE/KEY) allows multiple projects with the same short key."},
	{"Projects", "htb project use KEY", "Select the project used when a command does not include --project. KEY can be a short key (e.g., SITE) or a namespaced key (e.g., ALICE/SITE). If ambiguous, the CLI will list options."},
	{"Projects", "htb project rename --new-key KEY [--project KEY]", "Rename a project key, including its namespace. Project administrators only; when the current project is renamed, the local selection is updated."},
	{"Namespaces", "htb namespace claim NAME | list", "Reserve a namespace before using it in a project key. The number of reserved namespaces depends on the account plan."},
	{"Projects", "htb project members [--project KEY]", "List project members. Administrators can use member IDs to change roles or remove access."},
	{"Projects", "htb project member set-role --user ID --role read|write|admin [--project KEY]", "Change a non-owner member role. The project owner cannot be demoted."},
	{"Projects", "htb project member remove --user ID [--project KEY]", "Remove a non-owner member from a project."},
	{"Roadmap", "htb feature create --key KEY --name NAME [--project KEY] [--description TEXT] [--due-date YYYY-MM-DD]", "Create a roadmap feature. It uses the current project unless --project is provided; --due-date is optional."},
	{"Tickets", "htb ticket create --title TITLE [--type user_story|technical_task|bug|incident] [--project KEY] [--parent REF] [--related REF] [--feature KEY] [--description TEXT] [--priority low|normal|high|urgent] [--label TAG]", "Create a ticket in the current project by default. A technical task must use --parent with a user-story reference; repeat --label to attach several labels."},
	{"Tickets", "htb ticket list [--project KEY | --all-projects] [--feature KEY] [--status STATUS] [--priority PRIORITY] [--label LABEL] [--query TEXT] [--archived] [--tree] [--json|--csv]", "Search the current project by default, another project with --project, or every accessible project with --all-projects."},
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
	{"Tickets", "htb ticket archive REF | unarchive REF", "Archive or restore a ticket. Only its creator or a project administrator can do this."},
	{"Tickets", "htb ticket delete --confirm REF", "Permanently delete a ticket created by you, or as project administrator. Use only to correct entry mistakes or duplicates."},
	{"Invitations", "htb invite create [--project KEY | --namespace NAME] [--role read|write|admin] [--expires-at RFC3339]", "Create a shareable invitation for the current project or every project in a namespace. Choose the member role and an expiry time; it defaults to 7 days."},
	{"Invitations", "htb invite accept CODE", "Accept an invitation code and gain access to its project or namespace."},
	{"Invitations", "htb invite list [--project KEY | --namespace NAME]", "List active invitations without exposing their codes."},
	{"Invitations", "htb invite revoke ID [--project KEY | --namespace NAME]", "Revoke an active invitation."},
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
	DeleteClientRequest(context.Context, store.Actor, int64) error
}

type clientStoryStore interface {
	ListClientStories(context.Context, store.Actor, string) ([]store.ClientStory, error)
	ListClientStoriesPage(context.Context, store.Actor, string, string, int, int) (store.ClientStoryPage, error)
	GetClientStory(context.Context, store.Actor, string) (store.ClientStory, error)
	SetClientStoryPublished(context.Context, store.Actor, string, bool) error
	ListInternalStoryComments(context.Context, store.Actor, string) ([]store.Comment, error)
	AddInternalStoryComment(context.Context, store.Actor, string, string) (store.Comment, error)
	ListClientStoryComments(context.Context, store.Actor, string) ([]store.ClientStoryComment, error)
	AddClientStoryComment(context.Context, store.Actor, string, string) (store.ClientStoryComment, error)
	UpdateClientStoryComment(context.Context, store.Actor, string, int64, string) (store.ClientStoryComment, error)
	DeleteClientStoryComment(context.Context, store.Actor, string, int64) error
}

func New(s *store.Store, verifier auth.Verifier, deviceConfig auth.DeviceConfig, releaseURL string, log *slog.Logger) *Server {
	return &Server{store: s, mcp: s, sessions: s, requests: s, stories: s, verifier: verifier, deviceConfig: deviceConfig, releaseURL: releaseURL, publicURL: "https://htboard.xyz", log: log}
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

// SetMCPVerifier accepts access tokens issued to the separate Zitadel DCR
// project used by ChatGPT. The REST API continues to use the primary verifier.
func (s *Server) SetMCPVerifier(verifier auth.Verifier) { s.mcpVerifier = verifier }

// SetMCPEnabled exposes the MCP and OAuth discovery endpoints. It is opt-in
// because a public MCP endpoint needs its own Zitadel and proxy configuration.
func (s *Server) SetMCPEnabled(enabled bool) { s.mcpEnabled = enabled }

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
	mux.HandleFunc("GET /openapi.v1.yaml", s.openAPI)
	mux.HandleFunc("GET /login", s.login)
	mux.HandleFunc("GET /auth/callback", s.browserCallback)
	mux.HandleFunc("POST /logout", s.logout)
	mux.Handle("GET /portal", s.browserAuthenticated(http.HandlerFunc(s.portalProjects)))
	mux.Handle("POST /portal/invitations", s.browserAuthenticated(http.HandlerFunc(s.portalAcceptInvitation)))
	mux.Handle("GET /portal/api/projects", s.browserAuthenticated(http.HandlerFunc(s.browserProjects)))
	mux.Handle("GET /portal/projects/{project...}", s.browserAuthenticated(http.HandlerFunc(s.portalProject)))
	mux.Handle("POST /portal/projects/{project...}", s.browserAuthenticated(http.HandlerFunc(s.portalProjectAction)))
	mux.Handle("GET /portal/stories/{ref...}", s.browserAuthenticated(http.HandlerFunc(s.portalStory)))
	mux.Handle("POST /portal/stories/{ref...}", s.browserAuthenticated(http.HandlerFunc(s.portalStoryAction)))
	mux.Handle("GET /portal/requests/{id}", s.browserAuthenticated(http.HandlerFunc(s.portalRequest)))
	mux.Handle("POST /portal/requests/{id}/comments", s.browserAuthenticated(http.HandlerFunc(s.portalAddComment)))
	mux.Handle("POST /portal/requests/{id}/delete", s.browserAuthenticated(http.HandlerFunc(s.portalDeleteRequest)))
	mux.HandleFunc("GET /auth/device-config", s.deviceConfiguration)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.mcpProtectedResourceMetadata)
	mux.HandleFunc("/mcp", s.mcpHandler)
	mux.Handle("/api/v1/", s.authenticated(http.HandlerFunc(s.api)))
	return s.logging(mux)
}

func (s *Server) portalProjectAction(w http.ResponseWriter, r *http.Request) {
	project, ok := strings.CutSuffix(r.PathValue("project"), "/requests")
	if !ok || project == "" {
		http.NotFound(w, r)
		return
	}
	s.portalCreateRequest(w, r, project)
}

func (s *Server) portalStoryAction(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("ref")
	if ref, commentID, ok := portalStoryCommentAction(path, "edit"); ok {
		s.portalUpdateStoryComment(w, r, ref, commentID)
		return
	}
	if ref, commentID, ok := portalStoryCommentAction(path, "delete"); ok {
		s.portalDeleteStoryComment(w, r, ref, commentID)
		return
	}
	if ref, ok := strings.CutSuffix(path, "/ticket-comments"); ok && ref != "" {
		s.portalAddInternalStoryComment(w, r, ref)
		return
	}
	if ref, ok := strings.CutSuffix(path, "/comments"); ok && ref != "" {
		s.portalAddStoryComment(w, r, ref)
		return
	}
	http.NotFound(w, r)
}

func portalStoryCommentAction(path, action string) (string, int64, bool) {
	path, ok := strings.CutSuffix(path, "/"+action)
	if !ok {
		return "", 0, false
	}
	ref, rawID, ok := projectItemSubresource(path, "comments")
	if !ok {
		return "", 0, false
	}
	commentID, err := strconv.ParseInt(rawID, 10, 64)
	return ref, commentID, err == nil && commentID > 0
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
		a := actor(r)
		writeJSON(w, 200, map[string]any{"subject": a.Subject, "superadmin": a.Superadmin, "name": a.Name})
	case r.Method == "GET" && path == "namespaces":
		s.listNamespaces(w, r)
	case r.Method == "POST" && path == "namespaces":
		s.claimNamespace(w, r)
	case strings.HasPrefix(path, "namespaces/"):
		s.namespaceAdministration(w, r, strings.TrimPrefix(path, "namespaces/"))
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

func (s *Server) listNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := s.store.ListNamespaces(r.Context(), actor(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"namespaces": namespaces})
}

func (s *Server) claimNamespace(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	namespace, err := s.store.ClaimNamespace(r.Context(), actor(r), in.Name)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, namespace)
}

func (s *Server) namespaceAdministration(w http.ResponseWriter, r *http.Request, path string) {
	if namespace, ok := projectSubresource(path, "invitations"); ok && r.Method == http.MethodGet {
		invitations, err := s.store.ListPendingNamespaceInvitations(r.Context(), actor(r), namespace)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitations": invitations})
		return
	}
	if namespace, invitation, ok := projectItemSubresource(path, "invitations"); ok && r.Method == http.MethodDelete {
		invitationID, err := strconv.ParseInt(invitation, 10, 64)
		if err != nil || invitationID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invitation ID must be numeric", nil)
			return
		}
		if err = s.store.RevokeNamespaceInvitation(r.Context(), actor(r), namespace, invitationID); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
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
	if project, ok := projectSubresource(path, "archive"); ok && r.Method == http.MethodPost {
		if err := s.store.SetProjectArchived(r.Context(), actor(r), project, true); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if project, ok := projectSubresource(path, "restore"); ok && r.Method == http.MethodPost {
		if err := s.store.SetProjectArchived(r.Context(), actor(r), project, false); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if project, ok := projectSubresource(path, "members"); ok && r.Method == http.MethodGet {
		members, err := s.store.ListProjectMembers(r.Context(), actor(r), project)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})
		return
	}
	if project, memberID, ok := projectItemSubresource(path, "members"); ok && (r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
		userID, err := strconv.ParseInt(memberID, 10, 64)
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
			err = s.store.UpdateProjectMemberRole(r.Context(), actor(r), project, userID, in.Role)
		} else {
			err = s.store.RemoveProjectMember(r.Context(), actor(r), project, userID)
		}
		if err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if project, ok := projectSubresource(path, "invitations"); ok && r.Method == http.MethodGet {
		invitations, err := s.store.ListPendingInvitations(r.Context(), actor(r), project)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitations": invitations})
		return
	}
	if project, invitation, ok := projectItemSubresource(path, "invitations"); ok && r.Method == http.MethodDelete {
		invitationID, err := strconv.ParseInt(invitation, 10, 64)
		if err != nil || invitationID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invitation ID must be numeric", nil)
			return
		}
		if err = s.store.RevokeInvitation(r.Context(), actor(r), project, invitationID); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if project, ok := projectSubresource(path, "key"); ok && r.Method == http.MethodPut {
		var in struct {
			Key string `json:"key"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := s.store.UpdateProjectKey(r.Context(), actor(r), project, in.Key); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
}

// projectSubresource extracts a complete project key before a named resource.
// Project keys may contain one slash as a namespace separator, so splitting the
// path into fixed positions would lose part of the key.
func projectSubresource(path, resource string) (string, bool) {
	project, ok := strings.CutSuffix(path, "/"+resource)
	return project, ok && project != ""
}

// projectItemSubresource extracts a complete project key and an item ID.
func projectItemSubresource(path, resource string) (string, string, bool) {
	marker := "/" + resource + "/"
	idx := strings.LastIndex(path, marker)
	if idx <= 0 || idx+len(marker) == len(path) {
		return "", "", false
	}
	item := path[idx+len(marker):]
	if strings.Contains(item, "/") {
		return "", "", false
	}
	return path[:idx], item, true
}

// ticketReference accepts a complete ticket reference, including a possible
// namespace in its project key.
func ticketReference(path string) (string, bool) {
	if _, _, err := domain.ParseReference(path); err != nil {
		return "", false
	}
	return path, true
}

// ticketSubresource extracts a complete ticket reference before a named
// resource. A ticket reference can contain a slash when its project is
// namespaced, so its path must be parsed from the end.
func ticketSubresource(path, resource string) (string, bool) {
	ref, ok := strings.CutSuffix(path, "/"+resource)
	if !ok {
		return "", false
	}
	return ticketReference(ref)
}

// ticketRevisionRestore extracts a ticket reference and a revision from a
// restore route, preserving an optional project namespace.
func ticketRevisionRestore(path string) (string, string, bool) {
	path, ok := strings.CutSuffix(path, "/restore")
	if !ok {
		return "", "", false
	}
	ref, revision, ok := projectItemSubresource(path, "versions")
	if !ok {
		return "", "", false
	}
	ref, ok = ticketReference(ref)
	return ref, revision, ok
}

func (s *Server) listTickets(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	filter := store.TicketFilter{
		ParentOnly: r.URL.Query().Get("tree") != "true",
		Archived:   r.URL.Query().Get("archived") == "true",
		FeatureKey: r.URL.Query().Get("feature"),
		Status:     domain.Status(r.URL.Query().Get("status")),
		Priority:   r.URL.Query().Get("priority"),
		Label:      r.URL.Query().Get("label"),
		Query:      r.URL.Query().Get("query"),
	}
	if project != "" {
		items, err := s.store.ListTickets(r.Context(), actor(r), project, filter)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"tickets": items})
		return
	}
	projects, err := s.store.ListProjects(r.Context(), actor(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	items := make([]store.Ticket, 0)
	for _, accessibleProject := range projects {
		projectTickets, err := s.store.ListTickets(r.Context(), actor(r), accessibleProject.Key, filter)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		items = append(items, projectTickets...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Project == items[j].Project {
			return items[i].Number > items[j].Number
		}
		return items[i].Project < items[j].Project
	})
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
	if ref, revisionText, ok := ticketRevisionRestore(tail); ok && r.Method == "POST" {
		revision, err := strconv.Atoi(revisionText)
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
	if ref, ok := ticketSubresource(tail, "comments"); ok && r.Method == "GET" {
		comments, err := s.store.Comments(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"comments": comments})
		return
	}
	if ref, ok := ticketSubresource(tail, "comments"); ok && r.Method == "POST" {
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
	if ref, ok := ticketSubresource(tail, "activity"); ok && r.Method == "GET" {
		activity, err := s.store.Activity(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"activity": activity})
		return
	}
	if ref, ok := ticketSubresource(tail, "claim"); ok && r.Method == "POST" {
		ticket, err := s.store.Claim(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, ticket)
		return
	}
	if ref, ok := ticketSubresource(tail, "release"); ok && r.Method == "POST" {
		if err := s.store.Release(r.Context(), actor(r), ref); err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "released"})
		return
	}
	if ref, ok := ticketSubresource(tail, "archive"); ok && r.Method == "POST" {
		if err := s.store.SetTicketArchived(r.Context(), actor(r), ref, true); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if ref, ok := ticketSubresource(tail, "unarchive"); ok && r.Method == "POST" {
		if err := s.store.SetTicketArchived(r.Context(), actor(r), ref, false); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if ref, ok := ticketSubresource(tail, "versions"); ok && r.Method == "GET" {
		versions, err := s.store.Revisions(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"versions": versions})
		return
	}
	ref, ok := ticketReference(tail)
	if !ok {
		writeError(w, 404, "not_found", "Route not found", nil)
		return
	}
	switch r.Method {
	case "GET":
		ticket, err := s.store.GetTicket(r.Context(), actor(r), ref)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, 200, ticket)
		return
	case "PATCH":
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
	case "DELETE":
		if err := s.store.DeleteTicket(r.Context(), actor(r), ref); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	default:
	}
	writeError(w, 404, "not_found", "Route not found", nil)
}
func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Project   string      `json:"project"`
		Namespace string      `json:"namespace"`
		Role      domain.Role `json:"role"`
		ExpiresAt *time.Time  `json:"expires_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Project = strings.TrimSpace(in.Project)
	in.Namespace = strings.TrimSpace(in.Namespace)
	if (in.Project == "") == (in.Namespace == "") {
		writeError(w, http.StatusBadRequest, "invalid_request", "Specify exactly one project or namespace", nil)
		return
	}
	var code string
	var err error
	if in.Namespace != "" {
		code, err = s.store.CreateNamespaceInvitation(r.Context(), actor(r), in.Namespace, in.Role, invitationExpiry(in.ExpiresAt, time.Now()))
	} else {
		code, err = s.store.CreateInvitation(r.Context(), actor(r), in.Project, in.Role, invitationExpiry(in.ExpiresAt, time.Now()))
	}
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
	case errors.Is(err, store.ErrTicketHasDependencies):
		writeError(w, 409, "ticket_has_dependencies", "Remove ticket links or dependent tickets before deleting it", nil)
	case errors.Is(err, store.ErrProjectLimit):
		writeError(w, 403, "project_limit_reached", "Your account has reached its project limit", nil)
	case errors.Is(err, store.ErrNamespaceLimit):
		writeError(w, 403, "namespace_limit_reached", "Your account has reached its namespace limit", nil)
	case errors.Is(err, store.ErrNamespaceRequired):
		writeError(w, 403, "namespace_required", "Claim this namespace before using it", nil)
	case errors.Is(err, store.ErrNamespaceReserved):
		writeError(w, 403, "namespace_reserved", "This namespace is reserved by another account", nil)
	default:
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) {
			writeError(w, http.StatusInternalServerError, "server_error", "Unable to process the request", nil)
			return
		}
		// Store validation errors are intentionally terse. Detailed database and
		// implementation errors must never be reflected to an API client.
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "Request could not be processed", nil)
	}
}
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
