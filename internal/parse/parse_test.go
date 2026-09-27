package parse

import (
	"net/url"
	"strings"
	"testing"
)

func TestResolverLink(t *testing.T) {
	base, err := url.Parse("https://site.com/dir/page")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		href   string
		want   string
		wantOK bool
	}{
		{"absolute path", "/a", "https://site.com/a", true},
		{"relative", "a", "https://site.com/dir/a", true},
		{"parent dir", "../a", "https://site.com/a", true},
		{"only query", "?q=1", "https://site.com/dir/page?q=1", true},
		{"only fragment", "#top", "https://site.com/dir/page#top", true},
		{"protocol relative", "//cdn.site.com/x", "https://cdn.site.com/x", true},
		{"other site", "https://other.com/x", "https://other.com/x", true},
		{"spaces around", "  /a  ", "https://site.com/a", true},
		{"mailto", "mailto:a@b.c", "", false},
		{"javascript", "javascript:void(0)", "", false},
		{"tel", "tel:+375291234567", "", false},
		{"empty", "", "", false},
		{"only spaces", "   ", "", false},
		{"broken url", "http://[::1", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolverLink(base, tc.href)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got.String() != tc.want {
				t.Errorf("got %q, want %q", got.String(), tc.want)
			}
		})
	}
}

func TestParseHTML(t *testing.T) {
	base, err := url.Parse("https://site.com/")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantLinks []string
	}{
		{
			name:      "simple",
			html:      `<html><head><title>Hello</title></head><body><a href="/a">A</a><a href="https://site.com/b">B</a></body></html>`,
			wantTitle: "Hello",
			wantLinks: []string{"https://site.com/a", "https://site.com/b"},
		},
		{
			name:      "title with newlines",
			html:      "<title>\n   Hello\n   World  \n</title>",
			wantTitle: "Hello World",
		},
		{
			name:      "no title",
			html:      `<a href="/a">a</a>`,
			wantTitle: "",
			wantLinks: []string{"https://site.com/a"},
		},
		{
			name:      "empty title then text",
			html:      `<title></title><body>Text</body>`,
			wantTitle: "",
		},
		{
			name:      "two titles",
			html:      `<title>First</title><title>Second</title>`,
			wantTitle: "First",
		},
		{
			name:      "entities",
			html:      `<title>Tom &amp; Jerry</title>`,
			wantTitle: "Tom & Jerry",
		},
		{
			name:      "a without href",
			html:      `<a name="x">x</a><a href="/y">y</a>`,
			wantLinks: []string{"https://site.com/y"},
		},
		{
			name:      "uppercase tags",
			html:      `<A HREF="/x">x</A>`,
			wantLinks: []string{"https://site.com/x"},
		},
		{
			name: "junk links",
			html: `<a href="mailto:a@b.c">m</a><a href="">e</a><a href="javascript:void(0)">j</a>`,
		},
		{
			name:      "order",
			html:      `<a href="/c"></a><a href="/a"></a><a href="/b"></a>`,
			wantLinks: []string{"https://site.com/c", "https://site.com/a", "https://site.com/b"},
		},
		{
			name: "link inside script",
			html: `<script>var s = '<a href="/x">';</script>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := ParseHTML(strings.NewReader(tc.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if page.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", page.Title, tc.wantTitle)
			}

			if len(page.Links) != len(tc.wantLinks) {
				t.Fatalf("len(Links) = %d, want %d (got %v)", len(page.Links), len(tc.wantLinks), page.Links)
			}
			for i, want := range tc.wantLinks {
				if got := page.Links[i].String(); got != want {
					t.Errorf("Links[%d] = %q, want %q", i, got, want)
				}
			}
		})
	}
}
