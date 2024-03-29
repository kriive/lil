package lil

import (
	"context"
	"net/url"
	"time"
)

var (
	ErrEmptyURL         = Errorf(EINVALID, "Missing URL.")
	ErrEmptyKey         = Errorf(EINVALID, "Missing Key.")
	ErrEmptyOwner       = Errorf(EINVALID, "Missing owner.")
	ErrInvalidURLScheme = Errorf(EINVALID, "Invalid URL scheme. Only http and https are supported.")
)

// Short defines a shortened URL.
type Short struct {
	// URL stores the original URL.
	URL url.URL `json:"url"`

	// Key stores the key needed to retrieve the original URL.
	Key string `json:"key"`

	Owner   *User `json:"-"`
	OwnerID int   `json:"-"`

	// CreatedAt and UpdatedAt get filled by the service.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ShortService represents a service for managing Shorts.
type ShortService interface {
	// Retrieves a single Short by Key. Returns ENOTFOUND if the Short
	// object does not exist. Returns EUNAUTHORIZED if the Short does not
	// belong to the user.
	FindShortByKey(ctx context.Context, key string) (*Short, error)

	// Retrieves a single Short by Key. Returns ENOTFOUND if the Short
	// object does not exist. Does not check if the Short does belong
	// to the user.
	SearchShort(ctx context.Context, key string) (*Short, error)

	// Retrieves a list of Shorts based on a filter. Returns a count of the
	// matching objects that may be different from the actual count of objects
	// returned (if you have set the "Limit" field).
	FindShorts(ctx context.Context, filter ShortFilter) ([]*Short, int, error)

	// Creates a new Short.
	CreateShort(ctx context.Context, short *Short) error

	// Permanently removes a Short. Returns a ENOTFOUND if the key
	// does not belong to any Short.
	DeleteShort(ctx context.Context, key string) error
}

// ShortFilter represents a filter used by FindShorts().
type ShortFilter struct {
	Key *string  `json:"key"`
	URL *url.URL `json:"url"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// Validate returns an error if Short has invalid fields.
// Only performs basic validation.
func (s *Short) Validate() error {
	if s.URL.String() == "" {
		return ErrEmptyURL
	}

	if s.URL.Scheme != "https" && s.URL.Scheme != "http" {
		return ErrInvalidURLScheme
	}

	if s.Key == "" {
		return ErrEmptyKey
	}

	if s.OwnerID == 0 {
		return ErrEmptyOwner
	}

	return nil
}

// Only the short owner can delete the short.
func CanEditShort(ctx context.Context, short *Short) bool {
	return short.OwnerID == UserIDFromContext(ctx)
}
