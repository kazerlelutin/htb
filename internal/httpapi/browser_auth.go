package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kazerlelutin/htb/internal/store"
)

const (
	browserStateCookie   = "__Host-htb-login"
	browserSessionCookie = "__Host-htb-session"
	browserStateLifetime = 10 * time.Minute
	browserSessionTTL    = 30 * 24 * time.Hour
)

type browserSessionTokenKey struct{}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.browserLogin == nil {
		http.NotFound(w, r)
		return
	}
	state, err := randomBrowserValue(24)
	if err != nil {
		s.log.Error("generate browser login state", "error", err)
		http.Error(w, "Unable to start sign in.", http.StatusInternalServerError)
		return
	}
	verifier, err := randomBrowserValue(48)
	if err != nil {
		s.log.Error("generate PKCE verifier", "error", err)
		http.Error(w, "Unable to start sign in.", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(browserStateLifetime)
	http.SetCookie(w, s.loginStateCookie(state, verifier, expires))
	http.Redirect(w, r, s.browserLogin.AuthorizationURL(state, verifier), http.StatusFound)
}

func (s *Server) browserCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.browserLogin == nil {
		http.NotFound(w, r)
		return
	}
	state, verifier, err := s.readLoginState(r)
	if err != nil || !hmac.Equal([]byte(state), []byte(r.URL.Query().Get("state"))) || r.URL.Query().Get("code") == "" {
		s.clearLoginState(w)
		http.Error(w, "Unable to complete sign in. Please try again.", http.StatusBadRequest)
		return
	}
	s.clearLoginState(w)
	principal, err := s.browserLogin.Exchange(r.Context(), r.URL.Query().Get("code"), verifier)
	if err != nil {
		s.log.Warn("browser login failed")
		http.Error(w, "Unable to complete sign in. Please try again.", http.StatusUnauthorized)
		return
	}
	actor, err := s.sessions.BrowserActor(r.Context(), principal.Subject, principal.Name, principal.Email)
	if err != nil {
		if errors.Is(err, store.ErrForbidden) {
			http.Error(w, "This account does not have access to a project.", http.StatusForbidden)
			return
		}
		s.log.Error("resolve browser actor", "error", err)
		http.Error(w, "Unable to complete sign in. Please try again.", http.StatusInternalServerError)
		return
	}
	token, err := s.sessions.CreateWebSession(r.Context(), actor, browserSessionTTL)
	if err != nil {
		s.log.Error("create browser session", "error", err)
		http.Error(w, "Unable to complete sign in. Please try again.", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: browserSessionCookie, Value: token, Path: "/", Expires: time.Now().Add(browserSessionTTL), MaxAge: int(browserSessionTTL.Seconds()), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/portal", http.StatusSeeOther)
}

func (s *Server) browserAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.browserLogin == nil {
			http.NotFound(w, r)
			return
		}
		cookie, err := r.Cookie(browserSessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required", nil)
			return
		}
		actor, err := s.sessions.WebSessionActor(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication is required", nil)
			return
		}
		ctx := context.WithValue(r.Context(), actorKey{}, actor)
		ctx = context.WithValue(ctx, browserSessionTokenKey{}, cookie.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) browserProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.sessions.ListProjects(r.Context(), actor(r))
	if err != nil {
		s.log.Error("list browser projects", "error", err)
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to list projects", nil)
		return
	}
	type clientProject struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	items := make([]clientProject, 0, len(projects))
	for _, project := range projects {
		items = append(items, clientProject{Key: project.Key, Name: project.Name})
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]any{"projects": items})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.browserLogin == nil {
		http.NotFound(w, r)
		return
	}
	if cookie, err := r.Cookie(browserSessionCookie); err == nil {
		if err := s.sessions.RevokeWebSession(r.Context(), cookie.Value); err != nil {
			s.log.Error("revoke browser session", "error", err)
			http.Error(w, "Unable to sign out. Please try again.", http.StatusInternalServerError)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: browserSessionCookie, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) loginStateCookie(state, verifier string, expires time.Time) *http.Cookie {
	payload := strings.Join([]string{state, verifier, strconv.FormatInt(expires.Unix(), 10)}, ".")
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write([]byte(encoded))
	value := encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return &http.Cookie{Name: browserStateCookie, Value: value, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func (s *Server) readLoginState(r *http.Request) (string, string, error) {
	cookie, err := r.Cookie(browserStateCookie)
	if err != nil {
		return "", "", err
	}
	encoded, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return "", "", fmt.Errorf("invalid login state")
	}
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", "", err
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write([]byte(encoded))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return "", "", fmt.Errorf("invalid login state signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(string(payload), ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid login state payload")
	}
	expires, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().After(time.Unix(expires, 0)) {
		return "", "", fmt.Errorf("expired login state")
	}
	return parts[0], parts[1], nil
}

func (s *Server) clearLoginState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: browserStateCookie, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func randomBrowserValue(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
