package moviedata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"cloud.google.com/go/civil"
	"github.com/MarkRosemaker/moviedata/databases/data"
	"github.com/go-api-libs/themoviedb/pkg/themoviedb"
)

// MovieID is a film's id at The Movie Database.
type MovieID int32

// Movie is a film, as The Movie Database has it.
type Movie struct {
	ID               MovieID
	Title            string
	OriginalTitle    string
	OriginalLanguage string
	Overview         string
	ReleaseDate      civil.Date // zero where unknown
	Popularity       float64
	VoteAverage      float64
	VoteCount        int
	PosterPath       string // empty where it has none
}

func movieFromAPI(m themoviedb.MovieCommon) Movie {
	return Movie{
		ID: MovieID(m.ID), Title: m.Title, OriginalTitle: m.OriginalTitle, OriginalLanguage: m.OriginalLanguage,
		Overview: m.Overview, ReleaseDate: m.ReleaseDate, Popularity: m.Popularity,
		VoteAverage: m.VoteAverage, VoteCount: m.VoteCount, PosterPath: m.PosterPath,
	}
}

func movieFromRow(r data.Movie) Movie {
	return Movie{
		ID: MovieID(r.ID), Title: r.Title, OriginalTitle: r.OriginalTitle, OriginalLanguage: r.OriginalLanguage,
		Overview: r.Overview, ReleaseDate: dateOf(r.ReleaseDate), Popularity: r.Popularity,
		VoteAverage: r.VoteAverage, VoteCount: int(r.VoteCount), PosterPath: r.PosterPath,
	}
}

func putMovie(ctx context.Context, q *data.Queries, m Movie) error {
	return q.PutMovie(ctx, data.PutMovieParams{
		ID: int64(m.ID), Title: m.Title, OriginalTitle: m.OriginalTitle, OriginalLanguage: m.OriginalLanguage,
		Overview: m.Overview, ReleaseDate: nullDate(m.ReleaseDate), Popularity: m.Popularity,
		VoteAverage: m.VoteAverage, VoteCount: int64(m.VoteCount), PosterPath: m.PosterPath,
	})
}

// Movie is film id.
func (c *Client) Movie(ctx context.Context, id MovieID) (Movie, error) {
	return kept(
		func() (Movie, error) {
			r, err := c.db.GetMovie(ctx, int64(id))

			return movieFromRow(r), err
		},
		func() (Movie, error) {
			cli, err := c.client()
			if err != nil {
				return Movie{}, err
			}

			d, err := cli.GetMovie(ctx, themoviedb.MovieID(id), nil)
			if err != nil {
				return Movie{}, fmt.Errorf("getting film %d: %w", id, notFound(err))
			}

			return movieFromAPI(d.MovieCommon), nil
		},
		func(m Movie) error { return putMovie(ctx, c.db.Queries, m) },
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

		out := make([]Movie, len(rows))
		for i, r := range rows {
			out[i] = movieFromRow(r.Movie)
		}

		return out, nil
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

	out := make([]Movie, len(res.Results))
	for i, m := range res.Results {
		out[i] = movieFromAPI(m.MovieCommon)
	}

	return out, c.db.WithTx(ctx, func(tx *data.Queries) error {
		id, err := tx.PutMovieSearch(ctx, data.PutMovieSearchParams(key))
		if err != nil {
			return err
		}

		if err := tx.ClearMovieSearchResults(ctx, id); err != nil {
			return err
		}

		for rank, m := range out {
			if err := putMovie(ctx, tx, m); err != nil {
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
