-- name: GetSeasonPoster :one
SELECT poster_path FROM seasons WHERE show_id = ? AND season_number = ?;

-- name: PutSeason :exec
INSERT INTO seasons (show_id, season_number, poster_path) VALUES (?, ?, ?)
ON CONFLICT (show_id, season_number) DO UPDATE SET
    poster_path = excluded.poster_path,
    fetched_at = CURRENT_TIMESTAMP;

-- name: ClearEpisodes :exec
DELETE FROM episodes WHERE show_id = ? AND season_number = ?;

-- name: PutEpisode :exec
INSERT INTO episodes (
    show_id, season_number, episode_number, name, overview, air_date,
    runtime, vote_average, still_path
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetEpisode :one
SELECT * FROM episodes WHERE show_id = ? AND season_number = ? AND episode_number = ?;
