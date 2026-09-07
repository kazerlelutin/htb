package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Principal struct {
	Subject, Name, Email string
	Superadmin           bool
}

// DeviceConfig contains public OIDC client settings that the CLI needs to
// start Zitadel Device Authorization. It deliberately contains no secret.
type DeviceConfig struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
	Audience string `json:"audience"`
}
type Verifier interface {
	Verify(context.Context, string) (Principal, error)
}

// ZitadelVerifier validates signed access tokens against the issuer JWKS.
type ZitadelVerifier struct {
	verifier                        *oidc.IDTokenVerifier
	superadminClaim, superadminRole string
}

func NewZitadelVerifier(ctx context.Context, issuer, audience, superadminClaim, superadminRole string) (*ZitadelVerifier, error) {
	if issuer == "" || audience == "" {
		return nil, fmt.Errorf("Zitadel issuer and audience are required")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover Zitadel issuer: %w", err)
	}
	if superadminClaim == "" {
		superadminClaim = "urn:zitadel:iam:org:project:roles"
	}
	if superadminRole == "" {
		superadminRole = "superadmin"
	}
	return &ZitadelVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience}), superadminClaim: superadminClaim, superadminRole: superadminRole}, nil
}

func (v *ZitadelVerifier) Verify(ctx context.Context, raw string) (Principal, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return Principal{}, err
	}
	var claims struct {
		Subject   string `json:"sub"`
		Name      string `json:"name"`
		Preferred string `json:"preferred_username"`
		Email     string `json:"email"`
	}
	if err := token.Claims(&claims); err != nil {
		return Principal{}, err
	}
	var all map[string]any
	if err := token.Claims(&all); err != nil {
		return Principal{}, err
	}
	p := Principal{Subject: claims.Subject, Name: claims.Name, Email: claims.Email}
	if p.Name == "" {
		p.Name = claims.Preferred
	}
	p.Superadmin = hasRole(all[v.superadminClaim], v.superadminRole)
	return p, nil
}

// hasRole accepts Zitadel's standard nested claim shape, while retaining
// compatibility with a flat claim configured by an existing installation.
func hasRole(value any, role string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed[role]; ok {
			return true
		}
		for _, child := range typed {
			if hasRole(child, role) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if hasRole(child, role) {
				return true
			}
		}
	case string:
		return typed == role
	}
	return false
}

func Bearer(r *http.Request) (string, error) {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") || len(strings.TrimSpace(value[7:])) == 0 {
		return "", fmt.Errorf("missing bearer token")
	}
	return strings.TrimSpace(value[7:]), nil
}
