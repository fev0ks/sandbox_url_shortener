package domain

import "errors"

var (
	// ErrNotFound indicates a link was not found or has been soft-deleted.
	ErrNotFound = errors.New("link not found")

	// ErrShortCodeCollision indicates a generated short code already exists.
	ErrShortCodeCollision = errors.New("short code collision")

	// ErrLinkGone indicates a link is expired or has reached its max click limit.
	ErrLinkGone = errors.New("link gone")
)
