package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

//go:embed templates/login.html
var loginFS embed.FS

var loginTpl = template.Must(template.ParseFS(loginFS, "templates/login.html"))

type loginViewModel struct {
	ClientID            string
	ClientName          string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	CSRF                string
	Error               string
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.authorizeGET(w, r)
	case http.MethodPost:
		s.authorizePOST(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) authorizeGET(w http.ResponseWriter, r *http.Request) {
	vm, err := s.parseAuthorizeParams(r.Context(), r.URL.Query())
	if err != nil {
		// Plain-text 400 is appropriate here: the GET form is presented to a
		// human in a browser, and a malformed authorize URL means the client
		// app is broken — no JSON-error contract applies.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	renderLogin(w, vm)
}

func (s *Server) authorizePOST(w http.ResponseWriter, r *http.Request) {
	// TODO: validate the csrf hidden field once login-session cookies are
	// paired with the form render. Today the template emits the field but
	// the handler ignores it — a known limitation acceptable for the
	// current single-user / developer-mode deployment.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	vm, err := s.parseAuthorizeParams(r.Context(), r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	email := r.PostForm.Get("email")
	apiKey := r.PostForm.Get("api_key")
	userAgent := r.PostForm.Get("user_agent")
	if email == "" || apiKey == "" || userAgent == "" {
		vm.Error = "All fields are required."
		renderLogin(w, vm)
		return
	}

	if err := s.cfg.ValidateOrganizze(r.Context(), email, apiKey, userAgent); err != nil {
		// Re-render the form with the error so the human can retry. Status
		// stays 200 — the form is the response body.
		vm.Error = "Invalid Organizze credentials: " + err.Error()
		renderLogin(w, vm)
		return
	}

	ciphertext, nonce, err := s.cfg.Cipher.Seal([]byte(apiKey))
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}
	user, err := s.cfg.Store.UpsertUserByEmail(r.Context(), storage.User{
		OrganizzeEmail: email,
		APIKeyCipher:   ciphertext,
		APIKeyNonce:    nonce,
		UserAgent:      userAgent,
	})
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	code := newRandomToken()
	if err := s.cfg.Store.CreateAuthCode(r.Context(), storage.AuthCode{
		CodeHash:            storage.HashToken(code),
		ClientID:            vm.ClientID,
		UserID:              user.ID,
		RedirectURI:         vm.RedirectURI,
		CodeChallenge:       vm.CodeChallenge,
		CodeChallengeMethod: vm.CodeChallengeMethod,
		ExpiresAt:           s.cfg.Now().Add(5 * time.Minute),
	}); err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	q := url.Values{"code": {code}, "state": {vm.State}}
	http.Redirect(w, r, vm.RedirectURI+"?"+q.Encode(), http.StatusSeeOther)
}

// parseAuthorizeParams validates the OAuth params and returns a populated
// view model (without Error). The same parsing is reused by GET and POST
// so the hidden-field round-trip on POST cannot smuggle a different
// client_id / redirect_uri past the validation we did on GET.
func (s *Server) parseAuthorizeParams(ctx context.Context, q url.Values) (loginViewModel, error) {
	clientID := q.Get("client_id")
	if clientID == "" {
		return loginViewModel{}, errors.New("invalid_request: client_id required")
	}
	client, err := s.cfg.Store.GetClient(ctx, clientID)
	if err != nil {
		return loginViewModel{}, errors.New("invalid_client: unknown client_id")
	}
	redirectURI := q.Get("redirect_uri")
	if !slices.Contains(client.RedirectURIs, redirectURI) {
		return loginViewModel{}, errors.New("invalid_redirect_uri: not registered for this client")
	}
	if rt := q.Get("response_type"); rt != "" && rt != "code" {
		return loginViewModel{}, errors.New("unsupported_response_type")
	}
	method := q.Get("code_challenge_method")
	if method == "" {
		method = "S256"
	}
	if method != "S256" {
		return loginViewModel{}, errors.New("invalid_request: only S256 supported")
	}
	challenge := q.Get("code_challenge")
	if challenge == "" {
		return loginViewModel{}, errors.New("invalid_request: PKCE code_challenge required")
	}
	// S256 challenge is base64url(SHA-256(verifier)) → exactly 43 chars
	// (32 bytes encoded with RawURLEncoding, no padding).
	if len(challenge) != 43 {
		return loginViewModel{}, errors.New("invalid_request: code_challenge must be 43-char base64url (S256)")
	}
	return loginViewModel{
		ClientID:            clientID,
		ClientName:          client.ClientName,
		RedirectURI:         redirectURI,
		State:               q.Get("state"),
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
	}, nil
}

func renderLogin(w http.ResponseWriter, vm loginViewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginTpl.Execute(w, vm)
}

func newRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
