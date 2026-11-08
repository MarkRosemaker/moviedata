### What it is

A cache in front of [The Movie Database](https://www.themoviedb.org), through
its Go client [go-api-libs/themoviedb](https://github.com/go-api-libs/themoviedb).
Each film, show, season and episode is fetched once and the fields of it that
are read are kept in a SQLite database, each
search is kept as its query and the ids it found, and each poster and still is
downloaded once into the `images` directory beside the database. After that,
the answer comes from there, offline and without a token.

A program uses this package, never `themoviedb` directly: `Movie`, `Show` and
`Episode` are this package's own, and it takes no client options: responses
the client cannot decode go to `debug/themoviedb.json`.

```go
path, _ := moviedata.Path() // ~/.moviedata/data.db
c, err := moviedata.Open(ctx, path) // reads TMDB_API_TOKEN to fetch what is not kept
if err != nil {
	return err
}
defer c.Close()

m, ok, err := c.Find(ctx, "The Matrix", 1999)
poster, err := c.Poster(ctx, m.ID) // a file under ~/.moviedata/images
```

What does not exist, at The Movie Database or in what it answered, is an error
that is `fs.ErrNotExist`.
