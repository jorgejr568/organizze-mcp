package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

// resetSchema wipes all five OAuth tables. Call after migrations are applied.
func resetSchema(t *testing.T, s *Postgres) {
	t.Helper()
	ctx := context.Background()
	for _, tbl := range []string{"oauth_tokens", "oauth_codes", "oauth_sessions", "oauth_clients", "oauth_users"} {
		if _, err := s.pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}

func newStore(t *testing.T) *Postgres {
	t.Helper()
	pool := requireDB(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewPostgres(pool)
	resetSchema(t, s)
	return s
}

func TestPostgres_Users(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	u, err := s.UpsertUserByEmail(ctx, User{
		OrganizzeEmail: "a@b.com",
		APIKeyCipher:   []byte{1, 2},
		APIKeyNonce:    []byte{3, 4},
		UserAgent:      "UA",
	})
	if err != nil {
		t.Fatalf("UpsertUserByEmail (insert): %v", err)
	}
	if u.ID == 0 {
		t.Error("expected ID assigned")
	}

	u2, err := s.UpsertUserByEmail(ctx, User{
		OrganizzeEmail: "a@b.com",
		APIKeyCipher:   []byte{9, 9},
		APIKeyNonce:    []byte{8, 8},
		UserAgent:      "UA2",
	})
	if err != nil {
		t.Fatalf("UpsertUserByEmail (update): %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("upsert created a new row: %d vs %d", u2.ID, u.ID)
	}
	if u2.APIKeyCipher[0] != 9 || u2.UserAgent != "UA2" {
		t.Errorf("upsert did not overwrite: %+v", u2)
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.OrganizzeEmail != "a@b.com" {
		t.Errorf("GetUser email = %q", got.OrganizzeEmail)
	}

	if _, err := s.GetUser(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgres_ClientsAndSessions(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})

	if err := s.CreateClient(ctx, Client{ID: "cli-1", ClientName: "ChatGPT", RedirectURIs: []string{"https://chat.openai.com/cb"}}); err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	c, err := s.GetClient(ctx, "cli-1")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if c.ClientName != "ChatGPT" || len(c.RedirectURIs) != 1 {
		t.Errorf("client = %+v", c)
	}

	sess := Session{ID: "sess-1", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("session userID = %d", got.UserID)
	}
	if err := s.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, "sess-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_AuthCode_ConsumeOnce(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})
	_ = s.CreateClient(ctx, Client{ID: "cli", ClientName: "X", RedirectURIs: []string{"https://cb"}})

	hash := HashToken("the-code")
	ac := AuthCode{
		CodeHash: hash, ClientID: "cli", UserID: u.ID,
		RedirectURI: "https://cb", CodeChallenge: "abc", CodeChallengeMethod: "S256",
		ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
	}
	if err := s.CreateAuthCode(ctx, ac); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	got, err := s.ConsumeAuthCode(ctx, hash)
	if err != nil {
		t.Fatalf("ConsumeAuthCode: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("consumed user = %d", got.UserID)
	}
	if _, err := s.ConsumeAuthCode(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Errorf("double consume should fail, got %v", err)
	}
}

func TestPostgres_Tokens_RevokeAndFamily(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	u, _ := s.UpsertUserByEmail(ctx, User{OrganizzeEmail: "x@x.com", APIKeyCipher: []byte{1}, APIKeyNonce: []byte{2}, UserAgent: "UA"})
	_ = s.CreateClient(ctx, Client{ID: "cli", ClientName: "X", RedirectURIs: []string{"https://cb"}})

	refresh := Token{
		TokenHash: HashToken("rt-1"), Kind: "refresh", ClientID: "cli", UserID: u.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}
	if err := s.CreateToken(ctx, refresh); err != nil {
		t.Fatalf("CreateToken refresh: %v", err)
	}
	access := Token{
		TokenHash: HashToken("at-1"), Kind: "access", ClientID: "cli", UserID: u.ID,
		RefreshFor: refresh.TokenHash,
		ExpiresAt:  time.Now().Add(time.Hour).UTC(),
	}
	if err := s.CreateToken(ctx, access); err != nil {
		t.Fatalf("CreateToken access: %v", err)
	}

	got, err := s.GetToken(ctx, access.TokenHash)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.UserID != u.ID || got.RevokedAt != nil {
		t.Errorf("token = %+v", got)
	}
	if err := s.RevokeRefreshFamily(ctx, refresh.TokenHash); err != nil {
		t.Fatalf("RevokeRefreshFamily: %v", err)
	}
	rev, _ := s.GetToken(ctx, refresh.TokenHash)
	if rev.RevokedAt == nil {
		t.Error("refresh not revoked")
	}
	revA, _ := s.GetToken(ctx, access.TokenHash)
	if revA.RevokedAt == nil {
		t.Error("descendant access not revoked")
	}
}
