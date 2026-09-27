package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"crawler/internal/parse"
)

type fakeFetcher struct {
	mu    sync.Mutex
	pages map[string]parse.Page
	errs  map[string]error
	calls map[string]int
}

func newFake() *fakeFetcher {
	return &fakeFetcher{
		pages: map[string]parse.Page{},
		errs:  map[string]error{},
		calls: map[string]int{},
	}
}

func (f *fakeFetcher) Fetch(ctx context.Context, u *url.URL) (parse.Page, error) {
	if err := ctx.Err(); err != nil {
		return parse.Page{}, err
	}

	key := u.String()
	f.mu.Lock()
	f.calls[key]++
	f.mu.Unlock()

	if err, ok := f.errs[key]; ok {
		return parse.Page{}, err
	}
	if p, ok := f.pages[key]; ok {
		return p, nil
	}
	return parse.Page{}, errors.New("not found")
}

func (f *fakeFetcher) add(t *testing.T, rawURL, title string, links ...string) {
	t.Helper()
	p := parse.Page{Title: title}
	for _, l := range links {
		p.Links = append(p.Links, mustURL(t, l))
	}
	f.pages[rawURL] = p
}

func (f *fakeFetcher) fail(rawURL string) {
	f.errs[rawURL] = errors.New("boom")
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func mustURLs(t *testing.T, ss ...string) []*url.URL {
	t.Helper()
	res := make([]*url.URL, 0, len(ss))
	for _, s := range ss {
		res = append(res, mustURL(t, s))
	}
	return res
}

func toJSON(t *testing.T, roots []*Node) string {
	t.Helper()
	b, err := json.Marshal(roots)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVisitedLink(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{"host case", "https://Example.com/a", "https://example.com/a", true},
		{"fragment", "https://site.com/page#top", "https://site.com/page", true},
		{"empty path", "https://site.com", "https://site.com/", true},
		{"path case", "https://site.com/About", "https://site.com/about", false},
		{"port", "https://site.com:8080/a", "https://site.com/a", false},
		{"www", "https://www.site.com/a", "https://site.com/a", false},
		{"scheme", "http://site.com/a", "https://site.com/a", false},
		{"query", "https://site.com/a?x=1", "https://site.com/a?x=2", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ka := visitedLink(mustURL(t, tc.a))
			kb := visitedLink(mustURL(t, tc.b))
			if (ka == kb) != tc.same {
				t.Errorf("keys %q and %q: same = %v, want %v", ka, kb, ka == kb, tc.same)
			}
		})
	}
}

func TestVisitedLinkDoesNotModify(t *testing.T) {
	u := mustURL(t, "https://Example.com/a#top")
	visitedLink(u)

	if got := u.String(); got != "https://Example.com/a#top" {
		t.Errorf("url changed to %q", got)
	}
}

func TestSiteLink(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://site.com/a", "site.com"},
		{"https://WWW.Site.com/x", "site.com"},
		{"https://site.com:8080/", "site.com"},
		{"http://www.site.com", "site.com"},
		{"https://cdn.site.com/", "cdn.site.com"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := siteLink(mustURL(t, tc.in)); got != tc.want {
				t.Errorf("siteLink(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCrawl(t *testing.T) {
	tests := []struct {
		name   string
		starts []string
		depth  int
		setup  func(t *testing.T, f *fakeFetcher)
		want   string
		once   []string
		never  []string
	}{
		{
			name:   "cycle",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b")
				f.add(t, "https://a.com/b", "B", "https://a.com/")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/b","title":"B","links":[]}]}]`,
			once: []string{"https://a.com/", "https://a.com/b"},
		},
		{
			name:   "depth limit",
			starts: []string{"https://a.com/"},
			depth:  2,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b")
				f.add(t, "https://a.com/b", "B", "https://a.com/c")
				f.add(t, "https://a.com/c", "C", "https://a.com/d")
				f.add(t, "https://a.com/d", "D")
			},
			want:  `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/b","title":"B","links":[{"resource":"https://a.com/c","title":"C","links":[]}]}]}]`,
			never: []string{"https://a.com/d"},
		},
		{
			name:   "depth zero",
			starts: []string{"https://a.com/"},
			depth:  0,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b")
				f.add(t, "https://a.com/b", "B")
			},
			want:  `[{"resource":"https://a.com/","title":"A","links":[]}]`,
			never: []string{"https://a.com/b"},
		},
		{
			name:   "foreign domain",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://b.com/x", "https://a.com/c")
				f.add(t, "https://b.com/x", "X")
				f.add(t, "https://a.com/c", "C")
			},
			want:  `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/c","title":"C","links":[]}]}]`,
			never: []string{"https://b.com/x"},
		},
		{
			name:   "www and case is same site",
			starts: []string{"https://site.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://site.com/", "S", "https://WWW.Site.com/x")
				f.add(t, "https://WWW.Site.com/x", "X")
			},
			want: `[{"resource":"https://site.com/","title":"S","links":[{"resource":"https://WWW.Site.com/x","title":"X","links":[]}]}]`,
			once: []string{"https://WWW.Site.com/x"},
		},
		{
			name:   "host case is same page",
			starts: []string{"https://example.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://example.com/", "E", "https://Example.com/b", "https://example.com/b")
				f.add(t, "https://Example.com/b", "B")
			},
			want:  `[{"resource":"https://example.com/","title":"E","links":[{"resource":"https://Example.com/b","title":"B","links":[]}]}]`,
			once:  []string{"https://Example.com/b"},
			never: []string{"https://example.com/b"},
		},
		{
			name:   "fragments",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b", "https://a.com/b#x", "https://a.com/#top")
				f.add(t, "https://a.com/b", "B")
			},
			want:  `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/b","title":"B","links":[]}]}]`,
			once:  []string{"https://a.com/", "https://a.com/b"},
			never: []string{"https://a.com/b#x", "https://a.com/#top"},
		},
		{
			name:   "empty path equals slash",
			starts: []string{"https://a.com"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com", "A", "https://a.com/")
			},
			want:  `[{"resource":"https://a.com","title":"A","links":[]}]`,
			never: []string{"https://a.com/"},
		},
		{
			name:   "shared child goes to first parent",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b", "https://a.com/c")
				f.add(t, "https://a.com/b", "B", "https://a.com/d")
				f.add(t, "https://a.com/c", "C", "https://a.com/d")
				f.add(t, "https://a.com/d", "D")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/b","title":"B","links":[{"resource":"https://a.com/d","title":"D","links":[]}]},{"resource":"https://a.com/c","title":"C","links":[]}]}]`,
			once: []string{"https://a.com/d"},
		},
		{
			name:   "failed page is skipped",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/b", "https://a.com/c")
				f.fail("https://a.com/b")
				f.add(t, "https://a.com/c", "C")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/c","title":"C","links":[]}]}]`,
			once: []string{"https://a.com/b"},
		},
		{
			name:   "failed root does not stop others",
			starts: []string{"https://a.com/", "https://x.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.fail("https://a.com/")
				f.add(t, "https://x.com/", "X")
			},
			want: `[{"resource":"https://x.com/","title":"X","links":[]}]`,
		},
		{
			name:   "two sites stay separate",
			starts: []string{"https://a.com/", "https://b.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://b.com/", "https://a.com/a2")
				f.add(t, "https://b.com/", "B", "https://a.com/", "https://b.com/b2")
				f.add(t, "https://a.com/a2", "A2")
				f.add(t, "https://b.com/b2", "B2")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/a2","title":"A2","links":[]}]},{"resource":"https://b.com/","title":"B","links":[{"resource":"https://b.com/b2","title":"B2","links":[]}]}]`,
			once: []string{"https://a.com/", "https://b.com/"},
		},
		{
			name:   "duplicate start url",
			starts: []string{"https://a.com/", "https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[]}]`,
			once: []string{"https://a.com/"},
		},
		{
			name:   "children keep page order",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.add(t, "https://a.com/", "A", "https://a.com/c", "https://a.com/x", "https://a.com/b")
				f.add(t, "https://a.com/c", "C")
				f.add(t, "https://a.com/x", "X")
				f.add(t, "https://a.com/b", "B")
			},
			want: `[{"resource":"https://a.com/","title":"A","links":[{"resource":"https://a.com/c","title":"C","links":[]},{"resource":"https://a.com/x","title":"X","links":[]},{"resource":"https://a.com/b","title":"B","links":[]}]}]`,
		},
		{
			name:   "all failed gives empty array, not null",
			starts: []string{"https://a.com/"},
			depth:  5,
			setup: func(t *testing.T, f *fakeFetcher) {
				f.fail("https://a.com/")
			},
			want: `[]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			tc.setup(t, f)

			c := NewCrawler(f, tc.depth, 1, nil)
			roots := c.Crawl(context.Background(), mustURLs(t, tc.starts...))

			if got := toJSON(t, roots); got != tc.want {
				t.Errorf("tree:\ngot  %s\nwant %s", got, tc.want)
			}

			for _, u := range tc.once {
				if f.calls[u] != 1 {
					t.Errorf("calls[%s] = %d, want 1", u, f.calls[u])
				}
			}
			for _, u := range tc.never {
				if f.calls[u] != 0 {
					t.Errorf("calls[%s] = %d, want 0", u, f.calls[u])
				}
			}
		})
	}
}

func TestCrawlCanceledContext(t *testing.T) {
	f := newFake()
	f.add(t, "https://a.com/", "A", "https://a.com/b")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewCrawler(f, 5, 10, nil)
	roots := c.Crawl(ctx, mustURLs(t, "https://a.com/"))

	if got := toJSON(t, roots); got != `[]` {
		t.Errorf("tree = %s, want []", got)
	}
}

func TestCrawlLog(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	f := newFake()
	f.add(t, "https://a.com/", "A", "https://a.com/b")
	f.fail("https://a.com/b")

	c := NewCrawler(f, 5, 1, logger)
	c.Crawl(context.Background(), mustURLs(t, "https://a.com/"))

	out := buf.String()
	if !strings.Contains(out, "OK https://a.com/") {
		t.Errorf("log has no OK line for root:\n%s", out)
	}
	if !strings.Contains(out, "FAIL https://a.com/b") {
		t.Errorf("log has no FAIL line for /b:\n%s", out)
	}
}

type slowFetcher struct {
	mu      sync.Mutex
	current int
	max     int
	delay   time.Duration
	pages   map[string]parse.Page
}

func (f *slowFetcher) Fetch(ctx context.Context, u *url.URL) (parse.Page, error) {
	f.mu.Lock()
	f.current++
	if f.current > f.max {
		f.max = f.current
	}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.current--
		f.mu.Unlock()
	}()

	select {
	case <-time.After(f.delay):
		return f.pages[u.String()], nil
	case <-ctx.Done():
		return parse.Page{}, ctx.Err()
	}
}

func newWideSite(t *testing.T, n int, delay time.Duration) *slowFetcher {
	t.Helper()
	root := parse.Page{Title: "Root"}
	for i := 0; i < n; i++ {
		root.Links = append(root.Links, mustURL(t, fmt.Sprintf("https://a.com/p%d", i)))
	}
	return &slowFetcher{
		delay: delay,
		pages: map[string]parse.Page{"https://a.com/": root},
	}
}

func TestCrawlConcurrencyLimit(t *testing.T) {
	for _, workers := range []int{3, 10} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			f := newWideSite(t, 50, 20*time.Millisecond)

			c := NewCrawler(f, 1, workers, nil)
			c.Crawl(context.Background(), mustURLs(t, "https://a.com/"))

			if f.max > workers {
				t.Errorf("max concurrent = %d, want <= %d", f.max, workers)
			}
			if f.max < 2 {
				t.Errorf("max concurrent = %d, requests did not run in parallel", f.max)
			}
		})
	}
}

func TestCrawlTimeout(t *testing.T) {
	f := newWideSite(t, 100, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := NewCrawler(f, 1, 2, nil)

	start := time.Now()
	roots := c.Crawl(ctx, mustURLs(t, "https://a.com/"))
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("crawl took %s, want it to stop near the 200ms timeout", elapsed)
	}
	if len(roots) != 1 {
		t.Fatalf("len(roots) = %d, want 1 (root should be fetched before timeout)", len(roots))
	}
	if n := len(roots[0].Links); n >= 100 {
		t.Errorf("root has %d children, want fewer than 100 (crawl should be interrupted)", n)
	}
}
