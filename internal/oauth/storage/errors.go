package storage

import "errors"

var (
	ErrNotFound = errors.New("oauth/storage: not found")
	ErrConflict = errors.New("oauth/storage: conflict")
)
