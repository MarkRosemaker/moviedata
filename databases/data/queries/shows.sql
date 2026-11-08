-- name: GetShow :one
SELECT * FROM shows WHERE id = ?;

-- name: PutShow :exec
INSERT INTO shows (
    id, name, original_name, original_language, overview, first_air_date,
    popularity, vote_average, vote_count, poster_path
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    original_name = excluded.original_name,
    original_language = excluded.original_language,
    overview = excluded.overview,
    first_air_date = excluded.first_air_date,
    popularity = excluded.popularity,
    vote_average = excluded.vote_average,
    vote_count = excluded.vote_count,
    poster_path = excluded.poster_path,
    fetched_at = CURRENT_TIMESTAMP;

-- name: GetShowSearch :one
SELECT id FROM show_searches WHERE query = ?;

-- name: PutShowSearch :one
INSERT INTO show_searches (query) VALUES (?)
ON CONFLICT (query) DO UPDATE SET fetched_at = CURRENT_TIMESTAMP
RETURNING id;

-- name: ClearShowSearchResults :exec
DELETE FROM show_search_results WHERE search_id = ?;

-- name: AddShowSearchResult :exec
INSERT INTO show_search_results (search_id, rank, show_id) VALUES (?, ?, ?);

-- name: ListShowSearchResults :many
SELECT sqlc.embed(shows) FROM show_search_results
JOIN shows ON shows.id = show_search_results.show_id
WHERE show_search_results.search_id = ?
ORDER BY show_search_results.rank;
