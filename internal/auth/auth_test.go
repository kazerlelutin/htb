package auth

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func TestBearer(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer signed-token")
	got, err := Bearer(r)
	if err != nil || got != "signed-token" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestBrowserAuthorizationUsesPKCE(t *testing.T) {
	authenticator := &BrowserAuthenticator{config: oauth2.Config{
		ClientID:    "web-client",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://id.example/authorize"},
		RedirectURL: "https://htb.example/auth/callback",
	}}
	authorize, err := url.Parse(authenticator.AuthorizationURL("random-state", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"))
	if err != nil {
		t.Fatal(err)
	}
	query := authorize.Query()
	if query.Get("state") != "random-state" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("authorization request does not use PKCE S256: %s", authorize.String())
	}
}

func TestBearerRejectsMissingHeader(t *testing.T) {
	if _, err := Bearer(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestHasRoleSupportsStandardZitadelClaim(t *testing.T) {
	claim := []any{map[string]any{"superadmin": map[string]any{"123": "association.example"}}}
	if !hasRole(claim, "superadmin") {
		t.Fatal("standard nested Zitadel role claim was not recognized")
	}
	if hasRole(claim, "unknown") {
		t.Fatal("unexpected role")
	}
}
