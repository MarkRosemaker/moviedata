-- name: GetMovie :one
SELECT data FROM movies WHERE id = ?;

-- name: PutMovie :exec
INSERT INTO movies (id, data) VALUES (?, ?)
ON CONFLICT (id) DO UPDATE SET data = excluded.data, fetched_at = CURRENT_TIMESTAMP;

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
SELECT m.data FROM movie_search_results r
JOIN movies m ON m.id = r.movie_id
WHERE r.search_id = ?
ORDER BY r.rank;
