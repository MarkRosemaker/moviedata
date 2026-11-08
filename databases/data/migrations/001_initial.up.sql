-- TODO: create initial migration here
CREATE TABLE
    IF NOT EXISTS foo (
        id INTEGER PRIMARY KEY,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
