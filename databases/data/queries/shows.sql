-- name: GetShow :one
SELECT data FROM shows WHERE id = ?;

-- name: PutShow :exec
INSERT INTO shows (id, data) VALUES (?, ?)
ON CONFLICT (id) DO UPDATE SET data = excluded.data, fetched_at = CURRENT_TIMESTAMP;

-- name: GetSeason :one
SELECT data FROM seasons WHERE show_id = ? AND season_number = ?;

-- name: PutSeason :exec
INSERT INTO seasons (show_id, season_number, data) VALUES (?, ?, ?)
ON CONFLICT (show_id, season_number) DO UPDATE SET data = excluded.data, fetched_at = CURRENT_TIMESTAMP;

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
SELECT s.data FROM show_search_results r
JOIN shows s ON s.id = r.show_id
WHERE r.search_id = ?
ORDER BY r.rank;
