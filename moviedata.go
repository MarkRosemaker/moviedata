// Package moviedata looks films and shows up at The Movie Database and keeps
// what it finds: the fields of them that are read, and searches, in a SQLite
// database, their images as files beside it. What is once fetched is answered
// from there ever after, so a program asks this package, never The Movie
// Database's client directly.
package moviedata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/MarkRosemaker/moviedata/databases/data"
	"github.com/go-api-libs/api"
	"github.com/go-api-libs/themoviedb/pkg/themoviedb"
)

// DebugFile is where the responses the client cannot decode are added, for it
// to learn from, relative to the working directory.
const DebugFile = "debug/themoviedb.json"

// Client answers from what it keeps, and fetches from The Movie Database what
// it does not keep yet.
//
// Where something does not exist, at The Movie Database or among a season's
// episodes or a film's images, the error it returns is [fs.ErrNotExist].
type Client struct {
	db  *data.DB
	dir string

	cli    *themoviedb.Client
	cliErr error
}

// Path is where the database lives by default: data.db in ~/.moviedata. Its
// images are kept in the images directory beside it.
func Path() (string, error) { return data.Path() }

// Open opens the database at path, creating it where it does not exist, and
// keeps images in the directory beside it.
//
// Without TMDB_API_TOKEN set it still opens:
// what is kept is answered, and fetching what is not fails.
func Open(ctx context.Context, path string) (*Client, error) {
	return open(ctx, path, themoviedb.WithDebugFile(DebugFile))
}

func open(ctx context.Context, path string, opts ...themoviedb.ClientOption) (*Client, error) {
	db, err := data.Open(ctx, path)
	if err != nil {
		return nil, err
	}

	cli, cliErr := themoviedb.NewClient(opts...)

	return &Client{db: db, dir: filepath.Dir(path), cli: cli, cliErr: cliErr}, nil
}

// Close closes the database.
func (c *Client) Close() error { return c.db.Close() }

// client is the client to fetch with, or why there is none.
func (c *Client) client() (*themoviedb.Client, error) {
	if c.cliErr != nil {
		return nil, fmt.Errorf("fetching from The Movie Database: %w", c.cliErr)
	}

	return c.cli, nil
}

// kept is what read finds, else what fetch answers, which write then keeps.
func kept[T any](read, fetch func() (T, error), write func(T) error) (T, error) {
	v, err := read()
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}

	if v, err = fetch(); err != nil {
		return v, err
	}

	return v, write(v)
}

// normalize is the query a search is kept under: two that differ in case or
// surrounding space find the same.
func normalize(query string) string { return strings.ToLower(strings.TrimSpace(query)) }

// notFound makes The Movie Database's 404 an [fs.ErrNotExist].
func notFound(err error) error {
	var e *api.Error
	if errors.As(err, &e) && e.Response != nil && e.Response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	}

	return err
}

// nullDate is d, or NULL where it is not a date: The Movie Database leaves dates it does not know empty.
func nullDate(d civil.Date) *civil.Date {
	if !d.IsValid() {
		return nil
	}

	return &d
}

func dateOf(d *civil.Date) civil.Date {
	if d == nil {
		return civil.Date{}
	}

	return *d
}
