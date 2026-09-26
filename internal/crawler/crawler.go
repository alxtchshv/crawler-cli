package crawler

import (
	"context"
	"crawler/internal/parse"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
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
	workers  int
}

type result struct {
	job  job
	page parse.Page
	err  error
}

func NewCrawler(f Fetcher, maxDepth int, logger *log.Logger) *Crawler {

	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Crawler{
		fetcher:  f,
		maxDepth: maxDepth,
		logger:   logger,
		workers:  10,
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

	jobs := make(chan job)
	results := make(chan result)
	var wg sync.WaitGroup

	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go c.worker(ctx, jobs, results, &wg)
	}

	pending := 0
loop:
	for len(queue) > 0 || pending > 0 {

		var sendCh chan job
		var next job
		if len(queue) > 0 {
			sendCh = jobs
			next = queue[0]
		}

		select {

		case <-ctx.Done():
			break loop

		case sendCh <- next:
			queue = queue[1:]
			pending++

		case r := <-results:

			pending--
			j := r.job

			if r.err != nil {
				c.logger.Printf("fetch %s: %v", j.url, r.err)
				continue
			}

			node := &Node{
				Resource: j.url.String(),
				Title:    r.page.Title,
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

			for _, link := range r.page.Links {

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
	}

	close(jobs)
	for ; pending > 0; pending-- {
		<-results
	}
	wg.Wait()

	return roots
}

func (c *Crawler) worker(ctx context.Context, jobs <-chan job, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {

		page, err := c.fetcher.Fetch(ctx, j.url)
		results <- result{job: j, page: page, err: err}
	}

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
