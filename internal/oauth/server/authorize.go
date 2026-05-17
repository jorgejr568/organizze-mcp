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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

//go:embed templates/login.html
var loginFS embed.FS

var loginTpl = template.Must(template.ParseFS(loginFS, "templates/login.html"))

type sessionUserView struct {
	Email   string
	Initial string
}

type loginViewModel struct {
	ClientID            string
	ClientName          string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	ConsentToken        string
	Error               string

	// SessionUser is populated when a valid browser-session cookie identifies
	// a previously-authorized user. The template renders a "Continue as X"
	// shortcut instead of the full credential form.
	SessionUser *sessionUserView

	// RawQuery is the GET query string (with `reset` stripped) so the
	// "Switch user" link can preserve the original OAuth params.
	RawQuery string
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

	// `?reset=1` is the "switch user" link target. Drop the session row + cookie
	// before rendering so the fresh form has no SessionUser context.
	if r.URL.Query().Get("reset") == "1" {
		s.clearBrowserSession(w, r)
	} else if user, ok := s.lookupBrowserSession(r); ok {
		vm.SessionUser = &sessionUserView{
			Email:   user.OrganizzeEmail,
			Initial: firstInitial(user.OrganizzeEmail),
		}
	}

	// RawQuery for the "Switch user" link — strip `reset` so we don't render
	// a duplicate when the user is already on a reset URL.
	q := r.URL.Query()
	q.Del("reset")
	vm.RawQuery = q.Encode()

	vm.ConsentToken = s.issueConsentToken(vm)
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

	// Skip-fill path: when use_session=1 is set we trust the browser cookie
	// to identify the returning user instead of re-prompting for the API key.
	// Stored credentials are re-validated against Organizze in case the key
	// was revoked since storage.
	if r.PostForm.Get("use_session") == "1" {
		if user, ok := s.lookupBrowserSession(r); ok {
			apiKey, err := s.cfg.Cipher.Open(user.APIKeyCipher, user.APIKeyNonce)
			if err != nil {
				http.Error(w, "server_error", http.StatusInternalServerError)
				return
			}
			if err := s.cfg.ValidateOrganizze(r.Context(), user.OrganizzeEmail, string(apiKey), user.UserAgent); err != nil {
				// Stored key no longer works — fall back to the full form.
				vm.Error = "Suas credenciais Organizze salvas não funcionam mais. Entre novamente."
				vm.RawQuery = r.URL.RawQuery
				vm.ConsentToken = s.issueConsentToken(vm)
				renderLogin(w, vm)
				return
			}
			s.completeAuthorize(w, r, vm, user)
			return
		}
		// Cookie missing/invalid — silently fall through to the full form so
		// the user can re-authenticate without a confusing "session expired"
		// message. They'll see the normal form with all fields blank.
	}

	email := r.PostForm.Get("email")
	apiKey := r.PostForm.Get("api_key")
	userAgent := r.PostForm.Get("user_agent")
	if email == "" || apiKey == "" || userAgent == "" {
		vm.Error = "Todos os campos são obrigatórios."
		vm.ConsentToken = s.issueConsentToken(vm)
		renderLogin(w, vm)
		return
	}

	if err := s.cfg.ValidateOrganizze(r.Context(), email, apiKey, userAgent); err != nil {
		// Re-render the form with the error so the human can retry. Status
		// stays 200 — the form is the response body. Re-issue the consent
		// token so the user has a fresh 10-min window to fix the error.
		vm.Error = "Credenciais Organizze inválidas: " + err.Error()
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
	s.completeAuthorize(w, r, vm, user)
}

// completeAuthorize is the tail shared by the full-form path and the
// skip-fill path. It mints the authorization code, writes a fresh browser
// session, and 303-redirects back to the OAuth client.
func (s *Server) completeAuthorize(w http.ResponseWriter, r *http.Request, vm loginViewModel, user storage.User) {
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
	s.writeBrowserSession(w, r, user.ID)
	q := url.Values{"code": {code}, "state": {vm.State}}
	http.Redirect(w, r, vm.RedirectURI+"?"+q.Encode(), http.StatusSeeOther)
}

// lookupBrowserSession reads the session cookie, validates it against the
// stored oauth_sessions row, and returns the associated user. Any failure
// path (missing cookie, bad signature, expired row, deleted user) returns
// ok=false silently — the caller falls back to the full credential form.
func (s *Server) lookupBrowserSession(r *http.Request) (storage.User, bool) {
	sessionID, ok := s.sessions.read(r)
	if !ok {
		return storage.User{}, false
	}
	sess, err := s.cfg.Store.GetSession(r.Context(), sessionID)
	if err != nil {
		return storage.User{}, false
	}
	user, err := s.cfg.Store.GetUser(r.Context(), sess.UserID)
	if err != nil {
		return storage.User{}, false
	}
	return user, true
}

// writeBrowserSession mints a new session row + signed cookie. Old session
// rows for the same user are left in place (cheap, expires naturally); the
// cookie is rotated on every authorize success so a leaked cookie has a
// bounded replay window.
func (s *Server) writeBrowserSession(w http.ResponseWriter, r *http.Request, userID int64) {
	id := newRandomToken()
	if err := s.cfg.Store.CreateSession(r.Context(), storage.Session{
		ID:        id,
		UserID:    userID,
		ExpiresAt: s.cfg.Now().Add(s.cfg.SessionTTL),
	}); err != nil {
		// Logging is best-effort; the OAuth flow still completes without a
		// session cookie — the user just sees the full form next time.
		s.cfg.Logger.Warn("write browser session", zap.Error(err))
		return
	}
	s.sessions.write(w, id)
}

// clearBrowserSession deletes the DB row referenced by the current cookie
// (if any) and clears the cookie at the client. Always invoked from the
// `?reset=1` GET path.
func (s *Server) clearBrowserSession(w http.ResponseWriter, r *http.Request) {
	if sessionID, ok := s.sessions.read(r); ok {
		_ = s.cfg.Store.DeleteSession(r.Context(), sessionID)
	}
	s.sessions.clear(w)
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

// firstInitial returns the first non-empty character of email, uppercased.
// Used for the avatar bubble in the skip-fill view. Falls back to "?" for
// the impossible empty case.
func firstInitial(email string) string {
	for _, r := range email {
		return strings.ToUpper(string(r))
	}
	return "?"
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
