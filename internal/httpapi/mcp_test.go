package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/store"
)

type mcpVerifierStub struct {
	principal auth.Principal
	err       error
}

func (stub mcpVerifierStub) Verify(context.Context, string) (auth.Principal, error) {
	return stub.principal, stub.err
}

type mcpStoreStub struct {
	actor      store.Actor
	projects   []store.Project
	tickets    []store.Ticket
	ticket     store.Ticket
	resolveErr error
	queryErr   error
}

func (stub *mcpStoreStub) ResolveActor(_ context.Context, subject, _, _ string, _ bool) (store.Actor, error) {
	if stub.resolveErr != nil {
		return store.Actor{}, stub.resolveErr
	}
	if subject != stub.actor.Subject {
		return store.Actor{}, store.ErrForbidden
	}
	return stub.actor, nil
}

func (stub *mcpStoreStub) ListProjects(context.Context, store.Actor) ([]store.Project, error) {
	return stub.projects, stub.queryErr
}

func (stub *mcpStoreStub) ListTickets(context.Context, store.Actor, string, store.TicketFilter) ([]store.Ticket, error) {
	return stub.tickets, stub.queryErr
}

func (stub *mcpStoreStub) GetTicket(context.Context, store.Actor, string) (store.Ticket, error) {
	return stub.ticket, stub.queryErr
}

func newMCPTestServer(data *mcpStoreStub) *Server {
	server := New(&store.Store{}, mcpVerifierStub{principal: auth.Principal{Subject: "person-1", Name: "Ada", Email: "ada@example.org"}}, auth.DeviceConfig{Issuer: "https://id.example.org"}, "", slog.Default())
	server.mcp = data
	server.SetPublicURL("https://tickets.example.org")
	server.SetMCPEnabled(true)
	return server
}

func performMCPRequest(t *testing.T, server *Server, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func performModernMCPRequest(t *testing.T, server *Server, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func TestMCPProtectedResourceMetadataAndAuthenticationChallenge(t *testing.T) {
	server := newMCPTestServer(&mcpStoreStub{actor: store.Actor{Subject: "person-1"}})

	metadata := httptest.NewRecorder()
	server.Handler().ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if metadata.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, want 200", metadata.Code)
	}
	var document map[string]any
	if err := json.NewDecoder(metadata.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if document["resource"] != "https://tickets.example.org/mcp" {
		t.Fatalf("resource = %#v", document["resource"])
	}
	if document["authorization_servers"].([]any)[0] != "https://id.example.org" {
		t.Fatalf("authorization_servers = %#v", document["authorization_servers"])
	}

	unauthenticated := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthenticated.Code)
	}
	if !strings.Contains(unauthenticated.Header().Get("WWW-Authenticate"), "oauth-protected-resource") {
		t.Fatalf("missing OAuth challenge: %q", unauthenticated.Header().Get("WWW-Authenticate"))
	}
}

func TestMCPIsDisabledUntilExplicitlyEnabled(t *testing.T) {
	server := New(&store.Store{}, mcpVerifierStub{principal: auth.Principal{Subject: "person-1"}}, auth.DeviceConfig{Issuer: "https://id.example.org"}, "", slog.Default())
	server.mcp = &mcpStoreStub{actor: store.Actor{Subject: "person-1"}}

	metadata := httptest.NewRecorder()
	server.Handler().ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if metadata.Code != http.StatusNotFound {
		t.Fatalf("disabled metadata status = %d, want 404", metadata.Code)
	}
	response := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "access-token")
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled MCP status = %d, want 404", response.Code)
	}
}

func TestMCPInitializesAndDescribesReadOnlyTools(t *testing.T) {
	server := newMCPTestServer(&mcpStoreStub{actor: store.Actor{Subject: "person-1"}})

	initialized := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, "access-token")
	if initialized.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200: %s", initialized.Code, initialized.Body.String())
	}
	if got := initialized.Header().Get("MCP-Protocol-Version"); got != mcpLegacyProtocolVersion {
		t.Fatalf("protocol header = %q", got)
	}
	var initializeResponse struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			Instructions    string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.NewDecoder(initialized.Body).Decode(&initializeResponse); err != nil {
		t.Fatal(err)
	}
	if initializeResponse.Result.ProtocolVersion != mcpLegacyProtocolVersion {
		t.Fatalf("protocol version = %q", initializeResponse.Result.ProtocolVersion)
	}
	if !strings.Contains(initializeResponse.Result.Instructions, "plan and propose") || !strings.Contains(initializeResponse.Result.Instructions, "Do not create") {
		t.Fatalf("MCP instructions do not preserve the planning boundary: %q", initializeResponse.Result.Instructions)
	}

	tools := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "access-token")
	if tools.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d", tools.Code)
	}
	if body := tools.Body.String(); !strings.Contains(body, `"htb_list_projects"`) || !strings.Contains(body, `"readOnlyHint":true`) || !strings.Contains(body, `"_meta":{"securitySchemes":[{"scopes":["openid","profile","email"],"type":"oauth2"}]}`) || strings.Contains(body, `"htb_create_ticket"`) {
		t.Fatalf("unexpected tools response: %s", body)
	}
}

func TestMCPModernDiscoveryExposesTools(t *testing.T) {
	server := newMCPTestServer(&mcpStoreStub{actor: store.Actor{Subject: "person-1"}})

	discovery := performModernMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`, "access-token")
	if discovery.Code != http.StatusOK {
		t.Fatalf("server/discover status = %d: %s", discovery.Code, discovery.Body.String())
	}
	if got := discovery.Header().Get("MCP-Protocol-Version"); got != mcpProtocolVersion {
		t.Fatalf("protocol header = %q", got)
	}
	if body := discovery.Body.String(); !strings.Contains(body, `"supportedVersions":["2026-07-28"]`) || !strings.Contains(body, `"io.modelcontextprotocol/serverInfo"`) || !strings.Contains(body, `"tools":{"listChanged":false}`) {
		t.Fatalf("unexpected discovery response: %s", body)
	}

	tools := performModernMCPRequest(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`, "access-token")
	if tools.Code != http.StatusOK || !strings.Contains(tools.Body.String(), `"htb_list_projects"`) || !strings.Contains(tools.Body.String(), `"cacheScope":"private"`) {
		t.Fatalf("modern tools response = %d %s", tools.Code, tools.Body.String())
	}
}

func TestMCPCanUseDedicatedDCRAudienceVerifier(t *testing.T) {
	server := newMCPTestServer(&mcpStoreStub{actor: store.Actor{Subject: "person-1"}})
	server.verifier = mcpVerifierStub{err: errors.New("primary HTB audience is not accepted")}
	server.SetMCPVerifier(mcpVerifierStub{principal: auth.Principal{Subject: "person-1", Name: "Ada", Email: "ada@example.org"}})

	response := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "dcr-access-token")
	if response.Code != http.StatusOK {
		t.Fatalf("dedicated MCP verifier status = %d: %s", response.Code, response.Body.String())
	}
}

func TestMCPReadsOnlyAccessibleTicketData(t *testing.T) {
	data := &mcpStoreStub{
		actor:    store.Actor{Subject: "person-1"},
		projects: []store.Project{{Key: "SITE", Name: "Website"}},
		tickets:  []store.Ticket{{Ref: "SITE-3", Project: "SITE", Title: "Ship MCP"}},
		ticket:   store.Ticket{Ref: "SITE-3", Project: "SITE", Title: "Ship MCP"},
	}
	server := newMCPTestServer(data)

	projects := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"htb_list_projects","arguments":{}}}`, "access-token")
	if projects.Code != http.StatusOK || !strings.Contains(projects.Body.String(), `"SITE"`) {
		t.Fatalf("projects response = %d %s", projects.Code, projects.Body.String())
	}
	tickets := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"htb_list_tickets","arguments":{"project":"SITE"}}}`, "access-token")
	if tickets.Code != http.StatusOK || !strings.Contains(tickets.Body.String(), `"SITE-3"`) {
		t.Fatalf("tickets response = %d %s", tickets.Code, tickets.Body.String())
	}
	invalid := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"htb_list_projects","arguments":{"unexpected":true}}}`, "access-token")
	if invalid.Code != http.StatusOK || !strings.Contains(invalid.Body.String(), `"isError":true`) || strings.Contains(invalid.Body.String(), "unexpected") {
		t.Fatalf("invalid tool response = %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestMCPDoesNotExposeStoreErrors(t *testing.T) {
	server := newMCPTestServer(&mcpStoreStub{actor: store.Actor{Subject: "person-1"}, queryErr: errors.New("database password: secret")})
	response := performMCPRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"htb_list_projects","arguments":{}}}`, "access-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body, _ := io.ReadAll(response.Body)
	if strings.Contains(string(body), "database password") || !strings.Contains(string(body), "unavailable") {
		t.Fatalf("store error leaked: %s", body)
	}
}
