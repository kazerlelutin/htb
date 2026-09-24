package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/store"
)

type browserLoginStub struct {
	principal auth.Principal
	err       error
}

func (stub browserLoginStub) AuthorizationURL(state, verifier string) string {
	return "https://id.example/authorize?state=" + state + "&verifier=" + verifier
}
func (stub browserLoginStub) Exchange(context.Context, string, string) (auth.Principal, error) {
	return stub.principal, stub.err
}

type browserSessionStub struct {
	actor    store.Actor
	token    string
	revoked  bool
	projects []store.Project
}

func (stub *browserSessionStub) BrowserActor(_ context.Context, subject string) (store.Actor, error) {
	if subject != stub.actor.Subject {
		return store.Actor{}, store.ErrForbidden
	}
	return stub.actor, nil
}

func (stub *browserSessionStub) CreateWebSession(context.Context, store.Actor, time.Duration) (string, error) {
	return stub.token, nil
}
func (stub *browserSessionStub) WebSessionActor(_ context.Context, token string) (store.Actor, error) {
	if token != stub.token {
		return store.Actor{}, errors.New("unknown session")
	}
	return stub.actor, nil
}
func (stub *browserSessionStub) RevokeWebSession(_ context.Context, token string) error {
	if token == stub.token {
		stub.revoked = true
	}
	return nil
}
func (stub *browserSessionStub) ListProjects(context.Context, store.Actor) ([]store.Project, error) {
	return stub.projects, nil
}

func TestPublicPages(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "https://github.example/releases?x=1&y=2", slog.Default())
	for _, path := range []string{"/", "/downloads?lang=en", "/commands?lang=en", "/mentions-legales", "/cgu", "/privacy"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got status %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`) {
			t.Fatalf("%s: public page does not reference the favicon", path)
		}
		if !strings.Contains(w.Body.String(), `<span class="cursor" aria-hidden="true">_</span>`) {
			t.Fatalf("%s: public page does not render the isolated wordmark cursor", path)
		}
		if path == "/downloads?lang=en" && !strings.Contains(w.Body.String(), "x=1&amp;y=2") {
			t.Fatalf("download link is not escaped: %s", w.Body.String())
		}
		if path == "/downloads?lang=en" && !strings.Contains(w.Body.String(), "/latest/download/install.sh") {
			t.Fatalf("download page does not link to the installer: %s", w.Body.String())
		}
		if path == "/downloads?lang=en" && (!strings.Contains(w.Body.String(), "/commands?lang=en") || strings.Contains(w.Body.String(), `class="command-guide"`)) {
			t.Fatalf("download page does not focus on installation and link to the command guide: %s", w.Body.String())
		}
		if path == "/commands?lang=en" && (!strings.Contains(w.Body.String(), `class="command-guide"`) || !strings.Contains(w.Body.String(), `class="command-entry"`) || strings.Contains(w.Body.String(), "/latest/download/install.sh")) {
			t.Fatalf("command page does not focus on readable command entries: %s", w.Body.String())
		}
		if path == "/commands?lang=en" && (!strings.Contains(w.Body.String(), `<span class="command-option">[--project KEY]</span>`) || !strings.Contains(w.Body.String(), `class="copy-command"`) || !strings.Contains(w.Body.String(), `data-copy-command=`)) {
			t.Fatalf("command page does not highlight options and provide copy buttons: %s", w.Body.String())
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "analytics.ben-to.fr") {
			t.Fatalf("%s: public CSP does not permit the consented analytics script", path)
		}
		if strings.Contains(w.Body.String(), "https://analytics.ben-to.fr/script.js") {
			t.Fatalf("%s: analytics must not be loaded in the initial HTML", path)
		}
		if path == "/" && (!strings.Contains(w.Body.String(), `id="simulateur"`) || !strings.Contains(w.Body.String(), `data-simulator-tab="simulator-track"`)) {
			t.Fatalf("home page is missing the accessible simulator: %s", w.Body.String())
		}
		if path == "/" && !strings.Contains(w.Body.String(), `href="/commands">Lire le guide complet des commandes`) {
			t.Fatalf("home page does not link to the command guide: %s", w.Body.String())
		}
		if path == "/commands?lang=en" {
			for _, text := range []string{
				`lang="en"`, "Command guide", "How to use it", "htb version", "htb config set-server URL", "htb auth login", "htb auth status",
				"htb project list", "htb project create --key KEY --name NAME", "htb project use KEY", "htb feature create --key KEY --name NAME",
				"htb ticket create --title TITLE", "htb ticket list", "htb ticket show REF", "htb ticket update --version N", "htb ticket comment REF TEXT", "htb ticket claim REF", "htb ticket versions REF", "htb ticket restore --version N REF REVISION",
				"htb invite create", "htb invite accept CODE",
			} {
				if !strings.Contains(w.Body.String(), text) {
					t.Fatalf("download page is missing %q: %s", text, w.Body.String())
				}
			}
			if strings.Contains(w.Body.String(), "Server version") || strings.Contains(w.Body.String(), "v1.2.3") {
				t.Fatalf("download page exposes server version: %s", w.Body.String())
			}
			for _, explanation := range []string{
				"Pass a command path", "KEY identifies the project", "A technical task must use --parent", "concurrent changes are not overwritten", "Choose the member role and an expiry time",
			} {
				if !strings.Contains(w.Body.String(), explanation) {
					t.Fatalf("download page is missing an explanation: %q", explanation)
				}
			}
		}
	}
}

func TestCommandSyntaxHighlightsOptionalArgumentsWithoutChangingText(t *testing.T) {
	got := string(commandSyntaxHTML(`htb ticket list [--project KEY] [--json|--csv]`))
	want := `htb ticket list <span class="command-option">[--project KEY]</span> <span class="command-option">[--json|--csv]</span>`
	if got != want {
		t.Fatalf("unexpected command syntax: got %q, want %q", got, want)
	}
	if got := string(commandSyntaxHTML(`htb command [--value <unsafe>]`)); !strings.Contains(got, `&lt;unsafe&gt;`) {
		t.Fatalf("command syntax does not escape option content: %q", got)
	}
}

func TestPublicFavicon(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/favicon.svg", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("unexpected content type: %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("unexpected cache control: %s", w.Header().Get("Cache-Control"))
	}
	if !strings.Contains(w.Body.String(), `<svg`) || !strings.Contains(w.Body.String(), `fill:#c8ff6b`) {
		t.Fatalf("unexpected favicon content: %s", w.Body.String())
	}
}

func TestHomePageDoesNotNameAuthenticationProvider(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	for _, test := range []struct {
		path string
		copy string
	}{
		{"/", "Invitez les bonnes personnes. Les droits de projet sont explicites et chaque changement reste traçable."},
		{"/?lang=en", "Invite the right people. Keep project permissions explicit and let every change remain traceable."},
	} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, test.path, nil))
		if !strings.Contains(w.Body.String(), test.copy) {
			t.Fatalf("%s: home page is missing marketing copy %q", test.path, test.copy)
		}
		if strings.Contains(w.Body.String(), "Zitadel") {
			t.Fatalf("%s: home page names the authentication provider", test.path)
		}
	}
}

func TestPublicAssetsDescribeConsentWithoutPreloadingAnalytics(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/public.js", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "htb.analytics-consent") || !strings.Contains(w.Body.String(), "dataset.websiteId") || !strings.Contains(w.Body.String(), "data-simulator-tab") || !strings.Contains(w.Body.String(), "data-copy-command") || !strings.Contains(w.Body.String(), "navigator.clipboard") {
		t.Fatalf("unexpected consent script: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("unexpected content type: %s", w.Header().Get("Content-Type"))
	}
}

func TestPublicStylesAnimateCursorWithReducedMotionFallback(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/public.css", nil))
	for _, rule := range []string{".wordmark .cursor", "@keyframes cursor-blink", "prefers-reduced-motion: reduce", "animation: none", ".simulator-tabs", ".simulator-panel[hidden]", ".command-guide", ".command-entry", ".command-option", ".copy-command"} {
		if !strings.Contains(w.Body.String(), rule) {
			t.Fatalf("public styles are missing %q", rule)
		}
	}
}

func TestDeviceConfigurationIsPublicWithoutSecrets(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{Issuer: "https://id.example", ClientID: "cli-id", Audience: "project-id"}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/device-config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "cli-id") || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("unexpected configuration: %s", w.Body.String())
	}
}

func TestBrowserInvitationRouteDoesNotExist(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/invite/code", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", w.Code)
	}
}

func TestBrowserLoginCreatesASignedShortLivedPKCEState(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/login", nil))
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "https://id.example/authorize?state=") {
		t.Fatalf("unexpected login redirect: %d %q", w.Code, w.Header().Get("Location"))
	}
	result := w.Result()
	if len(result.Cookies()) != 1 {
		t.Fatalf("login must set one state cookie: %#v", result.Cookies())
	}
	cookie := result.Cookies()[0]
	if cookie.Name != browserStateCookie || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("unsafe login state cookie: %#v", cookie)
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	request.AddCookie(cookie)
	state, verifier, err := s.readLoginState(request)
	if err != nil || state == "" || verifier == "" || !strings.Contains(w.Header().Get("Location"), "state="+state) {
		t.Fatalf("state cookie is not valid: state=%q verifier=%q err=%v", state, verifier, err)
	}
}

func TestClientEntryPointAppearsOnlyWhenBrowserLoginIsConfigured(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(w.Body.String(), `href="/login"`) {
		t.Fatal("client entry point is visible without browser login")
	}
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), `href="/login">Espace client`) {
		t.Fatal("configured browser login has no visible entry point")
	}
}

func TestBrowserCallbackRejectsTamperedState(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=one-time-code&state=altered", nil)
	request.AddCookie(s.loginStateCookie("expected", "verifier", time.Now().Add(time.Minute)))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "verifier") {
		t.Fatalf("tampered callback must fail without exposing secrets: %d %s", w.Code, w.Body.String())
	}
}

func TestBrowserCallbackCreatesRestrictedSessionForExistingMember(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{token: "opaque-session", actor: store.Actor{CredentialID: 8, UserID: 4, Subject: "zitadel-user"}}
	if err := s.SetBrowserLogin(browserLoginStub{principal: auth.Principal{Subject: "zitadel-user", Email: "user@example.test"}}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	s.sessions = sessions
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=one-time-code&state=expected", nil)
	request.AddCookie(s.loginStateCookie("expected", "verifier", time.Now().Add(time.Minute)))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/portal" {
		t.Fatalf("valid callback did not complete: %d %s", w.Code, w.Body.String())
	}
	var found bool
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == browserSessionCookie {
			found = true
			if cookie.Value != sessions.token || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != int(browserSessionTTL.Seconds()) {
				t.Fatalf("session cookie is unsafe: %#v", cookie)
			}
		}
	}
	if !found {
		t.Fatal("callback did not set a browser session")
	}
}

func TestBrowserCallbackRejectsExpiredState(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=one-time-code&state=expected", nil)
	request.AddCookie(s.loginStateCookie("expected", "verifier", time.Now().Add(-time.Minute)))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expired callback state accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestBrowserSessionOnlyAuthenticatesPortalRoutes(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{token: "browser-token", actor: store.Actor{CredentialID: 8, UserID: 4, Subject: "zitadel-user"}, projects: []store.Project{{Key: "ACME", Name: "Acme", Description: "internal"}}}
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	s.sessions = sessions
	request := httptest.NewRequest(http.MethodGet, "/portal/api/projects", nil)
	request.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: sessions.token})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"key":"ACME"`) || strings.Contains(w.Body.String(), "internal") {
		t.Fatalf("browser session did not receive public project summary: %d %s", w.Code, w.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: sessions.token})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("browser cookie must not authenticate internal API: %d %s", w.Code, w.Body.String())
	}
}

func TestPortalDisplaysAuthorizedProjectsAndEscapesTheirNames(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{token: "browser-token", actor: store.Actor{CredentialID: 8, UserID: 4, Subject: "zitadel-user"}, projects: []store.Project{{Key: "ACME", Name: "<script>alert(1)</script>"}}}
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	s.sessions = sessions
	request := httptest.NewRequest(http.MethodGet, "/portal", nil)
	request.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: sessions.token})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "&lt;script&gt;alert(1)&lt;/script&gt;") || strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
		t.Fatalf("portal did not escape project name: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("portal response is cacheable: %s", w.Header().Get("Cache-Control"))
	}
}

func TestBrowserLogoutRevokesAndClearsTheSession(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	sessions := &browserSessionStub{token: "browser-token"}
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("01234567890123456789012345678901")); err != nil {
		t.Fatal(err)
	}
	s.sessions = sessions
	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	request.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: sessions.token})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusSeeOther || !sessions.revoked {
		t.Fatalf("logout did not revoke session: %d %#v", w.Code, sessions)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != browserSessionCookie || cookies[0].MaxAge >= 0 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("logout did not clear secure session cookie: %#v", cookies)
	}
}

func TestSetBrowserLoginRejectsShortStateKey(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "", slog.Default())
	if err := s.SetBrowserLogin(browserLoginStub{}, []byte("too-short")); err == nil {
		t.Fatal("short browser-login state key was accepted")
	}
}
