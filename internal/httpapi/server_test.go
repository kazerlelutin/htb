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
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "v1.2.3", "https://github.example/releases?x=1&y=2", slog.Default())
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
	}
}

func TestDeviceConfigurationIsPublicWithoutSecrets(t *testing.T) {
	s := New(&store.Store{}, nil, auth.DeviceConfig{Issuer: "https://id.example", ClientID: "cli-id", Audience: "project-id"}, "dev", "", slog.Default())
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
	s := New(&store.Store{}, nil, auth.DeviceConfig{}, "dev", "", slog.Default())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/invite/code", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", w.Code)
	}
}
