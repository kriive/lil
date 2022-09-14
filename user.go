package lil

import (
	"context"
	"time"
)

// User represents a user in the system. Users are typically
// created via OAuth using the AuthService but users can
// also be create directly for testing.
type User struct {
	ID int `json:"id"`
	
	// User's preferred name & email.
	Name string `json:"name"`
	Email string `json:"email"`
	
	// Randomly generated API key for use with the CLI.
	APIKey string `json:"-"`

	// Timestamps for user creation & last update.
	CreatedAt time.Time
	UpdatedAt time.Time
	
	// List of associated OAuth authentication objects.
	Auths []*Auth `json:"auths"`
}

// Validate returns an error if the user contains invalid fields.
// This only performs basic validation.
func (u *User) Validate() error {
	if u.Name == "" {
		return Errorf(EINVALID, "User name required.")
	}
	return nil
}

// AvatarURL returns a URL to the avatar image for the user.
// This loops over all auth providers to find the first 
// available avatar.
func (u* User) AvatarURL(size int) string {
	for _, auth := range u.Auths {
		if s:=auth.AvatarURL(size); s != "" {
			return s
		}
	}
	return ""
}

type UserService interface {
	// Retrieves a user by ID along with their associated auth objects.
	// Returns ENOTFOUND if user does not exist.
	FindUserByID(ctx context.Context, id int) (*User, error)
	
	// Retrieves a list of users by filter. Also returns total count
	// of matching users which may differ from returned results if
	// filter.Limit is specified.
	FindUsers(ctx context.Context, filter UserFilter) ([]*User, int, error)
	
	// Creates a new user. This is only used for testing since users are
	// typically created during the OAuth creation process in AuthService.CreateAuth().
	CreateUser(ctx context.Context, user *User) error
	
	// Updates a user object. Returns EUNAUTHORIZED if current user is not
	// the user that is being updated. Returns ENOTFOUND id user does not
	// exist.
	UpdateUser(ctx context.Context, id int, upd UserUpdate) (*User, error)
	
	// Permanently deletes a user and all owned dials. Returns EUNAUTHORIZED
	// if current user is not the user being deleted. Returns ENOTFOUND if
	// user does not exist.
	DeleteUser(ctx context.Context, id int) error
}

// UserFilter represents a filter passed to FindUsers().
type UserFilter struct {
	// Filtering fields.
	ID *int `json:"id"`
	Email *string `json:"email"`
	APIKey *string `json:"apiKey"`
	
	// Restrict to subset of results.
	Offset int `json:"offset"`
	Limit int `json:"limit"`
}

// UserUpdate represents a set of fields to be updated via UpdateUser().
type UserUpdate struct {
	Name *string `json:"name"`
	Email *string `json:"email"`
}
