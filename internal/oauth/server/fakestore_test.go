package server

import (
	"context"
	"sync"

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

func (f *fakeStore) ConsumeAuthCode(context.Context, []byte) (storage.AuthCode, error) {
	panic("fakeStore.ConsumeAuthCode not implemented yet")
}

func (f *fakeStore) CreateToken(context.Context, storage.Token) error {
	panic("fakeStore.CreateToken not implemented yet")
}

func (f *fakeStore) GetToken(context.Context, []byte) (storage.Token, error) {
	panic("fakeStore.GetToken not implemented yet")
}

func (f *fakeStore) RevokeToken(context.Context, []byte) error {
	panic("fakeStore.RevokeToken not implemented yet")
}

func (f *fakeStore) RevokeRefreshFamily(context.Context, []byte) error {
	panic("fakeStore.RevokeRefreshFamily not implemented yet")
}

// Compile-time check.
var _ storage.Store = (*fakeStore)(nil)
