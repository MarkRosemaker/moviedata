package moviedata

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"

	"github.com/MarkRosemaker/moviedata/databases/data"
	"github.com/go-api-libs/themoviedb/pkg/themoviedb"
)

// Movie is film id.
func (c *Client) Movie(ctx context.Context, id MovieID) (Movie, error) {
	return kept(
		func() ([]byte, error) { return c.db.GetMovie(ctx, int64(id)) },
		func() (Movie, error) {
			cli, err := c.client()
			if err != nil {
				return Movie{}, err
			}

			d, err := cli.GetMovie(ctx, id, nil)
			if err != nil {
				return Movie{}, fmt.Errorf("getting film %d: %w", id, notFound(err))
			}

			m := Movie{MovieCommon: d.MovieCommon}
			for _, g := range d.Genres {
				m.GenreIDs = append(m.GenreIDs, g.ID)
			}

			return m, nil
		},
		func(b []byte) error { return c.db.PutMovie(ctx, data.PutMovieParams{ID: int64(id), Data: b}) },
	)
}

// Search is the films for query, best first.
func (c *Client) Search(ctx context.Context, query string) ([]Movie, error) {
	return c.searchMovies(ctx, query, 0)
}

// Find is the best match for a film's title and year (0 for unknown): the
// first result for both, else for the title alone.
func (c *Client) Find(ctx context.Context, title string, year int) (Movie, bool, error) {
	for _, y := range []int{year, 0} {
		found, err := c.searchMovies(ctx, title, y)
		if err != nil {
			return Movie{}, false, err
		}

		if len(found) > 0 {
			return found[0], true, nil
		}

		if y == 0 {
			break
		}
	}

	return Movie{}, false, nil
}

// Poster is the file film id's poster is kept in.
func (c *Client) Poster(ctx context.Context, id MovieID) (string, error) {
	m, err := c.Movie(ctx, id)
	if err != nil {
		return "", err
	}

	if m.PosterPath == "" {
		return "", fmt.Errorf("poster of film %d: %w", id, fs.ErrNotExist)
	}

	return c.image(ctx, m.PosterPath)
}

// searchMovies is the films for query in year (0 for any), kept with each film.
func (c *Client) searchMovies(ctx context.Context, query string, year int) ([]Movie, error) {
	key := data.GetMovieSearchParams{Query: normalize(query), Year: int64(year)}

	id, err := c.db.GetMovieSearch(ctx, key)
	if err == nil {
		rows, err := c.db.ListMovieSearchResults(ctx, id)
		if err != nil {
			return nil, err
		}

		return decodeAll[Movie](rows)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	cli, err := c.client()
	if err != nil {
		return nil, err
	}

	res, err := cli.SearchMovies(ctx, themoviedb.SearchMoviesParams{Query: key.Query, Year: year})
	if err != nil {
		return nil, fmt.Errorf("searching for the film %q: %w", query, err)
	}

	return res.Results, c.db.WithTx(ctx, func(tx *data.Queries) error {
		id, err := tx.PutMovieSearch(ctx, data.PutMovieSearchParams(key))
		if err != nil {
			return err
		}

		if err := tx.ClearMovieSearchResults(ctx, id); err != nil {
			return err
		}

		for rank, m := range res.Results {
			b, err := json.Marshal(m)
			if err != nil {
				return err
			}

			if err := tx.PutMovie(ctx, data.PutMovieParams{ID: int64(m.ID), Data: b}); err != nil {
				return err
			}

			if err := tx.AddMovieSearchResult(ctx, data.AddMovieSearchResultParams{
				SearchID: id, Rank: int64(rank), MovieID: int64(m.ID),
			}); err != nil {
				return err
			}
		}

		return nil
	})
}
