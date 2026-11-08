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

// ShowID is a show's id at The Movie Database.
type ShowID int32

// Show is a show, as The Movie Database has it.
type Show struct {
	ID               ShowID
	Name             string
	OriginalName     string
	OriginalLanguage string
	Overview         string
	FirstAirDate     civil.Date // zero where unknown
	Popularity       float64
	VoteAverage      float64
	VoteCount        int
	PosterPath       string // empty where it has none
}

func showFromAPI(s themoviedb.ShowBase) Show {
	return Show{
		ID: ShowID(s.ID), Name: s.Name, OriginalName: s.OriginalName, OriginalLanguage: s.OriginalLanguage,
		Overview: s.Overview, FirstAirDate: s.FirstAirDate, Popularity: s.Popularity,
		VoteAverage: s.VoteAverage, VoteCount: s.VoteCount, PosterPath: s.PosterPath,
	}
}

func showFromRow(r data.Show) Show {
	return Show{
		ID: ShowID(r.ID), Name: r.Name, OriginalName: r.OriginalName, OriginalLanguage: r.OriginalLanguage,
		Overview: r.Overview, FirstAirDate: dateOf(r.FirstAirDate), Popularity: r.Popularity,
		VoteAverage: r.VoteAverage, VoteCount: int(r.VoteCount), PosterPath: r.PosterPath,
	}
}

func putShow(ctx context.Context, q *data.Queries, s Show) error {
	return q.PutShow(ctx, data.PutShowParams{
		ID: int64(s.ID), Name: s.Name, OriginalName: s.OriginalName, OriginalLanguage: s.OriginalLanguage,
		Overview: s.Overview, FirstAirDate: nullDate(s.FirstAirDate), Popularity: s.Popularity,
		VoteAverage: s.VoteAverage, VoteCount: int64(s.VoteCount), PosterPath: s.PosterPath,
	})
}

// Episode is an episode of a show, as The Movie Database has it.
type Episode struct {
	ShowID        ShowID
	SeasonNumber  int32
	EpisodeNumber int32
	Name          string
	Overview      string
	AirDate       civil.Date // zero where unknown
	Runtime       int        // minutes
	VoteAverage   float64
	StillPath     string // empty where it has none
}

func episodeFromRow(r data.Episode) Episode {
	return Episode{
		ShowID: ShowID(r.ShowID), SeasonNumber: int32(r.SeasonNumber), EpisodeNumber: int32(r.EpisodeNumber),
		Name: r.Name, Overview: r.Overview, AirDate: dateOf(r.AirDate), Runtime: int(r.Runtime),
		VoteAverage: r.VoteAverage, StillPath: r.StillPath,
	}
}

// Show is show id.
func (c *Client) Show(ctx context.Context, id ShowID) (Show, error) {
	return kept(
		func() (Show, error) {
			r, err := c.db.GetShow(ctx, int64(id))

			return showFromRow(r), err
		},
		func() (Show, error) {
			cli, err := c.client()
			if err != nil {
				return Show{}, err
			}

			d, err := cli.GetShow(ctx, themoviedb.ShowID(id), nil)
			if err != nil {
				return Show{}, fmt.Errorf("getting show %d: %w", id, notFound(err))
			}

			return showFromAPI(d.ShowBase), nil
		},
		func(s Show) error { return putShow(ctx, c.db.Queries, s) },
	)
}

// seasonPoster is the poster path of season n of show id, fetching the season
// and its episodes where it is not kept yet.
func (c *Client) seasonPoster(ctx context.Context, id ShowID, n int32) (string, error) {
	return kept(
		func() (string, error) {
			return c.db.GetSeasonPoster(ctx, data.GetSeasonPosterParams{ShowID: int64(id), SeasonNumber: int64(n)})
		},
		func() (string, error) {
			cli, err := c.client()
			if err != nil {
				return "", err
			}

			d, err := cli.GetShowSeason(ctx, themoviedb.ShowID(id), n, nil)
			if err != nil {
				return "", fmt.Errorf("getting season %d of show %d: %w", n, id, notFound(err))
			}

			return d.PosterPath, c.db.WithTx(ctx, func(tx *data.Queries) error {
				if err := tx.PutSeason(ctx, data.PutSeasonParams{
					ShowID: int64(id), SeasonNumber: int64(n), PosterPath: d.PosterPath,
				}); err != nil {
					return err
				}

				if err := tx.ClearEpisodes(ctx, data.ClearEpisodesParams{ShowID: int64(id), SeasonNumber: int64(n)}); err != nil {
					return err
				}

				for _, ep := range d.Episodes {
					if err := tx.PutEpisode(ctx, data.PutEpisodeParams{
						ShowID: int64(id), SeasonNumber: int64(n), EpisodeNumber: int64(ep.EpisodeNumber),
						Name: ep.Name, Overview: ep.Overview, AirDate: nullDate(ep.AirDate),
						Runtime: int64(ep.Runtime), VoteAverage: ep.VoteAverage, StillPath: ep.StillPath,
					}); err != nil {
						return err
					}
				}

				return nil
			})
		},
		func(string) error { return nil },
	)
}

// Episode is episode e of season n of show id.
func (c *Client) Episode(ctx context.Context, id ShowID, n, e int32) (Episode, error) {
	if _, err := c.seasonPoster(ctx, id, n); err != nil {
		return Episode{}, err
	}

	r, err := c.db.GetEpisode(ctx, data.GetEpisodeParams{ShowID: int64(id), SeasonNumber: int64(n), EpisodeNumber: int64(e)})
	if errors.Is(err, sql.ErrNoRows) {
		return Episode{}, fmt.Errorf("episode %d of season %d of show %d: %w", e, n, id, fs.ErrNotExist)
	}

	return episodeFromRow(r), err
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

		out := make([]Show, len(rows))
		for i, r := range rows {
			out[i] = showFromRow(r.Show)
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

	res, err := cli.SearchShows(ctx, themoviedb.SearchShowsParams{Query: q})
	if err != nil {
		return nil, fmt.Errorf("searching for the show %q: %w", query, err)
	}

	out := make([]Show, len(res.Results))
	for i, s := range res.Results {
		out[i] = showFromAPI(s.ShowBase)
	}

	return out, c.db.WithTx(ctx, func(tx *data.Queries) error {
		id, err := tx.PutShowSearch(ctx, q)
		if err != nil {
			return err
		}

		if err := tx.ClearShowSearchResults(ctx, id); err != nil {
			return err
		}

		for rank, s := range out {
			if err := putShow(ctx, tx, s); err != nil {
				return err
			}

			if err := tx.AddShowSearchResult(ctx, data.AddShowSearchResultParams{
				SearchID: id, Rank: int64(rank), ShowID: int64(s.ID),
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
		if p, err := c.seasonPoster(ctx, id, n); err == nil && p != "" {
			return c.image(ctx, p)
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
