# Roadmap

## Migrate private-file-vault

Replace its `internal/tmdb` with this package, and its imports of
`themoviedb` with the aliases here. Its kept JSON in
`~/.private-file-vault/tmdb` is a cache and can be dropped.

## Refreshing

Nothing kept is fetched again. A film's popularity and votes drift, and a
running show gains seasons and episodes. Add an age past which a row is
fetched again (`fetched_at` is there for it), shorter for searches and
running shows than for released films.

## One fetch for concurrent first requests

Two goroutines asking for the same film at once both fetch it; the second
write wins and nothing breaks, but it is two requests. `singleflight` per
key would make it one.

## Not-found answers

A 404 is not kept, so asking again for a film The Movie Database lacks asks
it again. Keep negative answers if that becomes frequent.

## Language

Everything is fetched in The Movie Database's default language. Supporting
another means adding the language to every key.
