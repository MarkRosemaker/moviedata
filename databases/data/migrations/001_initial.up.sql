-- What The Movie Database answered, kept so it is asked once. Each resource is
-- the client's JSON for it, keyed by what identifies it at the API; each search
-- is its normalised query and the ids it found, best first.

CREATE TABLE movies (
    id INTEGER PRIMARY KEY,                                -- int64: the film's id
    data BLOB NOT NULL,                                    -- []byte: themoviedb.Movie as JSON
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- time.Time
);

CREATE TABLE shows (
    id INTEGER PRIMARY KEY,                                -- int64: the show's id
    data BLOB NOT NULL,                                    -- []byte: themoviedb.Show as JSON
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- time.Time
);

CREATE TABLE seasons (
    show_id INTEGER NOT NULL,                               -- int64
    season_number INTEGER NOT NULL,                         -- int64
    data BLOB NOT NULL,                                     -- []byte: themoviedb.SeasonDetails as JSON
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- time.Time
    PRIMARY KEY (show_id, season_number)
);

CREATE TABLE movie_searches (
    id INTEGER PRIMARY KEY,                                 -- int64
    query TEXT NOT NULL,                                    -- string: trimmed and lower-cased
    year INTEGER NOT NULL,                                  -- int64: 0 for any
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- time.Time
    UNIQUE (query, year)
);

CREATE TABLE movie_search_results (
    search_id INTEGER NOT NULL REFERENCES movie_searches (id) ON DELETE CASCADE, -- int64
    rank INTEGER NOT NULL,                                                       -- int64: 0 is best
    movie_id INTEGER NOT NULL REFERENCES movies (id),                            -- int64
    PRIMARY KEY (search_id, rank)
);

CREATE TABLE show_searches (
    id INTEGER PRIMARY KEY,                                 -- int64
    query TEXT NOT NULL,                                    -- string: trimmed and lower-cased
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- time.Time
    UNIQUE (query)
);

CREATE TABLE show_search_results (
    search_id INTEGER NOT NULL REFERENCES show_searches (id) ON DELETE CASCADE, -- int64
    rank INTEGER NOT NULL,                                                      -- int64: 0 is best
    show_id INTEGER NOT NULL REFERENCES shows (id),                             -- int64
    PRIMARY KEY (search_id, rank)
);
