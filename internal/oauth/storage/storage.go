// Package storage is the persistence layer for the OAuth Authorization Server.
// Store is the single interface every server handler consumes; postgres.go is
// the only concrete implementation in this iteration.
package storage

import (
	"context"
	"time"
)

type User struct {
	ID             int64
	OrganizzeEmail string
	APIKeyCipher   []byte
	APIKeyNonce    []byte
	UserAgent      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Client struct {
	ID               string
	ClientSecretHash []byte // nil for public/PKCE-only clients
	ClientName       string
	RedirectURIs     []string
	CreatedAt        time.Time
}

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthCode struct {
	CodeHash            []byte
	ClientID            string
	UserID              int64
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string // always "S256"
	Scope               string
	ExpiresAt           time.Time
	ConsumedAt          *time.Time
}

type Token struct {
	TokenHash  []byte
	Kind       string // "access" or "refresh"
	ClientID   string
	UserID     int64
	RefreshFor []byte // for access tokens: hash of the issuing refresh token; nil otherwise
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// Store is the persistence contract for the OAuth AS. Methods return
// ErrNotFound or ErrConflict where applicable. Implementations must be
// safe for concurrent use.
type Store interface {
	// Users
	UpsertUserByEmail(ctx context.Context, u User) (User, error)
	GetUser(ctx context.Context, id int64) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)

	// Clients
	CreateClient(ctx context.Context, c Client) error
	GetClient(ctx context.Context, id string) (Client, error)

	// Sessions
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	DeleteSession(ctx context.Context, id string) error

	// Authorization codes
	CreateAuthCode(ctx context.Context, ac AuthCode) error
	ConsumeAuthCode(ctx context.Context, codeHash []byte) (AuthCode, error)

	// Tokens
	CreateToken(ctx context.Context, tok Token) error
	GetToken(ctx context.Context, tokenHash []byte) (Token, error)
	RevokeToken(ctx context.Context, tokenHash []byte) error
	RevokeRefreshFamily(ctx context.Context, refreshHash []byte) error
}
