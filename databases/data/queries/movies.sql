-- name: GetMovie :one
SELECT * FROM movies WHERE id = ?;

-- name: PutMovie :exec
INSERT INTO movies (
    id, title, original_title, original_language, overview, release_date,
    popularity, vote_average, vote_count, poster_path
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    title = excluded.title,
    original_title = excluded.original_title,
    original_language = excluded.original_language,
    overview = excluded.overview,
    release_date = excluded.release_date,
    popularity = excluded.popularity,
    vote_average = excluded.vote_average,
    vote_count = excluded.vote_count,
    poster_path = excluded.poster_path,
    fetched_at = CURRENT_TIMESTAMP;

-- name: GetMovieSearch :one
SELECT id FROM movie_searches WHERE query = ? AND year = ?;

-- name: PutMovieSearch :one
INSERT INTO movie_searches (query, year) VALUES (?, ?)
ON CONFLICT (query, year) DO UPDATE SET fetched_at = CURRENT_TIMESTAMP
RETURNING id;

-- name: ClearMovieSearchResults :exec
DELETE FROM movie_search_results WHERE search_id = ?;

-- name: AddMovieSearchResult :exec
INSERT INTO movie_search_results (search_id, rank, movie_id) VALUES (?, ?, ?);

-- name: ListMovieSearchResults :many
SELECT sqlc.embed(movies) FROM movie_search_results
JOIN movies ON movies.id = movie_search_results.movie_id
WHERE movie_search_results.search_id = ?
ORDER BY movie_search_results.rank;
