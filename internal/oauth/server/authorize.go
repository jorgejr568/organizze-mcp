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
	ConsentToken        string
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
	vm.ConsentToken = signConsent(s.cfg.CookieSecret, consentBinding{
		ClientID:    vm.ClientID,
		RedirectURI: vm.RedirectURI,
		Challenge:   vm.CodeChallenge,
		State:       vm.State,
		IssuedAt:    s.cfg.Now().Unix(),
	})
	renderLogin(w, vm)
}

func (s *Server) authorizePOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	// CSRF + tamper guard. The consent_token is an HMAC-signed binding over
	// (client_id, redirect_uri, code_challenge, state, issued_at) rendered into
	// the GET form. POSTed values for those fields must match the bound values
	// — otherwise the browser was tricked into submitting a different
	// authorization (CSRF / login-CSRF) or a tampered form.
	binding, err := verifyConsent(s.cfg.CookieSecret, r.PostForm.Get("consent_token"), s.cfg.Now())
	if err != nil {
		http.Error(w, "invalid consent_token: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("client_id") != binding.ClientID ||
		r.PostForm.Get("redirect_uri") != binding.RedirectURI ||
		r.PostForm.Get("code_challenge") != binding.Challenge ||
		r.PostForm.Get("state") != binding.State {
		http.Error(w, "consent_token does not match POSTed params", http.StatusBadRequest)
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
		vm.ConsentToken = s.issueConsentToken(vm)
		renderLogin(w, vm)
		return
	}

	if err := s.cfg.ValidateOrganizze(r.Context(), email, apiKey, userAgent); err != nil {
		// Re-render the form with the error so the human can retry. Status
		// stays 200 — the form is the response body. Re-issue the consent
		// token so the user has a fresh 10-min window to fix the error.
		vm.Error = "Invalid Organizze credentials: " + err.Error()
		vm.ConsentToken = s.issueConsentToken(vm)
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
	if !isBase64URLNoPad(challenge, 43) {
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

func (s *Server) issueConsentToken(vm loginViewModel) string {
	return signConsent(s.cfg.CookieSecret, consentBinding{
		ClientID:    vm.ClientID,
		RedirectURI: vm.RedirectURI,
		Challenge:   vm.CodeChallenge,
		State:       vm.State,
		IssuedAt:    s.cfg.Now().Unix(),
	})
}

func renderLogin(w http.ResponseWriter, vm loginViewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginTpl.Execute(w, vm); err != nil {
		// Template failures here mean the embedded template diverged from the
		// view model — a programmer bug. The response has already been
		// committed (headers written), so we can't issue a clean 500; surface
		// it via the log channel that ServeHTTP wraps us with.
		_, _ = w.Write([]byte("\n<!-- template render error: " + err.Error() + " -->"))
	}
}

func newRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// isBase64URLNoPad reports whether s is exactly n base64url chars
// (A–Z, a–z, 0–9, '-', '_') with no padding. Used to validate the PKCE
// code_challenge — RFC 7636 §4.2 mandates the URL-safe base64 alphabet
// without padding.
func isBase64URLNoPad(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
