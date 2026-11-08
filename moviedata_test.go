package moviedata_test

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MarkRosemaker/moviedata"
)

// fake answers as The Movie Database would, and counts what it is asked.
type fake struct{ searches, seasons, movies, images atomic.Int32 }

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	status, body, typ := http.StatusOK, `{}`, "application/json"

	switch p := r.URL.Path; {
	case r.URL.Host == "image.tmdb.org":
		f.images.Add(1)

		body, typ = "JPEG", "image/jpeg"
	case strings.HasSuffix(p, "/search/movie") && r.URL.Query().Get("year") != "":
		f.searches.Add(1)

		body = `{"page":1,"results":[],"total_pages":0,"total_results":0}`
	case strings.HasSuffix(p, "/search/movie"):
		f.searches.Add(1)

		body = `{"page":1,"results":[{"id":122,"title":"The Lord of the Rings: The Return of the King",` +
			`"original_title":"The Lord of the Rings: The Return of the King","overview":"Aragorn is revealed...",` +
			`"popularity":31.5,"poster_path":"/rCzpDGLbOoPwLjy3OAm5NUPOTrC.jpg","release_date":"2003-12-01",` +
			`"adult":false,"genre_ids":[12],"original_language":"en","video":false,"vote_average":8.5,"vote_count":25000,"softcore":false}],` +
			`"total_pages":1,"total_results":1}`
	case strings.HasSuffix(p, "/movie/603"):
		f.movies.Add(1)

		body = `{"id":603,"title":"The Matrix","original_title":"The Matrix","overview":"","popularity":1,` +
			`"release_date":"1999-03-31","adult":false,"original_language":"en","video":false,"vote_average":8,"vote_count":1,` +
			`"softcore":false,"genres":[{"id":28,"name":"Action"}]}`
	case strings.HasSuffix(p, "/movie/604"):
		body = `{"id":604,"title":"Untitled","original_title":"Untitled","overview":"","popularity":0,` +
			`"adult":false,"original_language":"en","video":false,"vote_average":0,"vote_count":0,"softcore":false}`
	case strings.HasSuffix(p, "/search/tv"):
		f.searches.Add(1)

		body = `{"page":1,"results":[{"id":65494,"name":"The Crown","overview":"The gripping, decades-spanning inside story of Her Majesty Queen Elizabeth II.",` +
			`"first_air_date":"2016-11-04","popularity":80.1,"poster_path":"/crown.jpg","vote_average":8.2,"vote_count":5000,` +
			`"original_name":"The Crown","original_language":"en","origin_country":["GB"],"adult":false,"softcore":false,"genre_ids":[18]}],` +
			`"total_pages":1,"total_results":1}`
	case strings.HasSuffix(p, "/tv/65494/season/1"):
		f.seasons.Add(1)

		body = `{"_id":"x","id":81166,"name":"Season 1","air_date":"2016-11-04","overview":"","poster_path":"/season1.jpg","season_number":1,"vote_average":8,` +
			`"networks":[],"episodes":[{"id":1180924,"season_number":1,"episode_number":2,"name":"Hyde Park Corner",` +
			`"overview":"Elizabeth and Philip tour the Commonwealth.","air_date":"2016-11-04","production_code":"","runtime":58,` +
			`"still_path":"/still.jpg","vote_average":7.9,"vote_count":60,"show_id":65494,"episode_type":"standard"}]}`
	default:
		status, body = http.StatusNotFound, `{"success":false,"status_code":34,"status_message":"The resource you requested could not be found."}`
	}

	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {typ}},
		Body: io.NopCloser(strings.NewReader(body)), Request: r,
	}, nil
}

func open(t *testing.T, path string, opts ...moviedata.ClientOption) *moviedata.Client {
	t.Helper()

	c, err := moviedata.Open(t.Context(), path, opts...)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = c.Close() })

	return c
}

func withFake(f *fake) []moviedata.ClientOption {
	return []moviedata.ClientOption{moviedata.WithBearer("test"), moviedata.WithHTTPClient(&http.Client{Transport: f})}
}

// A film found is kept, so its details, its search and its poster are answered
// from the database and the disk after the first time, even without a token.
func TestMovies(t *testing.T) {
	ctx := t.Context()
	f := &fake{}
	path := filepath.Join(t.TempDir(), "data.db")
	c := open(t, path, withFake(f)...)

	// No film of that title in 2003 by this fake's say, so the title alone is searched too.
	for range 2 {
		m, ok, err := c.Find(ctx, "The Lord of the Rings 3 The Return of the King", 2003)
		if err != nil || !ok || m.ID != 122 {
			t.Fatalf("found %+v, %v, %v", m, ok, err)
		}
	}

	if found, err := c.Search(ctx, "  the lord of the rings 3 the RETURN of the king "); err != nil || len(found) != 1 {
		t.Fatalf("searched %+v, %v", found, err)
	}

	if n := f.searches.Load(); n != 2 {
		t.Errorf("searched %d times, want twice", n)
	}

	kept, err := c.Movie(ctx, 122)
	if err != nil || kept.Overview != "Aragorn is revealed..." || kept.ReleaseDate.Year != 2003 {
		t.Fatalf("kept %+v, %v", kept, err)
	}

	for range 2 {
		m, err := c.Movie(ctx, 603)
		if err != nil || m.Title != "The Matrix" || m.ReleaseDate.Year != 1999 {
			t.Fatalf("got %+v, %v", m, err)
		}
	}

	if n := f.movies.Load(); n != 1 {
		t.Errorf("fetched the film %d times, want once", n)
	}

	for range 2 {
		if m, err := c.Movie(ctx, 604); err != nil || m.ReleaseDate.IsValid() {
			t.Errorf("a film without a release date: %+v, %v", m, err)
		}
	}

	if _, err := c.Movie(ctx, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a film The Movie Database does not have: %v", err)
	}

	for range 2 {
		p, err := c.Poster(ctx, 122)
		if err != nil {
			t.Fatal(err)
		}

		if b, _ := os.ReadFile(p); string(b) != "JPEG" || p != filepath.Join(filepath.Dir(path), "images", "rCzpDGLbOoPwLjy3OAm5NUPOTrC.jpg") {
			t.Errorf("poster %s holds %q", p, b)
		}
	}

	if n := f.images.Load(); n != 1 {
		t.Errorf("the poster was fetched %d times, want once", n)
	}

	if _, err := c.Poster(ctx, 603); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a film without a poster: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMDB_API_TOKEN", "")

	offline := open(t, path)

	if m, ok, err := offline.Find(ctx, "The Lord of the Rings 3 The Return of the King", 2003); err != nil || !ok || m.ID != 122 {
		t.Errorf("found offline %+v, %v, %v", m, ok, err)
	}

	if _, err := offline.Poster(ctx, 122); err != nil {
		t.Errorf("poster offline: %v", err)
	}

	if _, err := offline.Movie(ctx, 13); err == nil {
		t.Error("a film never fetched was answered without a token")
	}
}

// A show, its season and the season's poster and an episode's still are kept once fetched.
func TestShows(t *testing.T) {
	ctx := t.Context()
	f := &fake{}
	c := open(t, filepath.Join(t.TempDir(), "data.db"), withFake(f)...)

	for range 2 {
		sh, ok, err := c.FindShow(ctx, "The Crown")
		if err != nil || !ok || sh.ID != 65494 {
			t.Fatalf("found %+v, %v, %v", sh, ok, err)
		}
	}

	if f.searches.Load() != 1 {
		t.Errorf("searched %d times, want once", f.searches.Load())
	}

	if kept, err := c.Show(ctx, 65494); err != nil || kept.FirstAirDate.Year != 2016 {
		t.Errorf("kept %+v, %v", kept, err)
	}

	for range 2 {
		ep, err := c.Episode(ctx, 65494, 1, 2)
		if err != nil || ep.Name != "Hyde Park Corner" || ep.Runtime != 58 || ep.AirDate.Year != 2016 {
			t.Fatalf("episode %+v, %v", ep, err)
		}
	}

	if _, err := c.Episode(ctx, 65494, 1, 99); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an episode the season does not have: %v", err)
	}

	for _, get := range []func() (string, error){
		func() (string, error) { return c.ShowPoster(ctx, 65494, 1) },
		func() (string, error) { return c.ShowPoster(ctx, 65494, 1) },
		func() (string, error) { return c.Still(ctx, 65494, 1, 2) },
		func() (string, error) { return c.ShowPoster(ctx, 65494, 0) },
	} {
		p, err := get()
		if err != nil {
			t.Fatal(err)
		}

		if b, _ := os.ReadFile(p); string(b) != "JPEG" {
			t.Errorf("%s holds %q", p, b)
		}
	}

	if f.seasons.Load() != 1 || f.images.Load() != 3 {
		t.Errorf("asked for the season %d times and images %d times, want once and three times", f.seasons.Load(), f.images.Load())
	}
}
