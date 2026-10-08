package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/store"
)

type clientRequestStub struct {
	items     []store.ClientRequest
	comments  []store.ClientRequestComment
	project   string
	created   bool
	commented bool
	deleted   bool
}

func (stub *clientRequestStub) CreateClientRequest(_ context.Context, _ store.Actor, project, title, body string) (store.ClientRequest, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return store.ClientRequest{}, store.ErrInvalidClientRequest
	}
	stub.created = true
	return store.ClientRequest{ID: 7, Project: project, Title: title, Body: body}, nil
}
func (stub *clientRequestStub) ListClientRequests(_ context.Context, _ store.Actor, project string) ([]store.ClientRequest, error) {
	allowed := stub.project
	if allowed == "" {
		allowed = "SITE"
	}
	if project != allowed {
		return nil, store.ErrForbidden
	}
	return stub.items, nil
}
func (stub *clientRequestStub) GetClientRequest(_ context.Context, _ store.Actor, id int64) (store.ClientRequest, error) {
	for _, item := range stub.items {
		if item.ID == id {
			return item, nil
		}
	}
	return store.ClientRequest{}, store.ErrNotFound
}
func (stub *clientRequestStub) ListClientRequestComments(context.Context, store.Actor, int64) ([]store.ClientRequestComment, error) {
	return stub.comments, nil
}
func (stub *clientRequestStub) AddClientRequestComment(_ context.Context, _ store.Actor, _ int64, body string) (store.ClientRequestComment, error) {
	if strings.TrimSpace(body) == "" {
		return store.ClientRequestComment{}, store.ErrInvalidClientRequest
	}
	stub.commented = true
	return store.ClientRequestComment{ID: 3, Body: body}, nil
}
func (stub *clientRequestStub) UpdateClientRequest(context.Context, store.Actor, int64, store.ClientRequestUpdate) (store.ClientRequest, error) {
	return store.ClientRequest{}, nil
}
func (stub *clientRequestStub) DeleteClientRequest(_ context.Context, _ store.Actor, id int64) error {
	for index, item := range stub.items {
		if item.ID == id {
			stub.items = append(stub.items[:index], stub.items[index+1:]...)
			stub.deleted = true
			return nil
		}
	}
	return store.ErrNotFound
}

func TestClientPortalGroupsNamespacedProjectsAndRoutes(t *testing.T) {
	s, requests, sessions := portalTestServer()
	key := "MO5/PROMEAI"
	sessions.projects = []store.Project{
		{Key: "KAZERLELUTIN/BENTO", Name: "Ben-to", Role: "read"},
		{Key: "KAZERLELUTIN/HTB", Name: "HTB", Role: "write"},
		{Key: "MO5/ANBY", Name: "ANBY", Role: "admin"},
		{Key: key, Name: "Promeai", Role: "read"},
	}
	requests.project = key
	s.stories = &clientStoryStub{project: key, stories: []store.ClientStory{{Ref: key + "-1", Project: key, Title: "Story namespacée", Published: true}}}

	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("portal home: %d %s", w.Code, body)
	}
	for _, want := range []string{"KAZERLELUTIN", "MO5", `href="/portal/projects/MO5/PROMEAI"`, "Rôle : Écriture", "Rôle : Administration", `>PROMEAI · Rôle : Lecture</span>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("grouped portal is missing %q: %s", want, body)
		}
	}

	for _, path := range []string{"/portal/projects/MO5/PROMEAI", "/portal/stories/MO5/PROMEAI-1"} {
		w = httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("namespaced portal route %s: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/portal/projects/MO5/PROMEAI" && !strings.Contains(w.Body.String(), "Votre rôle : Lecture") {
			t.Fatalf("project page does not show the member role: %s", w.Body.String())
		}
	}
	for _, path := range []string{"/portal/projects/MO5/PROMEAI/requests", "/portal/stories/MO5/PROMEAI-1/comments"} {
		w = httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, path, url.Values{"body": {"Test"}}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("namespaced portal action %s: got %d, want CSRF rejection", path, w.Code)
		}
	}
}

func portalTestServer() (*Server, *clientRequestStub, *browserSessionStub) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{actor: store.Actor{Subject: "zitadel-user", CredentialID: 1}, token: "valid", projects: []store.Project{{Key: "SITE", Name: "Site client", Role: "read"}}}
	requests := &clientRequestStub{items: []store.ClientRequest{{ID: 7, Project: "SITE", Title: "Une demande", Body: "# Sujet\n<script>alert(1)</script>\n[lien](javascript:alert(1))", Status: "received"}}}
	s.sessions, s.requests = sessions, requests
	s.stories = &clientStoryStub{}
	_ = s.SetBrowserLogin(browserLoginStub{}, []byte(strings.Repeat("x", 32)))
	return s, requests, sessions
}

func portalRequest(method, path string, values url.Values) *http.Request {
	body := ""
	if values != nil {
		body = values.Encode()
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: "valid"})
	if values != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

func TestClientPortalUsesProjectMembershipAndEscapesMarkdown(t *testing.T) {
	s, _, _ := portalTestServer()
	for _, path := range []string{"/portal", "/portal/projects/SITE", "/portal/requests/7"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`) {
			t.Fatalf("%s: portal page does not reference the favicon", path)
		}
		if !strings.Contains(w.Body.String(), `<span class="cursor" aria-hidden="true">_</span>`) {
			t.Fatalf("%s: portal page does not render the animated wordmark cursor", path)
		}
		if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatalf("%s: response may be cached", path)
		}
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/requests/7", nil))
	if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "href=\"javascript:") || !strings.Contains(w.Body.String(), "<h2>Sujet</h2>") {
		t.Fatalf("unsafe or missing markdown: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/projects/OTHER", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("unauthorized project returned %d", w.Code)
	}
}

func TestClientPortalFormsRequireSessionCSRF(t *testing.T) {
	s, requests, sessions := portalTestServer()
	for _, path := range []string{"/portal/projects/SITE/requests", "/portal/requests/7/comments", "/portal/requests/7/delete", "/portal/invitations"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, path, url.Values{"title": {"Titre"}, "body": {"Texte"}, "code": {"invite"}}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: missing CSRF returned %d", path, w.Code)
		}
	}
	if requests.created || requests.commented || requests.deleted {
		t.Fatal("form action ran without CSRF")
	}
	csrf := s.portalCSRF(portalRequest(http.MethodGet, "/portal", nil).WithContext(context.WithValue(context.Background(), browserSessionTokenKey{}, "valid")))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/projects/SITE/requests", url.Values{"csrf": {csrf}, "title": {"Titre"}, "body": {"Texte"}}))
	if w.Code != http.StatusSeeOther || !requests.created {
		t.Fatalf("request creation: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/invitations", url.Values{"csrf": {csrf}, "code": {"invite"}}))
	if w.Code != http.StatusSeeOther || sessions.acceptedCode != "invite" {
		t.Fatalf("invitation acceptance: %d", w.Code)
	}
}

func TestClientPortalListsAndDeletesPendingOrRejectedRequests(t *testing.T) {
	s, requests, _ := portalTestServer()
	requests.items = []store.ClientRequest{
		{ID: 7, Project: "SITE", Title: "En attente", Status: "received"},
		{ID: 8, Project: "SITE", Title: "Rejetée", Status: "rejected"},
		{ID: 9, Project: "SITE", Title: "En cours", Status: "in_progress"},
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/projects/SITE", nil))
	body := w.Body.String()
	for _, want := range []string{"Demandes proposées", "En attente", "Rejetée", "/portal/requests/7/delete", "/portal/requests/8/delete", `method="get"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("project page does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, "En cours") {
		t.Fatalf("project page must only list deletable requests: %s", body)
	}
	if strings.Contains(body, "Demandes en attente") || strings.Contains(body, "Demandes rejetées") {
		t.Fatalf("project page must rely on request status instead of list headings: %s", body)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/requests/8/delete", nil))
	body = w.Body.String()
	for _, want := range []string{"Supprimer la demande ?", "Vous êtes sur le point de supprimer définitivement « Rejetée ».", "Cette action est irréversible.", "Oui, supprimer la demande", "Annuler", `action="/portal/requests/8/delete" method="post"`, `name="confirmation"`} {
		if w.Code != http.StatusOK || !strings.Contains(body, want) {
			t.Fatalf("delete confirmation does not contain %q: %d %s", want, w.Code, body)
		}
	}
	if requests.deleted {
		t.Fatal("opening the delete confirmation must not delete the request")
	}
	csrf := s.portalCSRF(portalRequest(http.MethodGet, "/portal", nil).WithContext(context.WithValue(context.Background(), browserSessionTokenKey{}, "valid")))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/requests/8/delete", url.Values{"csrf": {csrf}}))
	if w.Code != http.StatusForbidden || requests.deleted {
		t.Fatalf("unconfirmed request deletion: %d %s", w.Code, w.Body.String())
	}
	confirmation := s.portalDeleteConfirmationToken(portalRequest(http.MethodGet, "/portal", nil).WithContext(context.WithValue(context.Background(), browserSessionTokenKey{}, "valid")), 8)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/requests/8/delete", url.Values{"csrf": {csrf}, "confirmation": {confirmation}}))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/portal/projects/SITE" || !requests.deleted {
		t.Fatalf("request deletion: %d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestClientPortalDoesNotConfirmDeletionOfHandledRequest(t *testing.T) {
	s, requests, _ := portalTestServer()
	requests.items[0].Status = "in_progress"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/requests/7/delete", nil))
	if w.Code != http.StatusForbidden || requests.deleted {
		t.Fatalf("handled request delete confirmation: %d %s", w.Code, w.Body.String())
	}
}

func TestPortalMarkdownRejectsUnsafeLinks(t *testing.T) {
	got := string(renderPortalMarkdown("<img src=x onerror=alert(1)>\n[bad](javascript:alert(1))\n[good](https://example.org/a?x=1&y=2)\n**bold**"))
	if strings.Contains(got, "<img") || strings.Contains(got, "javascript:") || !strings.Contains(got, `href="https://example.org/a?x=1&amp;y=2"`) || !strings.Contains(got, "<strong>bold</strong>") {
		t.Fatalf("unexpected markdown rendering: %s", got)
	}
}

func TestPortalMarkdownSupportsEscapedNewlines(t *testing.T) {
	got := string(renderPortalMarkdown(`Introduction\n\n## Périmètre\n- [ ] Première étape\n- [x] Dernière étape`))
	for _, want := range []string{"<p>Introduction</p>", "<h3>Périmètre</h3>", "<li>☐ Première étape</li>", "<li>☑ Dernière étape</li>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("escaped newlines did not render Markdown %q: %s", want, got)
		}
	}
}

func TestClientRequestOnlyLinksToPublishedStory(t *testing.T) {
	s, requests, _ := portalTestServer()
	linked := "SITE-12"
	requests.items[0].LinkedTicketRef = &linked
	for _, test := range []struct {
		stories  []store.ClientStory
		wantLink bool
	}{
		{nil, false},
		{[]store.ClientStory{{Ref: linked, Project: "SITE", Title: "Export data"}}, true},
	} {
		s.stories = &clientStoryStub{stories: test.stories}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/requests/7", nil))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), `href="/portal/stories/SITE-12"`) != test.wantLink {
			t.Fatalf("published=%v: %d %s", test.wantLink, w.Code, w.Body.String())
		}
	}
}
