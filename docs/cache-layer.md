# Building a cache layer over a REST API

What this repository settled, for the next one that puts a database in front
of a `go-api-libs` client.

## Shape

- One devtool library repository per API, with one SQLite database declared
  in `devtool.json`. `Path()` gives `~/.<repo>/<name>.db`; files the API
  serves (images) go in a directory beside it, so one directory is the whole
  cache.
- The root package is the only thing consumers import. It defines its own
  result types (`Movie`, `Show`, `Episode`) with only the fields consumers
  read, and takes no client options: what consumers would pass is fixed
  inside (here the debug file, `debug/themoviedb.json`), so no consumer
  imports the client.
- Expose the calls consumers actually make, named for what they answer
  (`Movie`, `Find`, `Poster`), not one wrapper per endpoint. Start from a
  consumer's real usage and add calls as the next consumer needs them.

## Schema

- One table per resource, keyed by the endpoint's path parameters
  (`movies(id)`, `seasons(show_id, season_number)`), with a typed column for
  each field a consumer reads and a `fetched_at`. No JSON blobs, and no
  columns for fields nobody reads: list the fields from the consumer's code
  (`grep` its uses of the client's types) before writing the migration.
- Store an API's empty date as NULL (`*civil.Date` from sqlc), not a zero
  date, and map it back to the zero value at the boundary.
- A nested list the API returns whole (a season's episodes) gets a child
  table written in the same transaction as its parent, so the parent's row
  means "all children are kept" and a missing child is a real not-found.
- One table per search endpoint, keyed by the normalised query and every
  parameter that changes the answer (`UNIQUE (query, year)`), plus a results
  table of `(search_id, rank, resource_id)` referencing the resource table.
  Writing a search writes its resources too, in one transaction, so a search
  is a set of references, never a copy.
- Keep a search that found nothing: it is an answer.

## Code

- One generic helper does read, else fetch, then write (`kept` in
  `moviedata.go`); each call supplies the three functions. Per resource, write
  three small mappers: from the API type, from the database row, and to the
  insert parameters.
- Use `sqlc.embed(table)` in a join so a search's results scan into the
  table's own model. Search calls
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
  `http.RoundTripper` passed through the client's own `WithHTTPClient`,
  counting requests. The test is in the package itself and calls the
  unexported `open`, which takes the client's options that `Open` does not.
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
