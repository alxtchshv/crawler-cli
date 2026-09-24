package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"crawler/internal/parse"
)

const maxBodySize = 5 << 20

var (
	ErrRedirect = errors.New("redirect")
	ErrStatus   = errors.New("bad status")
	ErrNotHTML  = errors.New("not html")
	ErrTooLarge = errors.New("page too large")
)

type Fetcher struct {
	client         *http.Client
	requestTimeout time.Duration
}

func NewFetcher(requestTimeout time.Duration) *Fetcher {

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Fetcher{
		client:         client,
		requestTimeout: requestTimeout,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, u *url.URL) (parse.Page, error) {

	ctx, cancel := context.WithTimeout(ctx, f.requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return parse.Page{}, fmt.Errorf("request: %w", err)
	}

	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Crawler/1.0)")

	resp, err := f.client.Do(request)
	if err != nil {
		return parse.Page{}, fmt.Errorf("response: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return parse.Page{}, fmt.Errorf("%w: status code %d", ErrRedirect, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return parse.Page{}, fmt.Errorf("%w: status code %d", ErrStatus, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return parse.Page{}, fmt.Errorf("%w: content type %s", ErrNotHTML, contentType)
	}

	if mediaType != "text/html" {
		return parse.Page{}, fmt.Errorf("%w: content type %s", ErrNotHTML, mediaType)
	}

	if resp.ContentLength > maxBodySize {
		return parse.Page{}, fmt.Errorf("%w: content length %d", ErrTooLarge, resp.ContentLength)
	}

	page, err := parse.ParseHTML(io.LimitReader(resp.Body, maxBodySize), u)
	if err != nil {
		return parse.Page{}, err
	}

	return page, nil
}
