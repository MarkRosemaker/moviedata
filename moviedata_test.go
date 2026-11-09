package moviedata

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-api-libs/themoviedb/pkg/themoviedb"
)

// fake answers as The Movie Database would, from routes keyed by path (and
// "?year=" where a search gives one; images by "img" and their path), and
// counts what it is asked. A path it has no route for is a 404.
type fake struct {
	mu     sync.Mutex
	routes map[string]route
	calls  map[string]int
}

type route struct {
	status int
	body   string
}

func newFake(routes map[string]route) *fake {
	return &fake{routes: routes, calls: map[string]int{}}
}

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	key, typ := strings.TrimPrefix(r.URL.Path, "/3"), "application/json"
	if r.URL.Host == "image.tmdb.org" {
		key, typ = "img"+strings.TrimPrefix(r.URL.Path, "/t/p/original"), "image/jpeg"
	} else if y := r.URL.Query().Get("year"); y != "" {
		key += "?year=" + y
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls[key]++

	rt, ok := f.routes[key]
	if !ok {
		rt = route{http.StatusNotFound, `{"success":false,"status_code":34,"status_message":"The resource you requested could not be found."}`}
	}

	if rt.status == 0 {
		rt.status = http.StatusOK
	}

	return &http.Response{
		StatusCode: rt.status, Header: http.Header{"Content-Type": {typ}},
		Body: io.NopCloser(strings.NewReader(rt.body)), Request: r,
	}, nil
}

func (f *fake) set(key string, rt route) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.routes[key] = rt
}

// asked is how often key was asked for.
func (f *fake) asked(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls[key]
}

func (f *fake) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0
	for _, c := range f.calls {
		n += c
	}

	return n
}

const (
	returnOfTheKing = `{"id":122,"title":"The Lord of the Rings: The Return of the King",` +
		`"original_title":"The Lord of the Rings: The Return of the King","overview":"Aragorn is revealed...",` +
		`"popularity":31.5,"poster_path":"/rCzpDGLbOoPwLjy3OAm5NUPOTrC.jpg","release_date":"2003-12-01",` +
		`"adult":false,"genre_ids":[12],"original_language":"en","video":false,"vote_average":8.5,"vote_count":25000,"softcore":false}`
	theMatrix = `{"id":603,"title":"The Matrix","original_title":"The Matrix","overview":"","popularity":1,` +
		`"release_date":"1999-03-31","adult":false,"original_language":"en","video":false,"vote_average":8,"vote_count":1,` +
		`"softcore":false,"genres":[{"id":28,"name":"Action"}]}`
	untitled = `{"id":604,"title":"Untitled","original_title":"Untitled","overview":"","popularity":0,` +
		`"adult":false,"original_language":"en","video":false,"vote_average":0,"vote_count":0,"softcore":false}`
	theCrown = `{"id":65494,"name":"The Crown","overview":"The gripping, decades-spanning inside story of Her Majesty Queen Elizabeth II.",` +
		`"first_air_date":"2016-11-04","popularity":80.1,"poster_path":"/crown.jpg","vote_average":8.2,"vote_count":5000,` +
		`"original_name":"The Crown","original_language":"en","origin_country":["GB"],"adult":false,"softcore":false,"genre_ids":[18]}`
	gameOfThrones = `{"id":1399,"name":"Game of Thrones","overview":"Seven noble families fight for control.",` +
		`"first_air_date":"2011-04-17","popularity":300,"vote_average":8.4,"vote_count":20000,` +
		`"original_name":"Game of Thrones","original_language":"en","origin_country":["US"],"adult":false,"softcore":false,` +
		`"genres":[{"id":18,"name":"Drama"}]}`
	crownSeason1 = `{"_id":"x","id":81166,"name":"Season 1","air_date":"2016-11-04","overview":"","poster_path":"/season1.jpg","season_number":1,"vote_average":8,` +
		`"networks":[],"episodes":[{"id":1180924,"season_number":1,"episode_number":2,"name":"Hyde Park Corner",` +
		`"overview":"Elizabeth and Philip tour the Commonwealth.","air_date":"2016-11-04","production_code":"","runtime":58,` +
		`"still_path":"/still.jpg","vote_average":7.9,"vote_count":60,"show_id":65494,"episode_type":"standard"}]}`
	thronesSeason1 = `{"_id":"y","id":3624,"name":"Season 1","air_date":"2011-04-17","overview":"","season_number":1,"vote_average":8,` +
		`"networks":[],"episodes":[{"id":63056,"season_number":1,"episode_number":1,"name":"Winter Is Coming",` +
		`"overview":"","air_date":"2011-04-17","production_code":"","runtime":62,` +
		`"vote_average":8,"vote_count":300,"show_id":1399,"episode_type":"standard"}]}`
)

func results(items ...string) string {
	return `{"page":1,"results":[` + strings.Join(items, ",") + `],"total_pages":1,"total_results":` +
		map[bool]string{true: "1", false: "0"}[len(items) > 0] + `}`
}

func openTest(t *testing.T, path string, opts ...themoviedb.ClientOption) *Client {
	t.Helper()

	c, err := open(t.Context(), path, opts...)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = c.Close() })

	return c
}

func withFake(f *fake) []themoviedb.ClientOption {
	return []themoviedb.ClientOption{themoviedb.WithBearer("test"), themoviedb.WithHTTPClient(&http.Client{Transport: f})}
}

func newDB(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "data.db")
}

// A film found is kept, so its details, its search and its poster are answered
// from the database and the disk after the first time, even without a token.
func TestMovies(t *testing.T) {
	ctx := t.Context()
	f := newFake(map[string]route{
		"/search/movie?year=2003":             {body: results()},
		"/search/movie":                       {body: results(returnOfTheKing)},
		"/movie/603":                          {body: theMatrix},
		"/movie/604":                          {body: untitled},
		"img/rCzpDGLbOoPwLjy3OAm5NUPOTrC.jpg": {body: "JPEG"},
	})
	path := newDB(t)
	c := openTest(t, path, withFake(f)...)

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

	if n := f.asked("/search/movie?year=2003") + f.asked("/search/movie"); n != 2 {
		t.Errorf("searched %d times, want twice", n)
	}

	kept, err := c.Movie(ctx, 122)
	if err != nil || kept.Overview != "Aragorn is revealed..." || kept.ReleaseDate.Year != 2003 || kept.VoteCount != 25000 {
		t.Fatalf("kept %+v, %v", kept, err)
	}

	for range 2 {
		m, err := c.Movie(ctx, 603)
		if err != nil || m.Title != "The Matrix" || m.ReleaseDate.Year != 1999 {
			t.Fatalf("got %+v, %v", m, err)
		}
	}

	if n := f.asked("/movie/603"); n != 1 {
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

	if n := f.asked("img/rCzpDGLbOoPwLjy3OAm5NUPOTrC.jpg"); n != 1 {
		t.Errorf("the poster was fetched %d times, want once", n)
	}

	if _, err := c.Poster(ctx, 603); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a film without a poster: %v", err)
	}

	if _, err := c.Poster(ctx, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the poster of a film The Movie Database does not have: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMDB_API_TOKEN", "")

	offline := openTest(t, path)

	if m, ok, err := offline.Find(ctx, "The Lord of the Rings 3 The Return of the King", 2003); err != nil || !ok || m.ID != 122 {
		t.Errorf("found offline %+v, %v, %v", m, ok, err)
	}

	if _, err := offline.Poster(ctx, 122); err != nil {
		t.Errorf("poster offline: %v", err)
	}

	if _, err := offline.Movie(ctx, 13); err == nil {
		t.Error("a film never fetched was answered without a token")
	}

	if _, err := offline.Search(ctx, "Forrest Gump"); err == nil {
		t.Error("a search never made was answered without a token")
	}
}

// Find takes the search with the year where it finds something, and keeps a
// search that found nothing as an answer.
func TestFind(t *testing.T) {
	ctx := t.Context()
	f := newFake(map[string]route{
		"/search/movie?year=2003": {body: results(returnOfTheKing)},
	})
	c := openTest(t, newDB(t), withFake(f)...)

	if m, ok, err := c.Find(ctx, "The Return of the King", 2003); err != nil || !ok || m.ID != 122 {
		t.Fatalf("found %+v, %v, %v", m, ok, err)
	}

	if n := f.asked("/search/movie"); n != 0 {
		t.Errorf("searched without the year %d times, want never", n)
	}

	f.set("/search/movie", route{body: results()})

	for range 2 {
		if m, ok, err := c.Find(ctx, "No Such Film", 0); err != nil || ok {
			t.Fatalf("found %+v, %v, %v", m, ok, err)
		}
	}

	if n := f.asked("/search/movie"); n != 1 {
		t.Errorf("searched for nothing %d times, want once", n)
	}
}

// What fails is not kept: the next call asks again.
func TestFailuresAreNotKept(t *testing.T) {
	ctx := t.Context()
	broken := route{http.StatusInternalServerError, `{"success":false,"status_code":11,"status_message":"Internal error."}`}
	f := newFake(map[string]route{
		"/movie/603":    broken,
		"/search/movie": broken,
		"/search/tv":    broken,
		"/tv/65494":     {body: theCrown},
		"img/crown.jpg": broken,
	})
	c := openTest(t, newDB(t), withFake(f)...)

	if _, err := c.Movie(ctx, 603); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a failing film: %v", err)
	}

	if _, err := c.Search(ctx, "The Matrix"); err == nil {
		t.Error("a failing search was answered")
	}

	if _, _, err := c.FindShow(ctx, "The Crown"); err == nil {
		t.Error("a failing show search was answered")
	}

	if _, err := c.ShowPoster(ctx, 65494, 0); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a failing poster: %v", err)
	}

	f.set("/movie/603", route{body: theMatrix})
	f.set("/search/movie", route{body: results(strings.Replace(theMatrix, `"genres":[{"id":28,"name":"Action"}]`, `"genre_ids":[28]`, 1))})
	f.set("/search/tv", route{body: results(theCrown)})
	f.set("img/crown.jpg", route{body: "JPEG"})

	if m, err := c.Movie(ctx, 603); err != nil || m.ID != 603 {
		t.Errorf("film after the failure: %+v, %v", m, err)
	}

	if found, err := c.Search(ctx, "The Matrix"); err != nil || len(found) != 1 {
		t.Errorf("search after the failure: %+v, %v", found, err)
	}

	if sh, ok, err := c.FindShow(ctx, "The Crown"); err != nil || !ok || sh.ID != 65494 {
		t.Errorf("show search after the failure: %+v, %v, %v", sh, ok, err)
	}

	if _, err := c.ShowPoster(ctx, 65494, 0); err != nil {
		t.Errorf("poster after the failure: %v", err)
	}
}

// A show, its season and the season's poster and an episode's still are kept once fetched.
func TestShows(t *testing.T) {
	ctx := t.Context()
	f := newFake(map[string]route{
		"/search/tv":         {body: results(theCrown)},
		"/tv/65494/season/1": {body: crownSeason1},
		"img/season1.jpg":    {body: "JPEG"},
		"img/still.jpg":      {body: "JPEG"},
		"img/crown.jpg":      {body: "JPEG"},
	})
	path := newDB(t)
	c := openTest(t, path, withFake(f)...)

	for range 2 {
		sh, ok, err := c.FindShow(ctx, "The Crown")
		if err != nil || !ok || sh.ID != 65494 {
			t.Fatalf("found %+v, %v, %v", sh, ok, err)
		}
	}

	if found, err := c.SearchShows(ctx, " THE CROWN"); err != nil || len(found) != 1 {
		t.Errorf("searched %+v, %v", found, err)
	}

	if n := f.asked("/search/tv"); n != 1 {
		t.Errorf("searched %d times, want once", n)
	}

	if kept, err := c.Show(ctx, 65494); err != nil || kept.FirstAirDate.Year != 2016 || kept.OriginalName != "The Crown" {
		t.Errorf("kept %+v, %v", kept, err)
	}

	for range 2 {
		ep, err := c.Episode(ctx, 65494, 1, 2)
		if err != nil || ep.Name != "Hyde Park Corner" || ep.Runtime != 58 || ep.AirDate.Year != 2016 || ep.ShowID != 65494 {
			t.Fatalf("episode %+v, %v", ep, err)
		}
	}

	if _, err := c.Episode(ctx, 65494, 1, 99); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an episode the season does not have: %v", err)
	}

	for _, tc := range []struct {
		get  func() (string, error)
		file string
	}{
		{func() (string, error) { return c.ShowPoster(ctx, 65494, 1) }, "season1.jpg"},
		{func() (string, error) { return c.ShowPoster(ctx, 65494, 1) }, "season1.jpg"},
		{func() (string, error) { return c.Still(ctx, 65494, 1, 2) }, "still.jpg"},
		{func() (string, error) { return c.ShowPoster(ctx, 65494, 0) }, "crown.jpg"},
	} {
		p, err := tc.get()
		if err != nil {
			t.Fatal(err)
		}

		if b, _ := os.ReadFile(p); string(b) != "JPEG" || filepath.Base(p) != tc.file {
			t.Errorf("%s holds %q, want %s", p, b, tc.file)
		}
	}

	if f.asked("/tv/65494/season/1") != 1 || f.asked("img/season1.jpg") != 1 {
		t.Errorf("asked for the season %d times and its poster %d times, want once each",
			f.asked("/tv/65494/season/1"), f.asked("img/season1.jpg"))
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMDB_API_TOKEN", "")

	offline := openTest(t, path)

	if ep, err := offline.Episode(ctx, 65494, 1, 2); err != nil || ep.Name != "Hyde Park Corner" {
		t.Errorf("episode offline: %+v, %v", ep, err)
	}

	if _, err := offline.SearchShows(ctx, "Succession"); err == nil {
		t.Error("a show search never made was answered without a token")
	}

	if _, err := offline.Episode(ctx, 65494, 2, 1); err == nil {
		t.Error("a season never fetched was answered without a token")
	}
}

// A show asked for by id is fetched once; where it or its season lacks
// something, the answer is fs.ErrNotExist, falling back to the show's poster
// where the season has none.
func TestShowByID(t *testing.T) {
	ctx := t.Context()
	f := newFake(map[string]route{
		"/tv/1399":          {body: gameOfThrones},
		"/tv/1399/season/1": {body: thronesSeason1},
		"/search/tv":        {body: results()},
	})
	c := openTest(t, newDB(t), withFake(f)...)

	for range 2 {
		sh, err := c.Show(ctx, 1399)
		if err != nil || sh.Name != "Game of Thrones" || sh.FirstAirDate.Year != 2011 || sh.VoteCount != 20000 || sh.PosterPath != "" {
			t.Fatalf("got %+v, %v", sh, err)
		}
	}

	if n := f.asked("/tv/1399"); n != 1 {
		t.Errorf("fetched the show %d times, want once", n)
	}

	if _, err := c.Show(ctx, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a show The Movie Database does not have: %v", err)
	}

	// Neither season 1 nor the show has a poster.
	if _, err := c.ShowPoster(ctx, 1399, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the poster of a show without one: %v", err)
	}

	if _, err := c.ShowPoster(ctx, 1, 0); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the poster of a show The Movie Database does not have: %v", err)
	}

	if _, err := c.Still(ctx, 1399, 1, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the still of an episode without one: %v", err)
	}

	if _, err := c.Still(ctx, 1399, 1, 9); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the still of an episode the season does not have: %v", err)
	}

	if _, err := c.Episode(ctx, 1399, 9, 1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an episode of a season The Movie Database does not have: %v", err)
	}

	if sh, ok, err := c.FindShow(ctx, "No Such Show"); err != nil || ok {
		t.Errorf("found %+v, %v, %v", sh, ok, err)
	}
}

// An image path that names no file is refused rather than written outside images.
func TestImagePathNamesNoFile(t *testing.T) {
	c := openTest(t, newDB(t), withFake(newFake(nil))...)

	for _, p := range []string{"/", "/..", ""} {
		if _, err := c.image(t.Context(), p); err == nil {
			t.Errorf("image path %q was accepted", p)
		}
	}
}

// Open needs no token, and Path is data.db in ~/.moviedata.
func TestOpen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("TMDB_API_TOKEN", "")

	p, err := Path()
	if err != nil || p != filepath.Join(home, ".moviedata", "data.db") {
		t.Fatalf("path %q, %v", p, err)
	}

	c, err := Open(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.Movie(t.Context(), 603); err == nil || !strings.Contains(err.Error(), "TMDB_API_TOKEN") {
		t.Errorf("fetching without a token: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(p); err != nil {
		t.Errorf("the database was not created: %v", err)
	}

	if _, err := Open(t.Context(), filepath.Join(p, "data.db")); err == nil {
		t.Error("a database under a file was opened")
	}
}
