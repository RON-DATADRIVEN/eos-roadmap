package contracts

import "errors"

// Contract validation errors for Payload operations
var (
	ErrNilPayload = errors.New("payload cannot be nil")
	ErrEmptyID    = errors.New("payload ID cannot be empty")
	ErrEmptyData  = errors.New("payload data cannot be empty")
	ErrNotFound   = errors.New("payload not found")
)

// Contract validation errors for Session operations
var (
	ErrNilSession        = errors.New("session cannot be nil")
	ErrEmptySessionID    = errors.New("session ID cannot be empty")
	ErrEmptyUserID       = errors.New("user ID cannot be empty")
	ErrInvalidCreatedAt  = errors.New("created_at timestamp cannot be zero")
	ErrInvalidExpiresAt  = errors.New("expires_at must be after created_at")
	ErrSessionNotFound   = errors.New("session not found")
)