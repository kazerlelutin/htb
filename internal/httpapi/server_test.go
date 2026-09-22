package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/store"
)

func TestPublicPages(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "https://github.example/releases?x=1&y=2", slog.Default())
	for _, path := range []string{"/", "/downloads?lang=en", "/mentions-legales", "/cgu", "/privacy"} {
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
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "analytics.ben-to.fr") {
			t.Fatalf("%s: public CSP does not permit the consented analytics script", path)
		}
		if strings.Contains(w.Body.String(), "https://analytics.ben-to.fr/script.js") {
			t.Fatalf("%s: analytics must not be loaded in the initial HTML", path)
		}
		if path == "/" && (!strings.Contains(w.Body.String(), `id="simulateur"`) || !strings.Contains(w.Body.String(), `data-simulator-tab="simulator-track"`)) {
			t.Fatalf("home page is missing the accessible simulator: %s", w.Body.String())
		}
		if path == "/downloads?lang=en" {
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
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "htb.analytics-consent") || !strings.Contains(w.Body.String(), "dataset.websiteId") || !strings.Contains(w.Body.String(), "data-simulator-tab") {
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
	for _, rule := range []string{".wordmark .cursor", "@keyframes cursor-blink", "prefers-reduced-motion: reduce", "animation: none", ".simulator-tabs", ".simulator-panel[hidden]"} {
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
