package sqlite_test

import (
	"context"
	"net/url"
	"reflect"
	"testing"

	"github.com/kriive/lil"
	"github.com/kriive/lil/sqlite"
)

func TestShortService_CreateShort(t *testing.T) {
	// Ensure a short can be created.
	t.Run("OK", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		s := sqlite.NewShortService(db)

		u, _ := url.Parse("https://example.com")
		key := "12345"

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		short := &lil.Short{URL: *u, Key: key}
		if err := s.CreateShort(ctx, short); err != nil {
			t.Fatal(err)
		} else if short.CreatedAt.IsZero() {
			t.Fatal("expected CreatedAt")
		} else if short.UpdatedAt.IsZero() {
			t.Fatal("expected UpdatedAt")
		} else if short.Owner == nil {
			t.Fatal("expected owner")
		}

		// Fetch short from database and compare.
		if other, err := s.FindShortByKey(ctx, key); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(short, other) {
			t.Fatalf("mismatch: %#v != %#v", short, other)
		}
	})

	t.Run("ErrMissingURL", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		s := sqlite.NewShortService(db)

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		short := &lil.Short{}
		if err := s.CreateShort(ctx, short); err == nil {
			t.Fatal("expected error")
		} else if lil.ErrorCode(err) != lil.EINVALID || lil.ErrorMessage(err) != "Missing URL." {
			t.Fatal(err)
		}
	})

	t.Run("ErrMissingKey", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		s := sqlite.NewShortService(db)

		u, _ := url.Parse("https://example.com")
		short := &lil.Short{URL: *u}

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		if err := s.CreateShort(ctx, short); err == nil {
			t.Fatal("expected err")
		} else if lil.ErrorCode(err) != lil.EINVALID || lil.ErrorMessage(err) != "Missing Key." {
			t.Fatal(err)
		}
	})

	t.Run("ErrDuplicateShort", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		s := sqlite.NewShortService(db)

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		u, _ := url.Parse("https://example.com")
		short := &lil.Short{URL: *u, Key: "12345"}

		MustCreateShort(t, ctx, db, short)

		if err := s.CreateShort(ctx, short); err == nil {
			t.Fatal("expected error")
		} else if lil.ErrorCode(err) != lil.ECONFLICT || lil.ErrorMessage(err) != "Short with the same key already exists." {
			t.Fatal(err)
		}
	})

	t.Run("ErrInvalidURLScheme", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		s := sqlite.NewShortService(db)

		u, err := url.ParseRequestURI("invalid-scheme://www.google.com")
		if err != nil {
			t.Fatal(err)
		}

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		if err := s.CreateShort(ctx, &lil.Short{
			URL: *u,
			Key: "12345",
		}); err == nil {
			t.Fatal("expected error")
		} else if lil.ErrorCode(err) != lil.EINVALID || lil.ErrorMessage(err) != "Invalid URL scheme. Only http and https are supported." {
			t.Fatal(err)
		}
	})
}

func TestShortService_FindShorts(t *testing.T) {
	t.Run("Key", func(t *testing.T) {
		db := MustOpenDB(t)
		defer MustCloseDB(t, db)

		_, ctx := MustCreateUser(t, context.Background(), db, &lil.User{
			Name:  "Test",
			Email: "Test",
		})

		url1, _ := url.Parse("https://1.example.com")
		key1 := "12345"
		MustCreateShort(t, ctx, db, &lil.Short{URL: *url1, Key: key1})

		url2, _ := url.Parse("https://2.example.com")
		key2 := "23456"
		MustCreateShort(t, ctx, db, &lil.Short{URL: *url2, Key: key2})

		url3, _ := url.Parse("https://3.example.com")
		key3 := "34567"
		MustCreateShort(t, ctx, db, &lil.Short{URL: *url3, Key: key3})

		s := sqlite.NewShortService(db)

		if a, n, err := s.FindShorts(ctx, lil.ShortFilter{}); err != nil {
			t.Fatal(err)
		} else if got, want := len(a), 3; got != want {
			t.Fatalf("len=%v, want %v", got, want)
		} else if got, want := a[0].Key, key1; got != want {
			t.Fatalf("key=%v, want %v", got, want)
		} else if got, want := a[1].Key, key2; got != want {
			t.Fatalf("key=%v, want %v", got, want)
		} else if got, want := a[2].Key, key3; got != want {
			t.Fatalf("key=%v, want %v", got, want)
		} else if got, want := n, 3; got != want {
			t.Fatalf("n=%v, want %v", got, want)
		}
	})
}

func MustCreateShort(tb testing.TB, ctx context.Context, db *sqlite.DB, short *lil.Short) *lil.Short {
	tb.Helper()
	if err := sqlite.NewShortService(db).CreateShort(ctx, short); err != nil {
		tb.Fatal(err)
	}
	return short
}
