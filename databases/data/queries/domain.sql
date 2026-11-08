-- name: GetFoo :one
SELECT
    *
FROM
    foo
WHERE
    id = ?;