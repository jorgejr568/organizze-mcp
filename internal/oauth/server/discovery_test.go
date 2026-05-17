package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedResourceMetadata(t *testing.T) {
	h := New(Config{PublicURL: "https://mcp.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["resource"] != "https://mcp.example.com/mcp" {
		t.Errorf("resource = %v", got["resource"])
	}
	servers, _ := got["authorization_servers"].([]any)
	if len(servers) != 1 || servers[0] != "https://mcp.example.com" {
		t.Errorf("authorization_servers = %v", servers)
	}
}

func TestAuthorizationServerMetadata(t *testing.T) {
	h := New(Config{PublicURL: "https://mcp.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for k, want := range map[string]string{
		"issuer":                 "https://mcp.example.com",
		"authorization_endpoint": "https://mcp.example.com/oauth/authorize",
		"token_endpoint":         "https://mcp.example.com/oauth/token",
		"registration_endpoint":  "https://mcp.example.com/oauth/register",
		"revocation_endpoint":    "https://mcp.example.com/oauth/revoke",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %s", k, got[k], want)
		}
	}
	codeMethods, _ := got["code_challenge_methods_supported"].([]any)
	if len(codeMethods) != 1 || codeMethods[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v", codeMethods)
	}
	authMethods, _ := got["token_endpoint_auth_methods_supported"].([]any)
	if len(authMethods) != 1 || authMethods[0] != "none" {
		t.Errorf("token_endpoint_auth_methods_supported = %v", authMethods)
	}
}
