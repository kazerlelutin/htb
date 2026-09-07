package auth

import (
	"net/http/httptest"
	"testing"
)

func TestBearer(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer signed-token")
	got, err := Bearer(r)
	if err != nil || got != "signed-token" {
		t.Fatalf("got %q, %v", got, err)
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
