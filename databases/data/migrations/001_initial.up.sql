-- What The Movie Database answered, kept so it is asked once: the fields of
-- each film, show, season and episode that are read, and each search as its
-- normalised query and the ids it found, best first.

CREATE TABLE movies (
    id INTEGER PRIMARY KEY,                                -- int64
    title TEXT NOT NULL,                                   -- string
    original_title TEXT NOT NULL,                          -- string
    original_language TEXT NOT NULL,                       -- string
    overview TEXT NOT NULL,                                -- string
    release_date DATE,                                     -- *civil.Date: NULL where unknown
    popularity REAL NOT NULL,                              -- float64
    vote_average REAL NOT NULL,                            -- float64
    vote_count INTEGER NOT NULL,                           -- int64
    poster_path TEXT NOT NULL,                             -- string: empty where it has none
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- time.Time
);

CREATE TABLE shows (
    id INTEGER PRIMARY KEY,                                -- int64
    name TEXT NOT NULL,                                    -- string
    original_name TEXT NOT NULL,                           -- string
    original_language TEXT NOT NULL,                       -- string
    overview TEXT NOT NULL,                                -- string
    first_air_date DATE,                                   -- *civil.Date: NULL where unknown
    popularity REAL NOT NULL,                              -- float64
    vote_average REAL NOT NULL,                            -- float64
    vote_count INTEGER NOT NULL,                           -- int64
    poster_path TEXT NOT NULL,                             -- string: empty where it has none
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP -- time.Time
);

-- A season kept is one whose episodes are all kept, so an episode it lacks
-- does not exist.
CREATE TABLE seasons (
    show_id INTEGER NOT NULL,                               -- int64
    season_number INTEGER NOT NULL,                         -- int64
    poster_path TEXT NOT NULL,                              -- string: empty where it has none
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- time.Time
    PRIMARY KEY (show_id, season_number)
);

CREATE TABLE episodes (
    show_id INTEGER NOT NULL,        -- int64
    season_number INTEGER NOT NULL,  -- int64
    episode_number INTEGER NOT NULL, -- int64
    name TEXT NOT NULL,              -- string
    overview TEXT NOT NULL,          -- string
    air_date DATE,                   -- *civil.Date: NULL where unknown
    runtime INTEGER NOT NULL,        -- int64: minutes
    vote_average REAL NOT NULL,      -- float64
    still_path TEXT NOT NULL,        -- string: empty where it has none
    PRIMARY KEY (show_id, season_number, episode_number),
    FOREIGN KEY (show_id, season_number) REFERENCES seasons (show_id, season_number) ON DELETE CASCADE
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
