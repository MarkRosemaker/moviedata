-- The database's first state. This is an example, written by devtool when the
-- database was declared: replace the notes table with the database's own
-- tables, and queries/notes.sql with its own queries, before anything relies
-- on it. Once merged, a migration is never edited again: change the schema in
-- the next one, 002_<title>.up.sql.
--
-- It has a column of each type that maps cleanly to Go, with the Go type
-- make generate gives it. Use these types and no others.

CREATE TABLE notes (
    id INTEGER PRIMARY KEY,                               -- int64
    title TEXT NOT NULL,                                  -- string
    body TEXT,                                            -- sql.NullString: NULL allowed
    pinned BOOLEAN NOT NULL DEFAULT FALSE,                -- bool
    score REAL,                                           -- sql.NullFloat64
    due DATE,                                             -- *civil.Date; civil.Date where NOT NULL
    remind_at DATETIME,                                   -- sql.NullTime
    attachment BLOB,                                      -- []byte, nil for NULL
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- time.Time, in UTC
);
