package server

import (
	"context"
	"sync"
	"time"

	"github.com/jorgejr568/organizze-mcp/internal/oauth/storage"
)

// fakeStore is an in-memory storage.Store used by the server tests.
// Methods are added as new tests need them; calling an unimplemented
// method panics so missing pieces fail loudly.
type fakeStore struct {
	mu       sync.Mutex
	users    map[int64]storage.User
	emails   map[string]int64
	clients  map[string]storage.Client
	sessions map[string]storage.Session
	codes    map[string]storage.AuthCode
	tokens   map[string]storage.Token
	nextID   int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:    map[int64]storage.User{},
		emails:   map[string]int64{},
		clients:  map[string]storage.Client{},
		sessions: map[string]storage.Session{},
		codes:    map[string]storage.AuthCode{},
		tokens:   map[string]storage.Token{},
	}
}

// For Task 9, only the client methods are exercised. Later tasks
// (11–14) extend fakeStore with the methods they need.
func (f *fakeStore) CreateClient(_ context.Context, c storage.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.clients[c.ID]; ok {
		return storage.ErrConflict
	}
	f.clients[c.ID] = c
	return nil
}

func (f *fakeStore) GetClient(_ context.Context, id string) (storage.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.clients[id]
	if !ok {
		return storage.Client{}, storage.ErrNotFound
	}
	return c, nil
}

// --- unimplemented methods on storage.Store; later tasks fill these in. ---
// Implement the bare minimum required to satisfy the interface so the
// fakeStore compiles; have them panic so accidental use is loud.

func (f *fakeStore) UpsertUserByEmail(_ context.Context, u storage.User) (storage.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.emails[u.OrganizzeEmail]; ok {
		u.ID = id
		f.users[id] = u
		return u, nil
	}
	f.nextID++
	u.ID = f.nextID
	f.users[u.ID] = u
	f.emails[u.OrganizzeEmail] = u.ID
	return u, nil
}

func (f *fakeStore) GetUser(_ context.Context, id int64) (storage.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return storage.User{}, storage.ErrNotFound
}

func (f *fakeStore) GetUserByEmail(_ context.Context, e string) (storage.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.emails[e]; ok {
		return f.users[id], nil
	}
	return storage.User{}, storage.ErrNotFound
}

func (f *fakeStore) CreateSession(context.Context, storage.Session) error {
	panic("fakeStore.CreateSession not implemented yet")
}

func (f *fakeStore) GetSession(context.Context, string) (storage.Session, error) {
	panic("fakeStore.GetSession not implemented yet")
}

func (f *fakeStore) DeleteSession(context.Context, string) error {
	panic("fakeStore.DeleteSession not implemented yet")
}

func (f *fakeStore) CreateAuthCode(_ context.Context, ac storage.AuthCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[string(ac.CodeHash)] = ac
	return nil
}

func (f *fakeStore) ConsumeAuthCode(_ context.Context, h []byte) (storage.AuthCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ac, ok := f.codes[string(h)]
	if !ok || ac.ConsumedAt != nil || ac.ExpiresAt.Before(time.Now()) {
		return storage.AuthCode{}, storage.ErrNotFound
	}
	now := time.Now().UTC()
	ac.ConsumedAt = &now
	f.codes[string(h)] = ac
	return ac, nil
}

func (f *fakeStore) CreateToken(_ context.Context, tok storage.Token) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[string(tok.TokenHash)] = tok
	return nil
}

func (f *fakeStore) GetToken(_ context.Context, h []byte) (storage.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[string(h)]
	if !ok {
		return storage.Token{}, storage.ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) RevokeToken(_ context.Context, h []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[string(h)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	t.RevokedAt = &now
	f.tokens[string(h)] = t
	return nil
}

func (f *fakeStore) RevokeRefreshFamily(_ context.Context, h []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, t := range f.tokens {
		if string(t.TokenHash) == string(h) || (t.RefreshFor != nil && string(t.RefreshFor) == string(h)) {
			now := time.Now().UTC()
			t.RevokedAt = &now
			f.tokens[k] = t
		}
	}
	return nil
}

// Compile-time check.
var _ storage.Store = (*fakeStore)(nil)
