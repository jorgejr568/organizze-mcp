package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// errInvalidClient signals an RFC 6749 §5.2 invalid_client. Handlers map it
// to a 400 with error=invalid_client; we don't expose the specific reason
// (missing secret vs. wrong secret vs. unknown client) to avoid leaking
// which client IDs exist.
var errInvalidClient = errors.New("invalid_client")

// authenticateClient resolves and authenticates the OAuth client for a token
// or revoke request. It accepts client_secret_basic (preferred) and
// client_secret_post (form fields). Clients whose stored ClientSecretHash is
// nil are public clients (registered before this server issued secrets, or
// future explicit-none registrations) and are accepted without a secret —
// PKCE is the binding mechanism for those, enforced by the caller.
//
// The form on r must already be parsed (r.ParseForm() called by the handler).
func authenticateClient(ctx context.Context, store storage.Store, r *http.Request) (string, error) {
	basicID, basicSecret, hasBasic := r.BasicAuth()
	formID := r.PostForm.Get("client_id")
	formSecret := r.PostForm.Get("client_secret")

	var clientID, presentedSecret string
	switch {
	case hasBasic:
		clientID = basicID
		presentedSecret = basicSecret
		// If the form also carries a client_id, RFC 6749 §2.3.1 says only
		// one method may be used per request. Tolerate a duplicate that
		// matches, reject a contradiction.
		if formID != "" && formID != basicID {
			return "", errInvalidClient
		}
	default:
		clientID = formID
		presentedSecret = formSecret
	}
	if clientID == "" {
		return "", errInvalidClient
	}

	client, err := store.GetClient(ctx, clientID)
	if err != nil {
		return "", errInvalidClient
	}

	if client.ClientSecretHash == nil {
		// Public client — PKCE is the binding factor (verified by the
		// authorization-code grant path). No secret expected; presence
		// of a stray client_secret is ignored.
		return clientID, nil
	}

	if presentedSecret == "" {
		return "", errInvalidClient
	}
	if subtle.ConstantTimeCompare(storage.HashToken(presentedSecret), client.ClientSecretHash) != 1 {
		return "", errInvalidClient
	}
	return clientID, nil
}
