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
	created   bool
	commented bool
}

func (stub *clientRequestStub) CreateClientRequest(_ context.Context, _ store.Actor, project, title, body string) (store.ClientRequest, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return store.ClientRequest{}, store.ErrInvalidClientRequest
	}
	stub.created = true
	return store.ClientRequest{ID: 7, Project: project, Title: title, Body: body}, nil
}
func (stub *clientRequestStub) ListClientRequests(_ context.Context, _ store.Actor, project string) ([]store.ClientRequest, error) {
	if project != "SITE" {
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

func portalTestServer() (*Server, *clientRequestStub, *browserSessionStub) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{actor: store.Actor{Subject: "zitadel-user", CredentialID: 1}, token: "valid", projects: []store.Project{{Key: "SITE", Name: "Site client"}}}
	requests := &clientRequestStub{items: []store.ClientRequest{{ID: 7, Project: "SITE", Title: "Une demande", Body: "# Sujet\n<script>alert(1)</script>\n[lien](javascript:alert(1))", Status: "received"}}}
	s.sessions, s.requests = sessions, requests
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
	for _, path := range []string{"/portal/projects/SITE/requests", "/portal/requests/7/comments", "/portal/invitations"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, path, url.Values{"title": {"Titre"}, "body": {"Texte"}, "code": {"invite"}}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: missing CSRF returned %d", path, w.Code)
		}
	}
	if requests.created || requests.commented {
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

func TestPortalMarkdownRejectsUnsafeLinks(t *testing.T) {
	got := string(renderPortalMarkdown("<img src=x onerror=alert(1)>\n[bad](javascript:alert(1))\n[good](https://example.org/a?x=1&y=2)\n**bold**"))
	if strings.Contains(got, "<img") || strings.Contains(got, "javascript:") || !strings.Contains(got, `href="https://example.org/a?x=1&amp;y=2"`) || !strings.Contains(got, "<strong>bold</strong>") {
		t.Fatalf("unexpected markdown rendering: %s", got)
	}
}
