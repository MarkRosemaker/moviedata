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

// Show is show id.
func (c *Client) Show(ctx context.Context, id ShowID) (Show, error) {
	return kept(
		func() ([]byte, error) { return c.db.GetShow(ctx, int64(id)) },
		func() (Show, error) {
			cli, err := c.client()
			if err != nil {
				return Show{}, err
			}

			d, err := cli.GetShow(ctx, id, nil)
			if err != nil {
				return Show{}, fmt.Errorf("getting show %d: %w", id, notFound(err))
			}

			sh := Show{ShowBase: d.ShowBase}
			for _, g := range d.Genres {
				sh.GenreIDs = append(sh.GenreIDs, g.ID)
			}

			return sh, nil
		},
		func(b []byte) error { return c.db.PutShow(ctx, data.PutShowParams{ID: int64(id), Data: b}) },
	)
}

// Season is season n of show id, with its episodes.
func (c *Client) Season(ctx context.Context, id ShowID, n int32) (Season, error) {
	key := data.GetSeasonParams{ShowID: int64(id), SeasonNumber: int64(n)}

	return kept(
		func() ([]byte, error) { return c.db.GetSeason(ctx, key) },
		func() (Season, error) {
			cli, err := c.client()
			if err != nil {
				return Season{}, err
			}

			d, err := cli.GetShowSeason(ctx, id, n, nil)
			if err != nil {
				return Season{}, fmt.Errorf("getting season %d of show %d: %w", n, id, notFound(err))
			}

			return *d, nil
		},
		func(b []byte) error {
			return c.db.PutSeason(ctx, data.PutSeasonParams{ShowID: key.ShowID, SeasonNumber: key.SeasonNumber, Data: b})
		},
	)
}

// Episode is episode e of season n of show id.
func (c *Client) Episode(ctx context.Context, id ShowID, n, e int32) (Episode, error) {
	se, err := c.Season(ctx, id, n)
	if err != nil {
		return Episode{}, err
	}

	for _, ep := range se.Episodes {
		if ep.EpisodeNumber == e {
			return ep, nil
		}
	}

	return Episode{}, fmt.Errorf("episode %d of season %d of show %d: %w", e, n, id, fs.ErrNotExist)
}

// SearchShows is the shows for query, best first.
func (c *Client) SearchShows(ctx context.Context, query string) ([]Show, error) {
	q := normalize(query)

	id, err := c.db.GetShowSearch(ctx, q)
	if err == nil {
		rows, err := c.db.ListShowSearchResults(ctx, id)
		if err != nil {
			return nil, err
		}

		return decodeAll[Show](rows)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	cli, err := c.client()
	if err != nil {
		return nil, err
	}

	res, err := cli.SearchShows(ctx, themoviedb.SearchShowsParams{Query: q})
	if err != nil {
		return nil, fmt.Errorf("searching for the show %q: %w", query, err)
	}

	return res.Results, c.db.WithTx(ctx, func(tx *data.Queries) error {
		id, err := tx.PutShowSearch(ctx, q)
		if err != nil {
			return err
		}

		if err := tx.ClearShowSearchResults(ctx, id); err != nil {
			return err
		}

		for rank, sh := range res.Results {
			b, err := json.Marshal(sh)
			if err != nil {
				return err
			}

			if err := tx.PutShow(ctx, data.PutShowParams{ID: int64(sh.ID), Data: b}); err != nil {
				return err
			}

			if err := tx.AddShowSearchResult(ctx, data.AddShowSearchResultParams{
				SearchID: id, Rank: int64(rank), ShowID: int64(sh.ID),
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

// FindShow is the best match for a show's name: the first result.
func (c *Client) FindShow(ctx context.Context, name string) (Show, bool, error) {
	found, err := c.SearchShows(ctx, name)
	if err != nil || len(found) == 0 {
		return Show{}, false, err
	}

	return found[0], true, nil
}

// ShowPoster is the file the poster of season n of show id is kept in, else
// the show's, for n 0 or a season without one.
func (c *Client) ShowPoster(ctx context.Context, id ShowID, n int32) (string, error) {
	if n > 0 {
		if se, err := c.Season(ctx, id, n); err == nil && se.PosterPath != "" {
			return c.image(ctx, se.PosterPath)
		}
	}

	sh, err := c.Show(ctx, id)
	if err != nil {
		return "", err
	}

	if sh.PosterPath == "" {
		return "", fmt.Errorf("poster of show %d: %w", id, fs.ErrNotExist)
	}

	return c.image(ctx, sh.PosterPath)
}

// Still is the file the still of episode e of season n of show id is kept in.
func (c *Client) Still(ctx context.Context, id ShowID, n, e int32) (string, error) {
	ep, err := c.Episode(ctx, id, n, e)
	if err != nil {
		return "", err
	}

	if ep.StillPath == "" {
		return "", fmt.Errorf("still of episode %d of season %d of show %d: %w", e, n, id, fs.ErrNotExist)
	}

	return c.image(ctx, ep.StillPath)
}
