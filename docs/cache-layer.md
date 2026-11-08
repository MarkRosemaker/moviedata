# Building a cache layer over a REST API

What this repository settled, for the next one that puts a database in front
of a `go-api-libs` client.

## Shape

- One devtool library repository per API, with one SQLite database declared
  in `devtool.json`. `Path()` gives `~/.<repo>/<name>.db`; files the API
  serves (images) go in a directory beside it, so one directory is the whole
  cache.
- The root package is the only thing consumers import. Alias the client's
  types (`type Movie = themoviedb.Movie`) and its options
  (`var WithBearer = themoviedb.WithBearer`) there, so no consumer imports
  the client and the client can be swapped behind it.
- Expose the calls consumers actually make, named for what they answer
  (`Movie`, `Find`, `Poster`), not one wrapper per endpoint. Start from a
  consumer's real usage and add calls as the next consumer needs them.

## Schema

- One table per resource, keyed by the endpoint's path parameters
  (`movies(id)`, `seasons(show_id, season_number)`), holding the client's
  type as a JSON `BLOB` and a `fetched_at`. Do not mirror the API's fields in
  columns: the client's type already does, and a migration per API change is
  cost with no reader.
- Add a typed column only when a query needs to filter or sort on it.
- One table per search endpoint, keyed by the normalised query and every
  parameter that changes the answer (`UNIQUE (query, year)`), plus a results
  table of `(search_id, rank, resource_id)` referencing the resource table.
  Writing a search writes its resources too, in one transaction, so a search
  is a set of references, never a copy.
- Keep a search that found nothing: it is an answer.

## Code

- One generic helper does read, else fetch, then write (`kept` in
  `moviedata.go`); each call supplies the three functions. Search calls
  follow one fixed pattern: look up the search, else fetch, then write search
  and resources in `WithTx`.
- Normalise queries before keying them (trim, lower-case) and send the
  normalised form, so the key always matches what was asked.
- Create the API client in `Open`, but keep its error rather than failing:
  a cache without a token still answers what it has.
- Map the API's 404 to `fs.ErrNotExist` at the boundary, so consumers test
  `errors.Is(err, fs.ErrNotExist)` and never import `go-api-libs/api`.
- Write downloaded files through a temporary file and rename, and name them
  by the API's own path where that is unique, so shared files are shared.

## Tests

- Open a real database in `t.TempDir()`; fake the API with an
  `http.RoundTripper` passed through `WithHTTPClient`, counting requests.
- Assert each call twice and the request count once: that is the cache's
  contract.
- Close and reopen without a token to show the answers survive and come from
  the database.

## Traps

- `queries/*.sql` must be ASCII (sqlc cuts by bytes, counts by characters).
- `go mod vendor` after `go get`, or `make generate` fails on an inconsistent
  vendor directory before it reaches sqlc.
- `git rm` of the last file in `queries/` removes the directory; recreate it
  before writing new query files there.
