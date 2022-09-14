package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/kriive/lil"
)

var _ lil.ShortService = (*ShortService)(nil)

type ShortService struct {
	db *DB
}

func NewShortService(db *DB) *ShortService {
	return &ShortService{db: db}
}

func (s *ShortService) FindShortByKey(ctx context.Context, key string) (*lil.Short, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	short, err := findShortByKey(ctx, tx, key)
	if err != nil {
		return nil, err
	} else if err := attachShortAssociations(ctx, tx, short); err != nil {
		return nil, err
	}

	return short, err
}

func findShortByKey(ctx context.Context, tx *Tx, key string) (*lil.Short, error) {
	shorts, _, err := findShorts(ctx, tx, lil.ShortFilter{Key: &key})
	if err != nil {
		return nil, err
	} else if len(shorts) == 0 {
		return nil, lil.Errorf(lil.ENOTFOUND, "Short not found.")
	}

	return shorts[0], nil
}

func findShorts(ctx context.Context, tx *Tx, filter lil.ShortFilter) (_ []*lil.Short, n int, err error) {
	// Build WHERE clause. Each part of the WHERE clause is AND-ed together.
	// Values are appended to an arg list to avoid SQL injection.
	where, args := []string{"1 = 1"}, []any{}
	if v := filter.Key; v != nil {
		where, args = append(where, "key = ?"), append(args, *v)
	}

	if v := filter.URL; v != nil {
		where, args = append(where, "url = ?"), append(args, *v)
	}

	rows, err := tx.QueryContext(ctx, `
			SELECT
				key,
				url,
				owner_id,
				created_at,
				updated_at,
				COUNT(*) OVER()
			FROM shorts
			WHERE `+strings.Join(where, " AND ")+`
			`+FormatLimitOffset(filter.Limit, filter.Offset),
		args...,
	)
	if err != nil {
		return nil, n, FormatError(err)
	}
	defer rows.Close()

	shorts := make([]*lil.Short, 0)
	for rows.Next() {
		var short lil.Short
		if err := rows.Scan(
			&short.Key,
			(*DBUrl)(&short.URL),
			&short.OwnerID,
			(*NullTime)(&short.CreatedAt),
			(*NullTime)(&short.UpdatedAt),
			&n,
		); err != nil {
			return nil, 0, err
		}
		shorts = append(shorts, &short)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return shorts, n, nil
}

// Retrieves a list of Shorts based on a filter. Returns a count of the
// matching objects that may be different from the actual count of objects
// returned (if you have set the "Limit" field).
func (s *ShortService) FindShorts(ctx context.Context, filter lil.ShortFilter) ([]*lil.Short, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	return findShorts(ctx, tx, filter)
}

// Creates a new Short.
func (s *ShortService) CreateShort(ctx context.Context, short *lil.Short) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := createShort(ctx, tx, short); err != nil {
		return err
	} else if attachShortAssociations(ctx, tx, short); err != nil {
		return err
	}
	return tx.Commit()
}

func createShort(ctx context.Context, tx *Tx, short *lil.Short) error {
	ownerID := lil.UserIDFromContext(ctx)
	if ownerID == 0 {
		return lil.Errorf(lil.EUNAUTHORIZED, "You must be logged in to create a short.")
	}
	short.OwnerID = ownerID

	short.CreatedAt = tx.now
	short.UpdatedAt = short.CreatedAt

	if err := short.Validate(); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
			INSERT INTO shorts (
				url,
				key,
				owner_id,
				created_at,
				updated_at
			)
			VALUES (?, ?, ?, ?, ?)
	`,
		(*DBUrl)(&short.URL),
		short.Key,
		short.OwnerID,
		(*NullTime)(&short.CreatedAt),
		(*NullTime)(&short.UpdatedAt),
	)
	if err != nil {
		return FormatError(err)
	}

	return nil
}

// Permanently removes a Short. Returns a ENOTFOUND if the key
// does not belong to any Short.
func (s *ShortService) DeleteShort(ctx context.Context, key string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := deleteShort(ctx, tx, key); err != nil {
		return err
	}

	return tx.Commit()
}

func deleteShort(ctx context.Context, tx *Tx, key string) error {
	if _, err := findShortByKey(ctx, tx, key); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM shorts WHERE key = ?`, key); err != nil {
		return FormatError(err)
	}

	return nil
}

// attachShortAssociations is a helper function to look up and attach the owner user to the short.
func attachShortAssociations(ctx context.Context, tx *Tx, short *lil.Short) (err error) {
	if short.Owner, err = findUserByID(ctx, tx, short.OwnerID); err != nil {
		return fmt.Errorf("attach short user: %w", err)
	}
	return nil
}
