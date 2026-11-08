-- Queries in sqlc's syntax, one file per table. This is an example, written by
-- devtool with migrations/001_initial.up.sql: replace it with the database's
-- own queries. make generate turns each into a method on DB, named as the
-- comment above it says, returning :one row, :many rows, or nothing (:exec).
-- Keep these files ASCII, comments included.

-- name: CreateNote :one
INSERT INTO notes (title, body, pinned, score, due, remind_at, attachment)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetNote :one
SELECT * FROM notes WHERE id = ?;

-- name: ListNotes :many
SELECT * FROM notes ORDER BY created_at DESC, id DESC;

-- name: DeleteNote :exec
DELETE FROM notes WHERE id = ?;
