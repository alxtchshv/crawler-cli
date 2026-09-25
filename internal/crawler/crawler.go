package crawler

import (
	"context"
	"crawler/internal/parse"
	"io"
	"log"
	"net/url"
	"strings"
)

type Fetcher interface {
	Fetch(ctx context.Context, u *url.URL) (parse.Page, error)
}

type Node struct {
	Resource string  `json:"resource"`
	Title    string  `json:"title"`
	Links    []*Node `json:"links"`
}

type job struct {
	url    *url.URL
	depth  int
	parent *Node
	site   string
}

type Crawler struct {
	fetcher  Fetcher
	maxDepth int
	logger   *log.Logger
}

func NewCrawler(f Fetcher, maxDepth int, logger *log.Logger) *Crawler {

	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Crawler{
		fetcher:  f,
		maxDepth: maxDepth,
		logger:   logger,
	}

}

func (c *Crawler) Crawl(ctx context.Context, starts []*url.URL) []*Node {

	visited := make(map[string]struct{})
	queue := make([]job, 0)
	roots := make([]*Node, 0)

	for _, u := range starts {

		key := visitedLink(u)
		if _, ok := visited[key]; ok {
			continue
		}

		visited[key] = struct{}{}
		queue = append(queue, job{url: u, depth: 0, parent: nil, site: siteLink(u)})
	}

	for len(queue) > 0 {

		j := queue[0]
		queue = queue[1:]

		page, err := c.fetcher.Fetch(ctx, j.url)
		if err != nil {
			c.logger.Printf("fetch %s: %v", j.url, err)
			continue
		}

		node := &Node{
			Resource: j.url.String(),
			Title:    page.Title,
			Links:    make([]*Node, 0),
		}

		if j.parent == nil {
			roots = append(roots, node)
		} else {
			j.parent.Links = append(j.parent.Links, node)
		}

		if j.depth >= c.maxDepth {
			continue
		}

		for _, link := range page.Links {

			if siteLink(link) != j.site {
				continue
			}

			key := visitedLink(link)
			if _, ok := visited[key]; ok {
				continue
			}

			visited[key] = struct{}{}
			queue = append(queue, job{url: link, depth: j.depth + 1, parent: node, site: j.site})
		}
	}

	return roots
}

func visitedLink(u *url.URL) string {

	tmpURL := *u
	tmpURL.Fragment = ""
	tmpURL.Host = strings.ToLower(tmpURL.Host)

	return tmpURL.String()
}

func siteLink(u *url.URL) string {

	site := u.Hostname()
	site = strings.ToLower(site)
	site = strings.TrimPrefix(site, "www.")

	return site
}
