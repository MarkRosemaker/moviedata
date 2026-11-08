// Package moviedata looks films and shows up at The Movie Database and keeps
// what it finds: their details and searches in a SQLite database, their images
// as files beside it. What is once fetched is answered from there ever after,
// so a program asks this package, never The Movie Database's client directly.
package moviedata

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/MarkRosemaker/moviedata/databases/data"
	"github.com/go-api-libs/api"
	"github.com/go-api-libs/themoviedb/pkg/themoviedb"
)

// The Movie Database's types, so a program needs no import of its client.
type (
	Movie        = themoviedb.Movie
	MovieID      = themoviedb.MovieID
	Show         = themoviedb.Show
	ShowID       = themoviedb.ShowID
	Season       = themoviedb.SeasonDetails
	Episode      = themoviedb.ShowEpisodeDetails
	ClientOption = themoviedb.ClientOption
)

// Options for the client that fetches what is not kept yet.
var (
	WithBearer     = themoviedb.WithBearer
	WithHTTPClient = themoviedb.WithHTTPClient
	WithBaseURL    = themoviedb.WithBaseURL
	WithDebugFile  = themoviedb.WithDebugFile
)

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
// Without TMDB_API_TOKEN set and no [WithBearer] among opts it still opens:
// what is kept is answered, and fetching what is not fails.
func Open(ctx context.Context, path string, opts ...ClientOption) (*Client, error) {
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

// kept is what read finds and decodes, else what fetch answers, which write
// then keeps.
func kept[T any](read func() ([]byte, error), fetch func() (T, error), write func([]byte) error) (T, error) {
	var v T

	b, err := read()
	if err == nil {
		return v, json.Unmarshal(b, &v)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}

	if v, err = fetch(); err != nil {
		return v, err
	}

	if b, err = json.Marshal(v); err != nil {
		return v, err
	}

	return v, write(b)
}

func decodeAll[T any](rows [][]byte) ([]T, error) {
	out := make([]T, len(rows))
	for i, b := range rows {
		if err := json.Unmarshal(b, &out[i]); err != nil {
			return nil, err
		}
	}

	return out, nil
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
