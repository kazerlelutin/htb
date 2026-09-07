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
	for _, path := range []string{"/", "/downloads"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got status %d", path, w.Code)
		}
		if path == "/downloads" && !strings.Contains(w.Body.String(), "x=1&amp;y=2") {
			t.Fatalf("download link is not escaped: %s", w.Body.String())
		}
		if path == "/downloads" && !strings.Contains(w.Body.String(), "/latest/download/install.sh") {
			t.Fatalf("download page does not link to the installer: %s", w.Body.String())
		}
		if path == "/downloads" {
			for _, text := range []string{
				`lang="en"`, "Command reference", "htb version", "htb config set-server URL", "htb auth login", "htb auth status",
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
