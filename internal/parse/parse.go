package parse

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type Page struct {
	Title string
	Links []*url.URL
}

func ParseHTML(reader io.Reader, sourceLink *url.URL) (Page, error) {

	var page Page
	inTitle := false
	tokenizer := html.NewTokenizer(reader)

	for {

		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:

			err := tokenizer.Err()
			if errors.Is(err, io.EOF) {
				return page, nil
			}

			return Page{}, fmt.Errorf("ошибка парсинга html: %w", err)

		case html.StartTagToken:

			token := tokenizer.Token()
			if token.Data == "title" {
				inTitle = true
			}

			if token.Data == "a" {

				for _, attr := range token.Attr {

					if attr.Key != "href" {
						continue
					}

					link, ok := resolverLink(sourceLink, attr.Val)
					if ok {
						page.Links = append(page.Links, link)
					}

					break
				}

			}

		case html.TextToken:

			if inTitle && page.Title == "" {
				page.Title = strings.Join(strings.Fields(tokenizer.Token().Data), " ")
			}

		case html.EndTagToken:

			if tokenizer.Token().Data == "title" {
				inTitle = false
			}

		}

	}

}

func resolverLink(sourceLink *url.URL, href string) (*url.URL, bool) {

	href = strings.TrimSpace(href)
	if href == "" {
		return nil, false
	}

	tmpLink, err := url.Parse(href)
	if err != nil {
		return nil, false
	}

	link := sourceLink.ResolveReference(tmpLink)

	scheme := link.Scheme
	if scheme != "http" && scheme != "https" {
		return nil, false
	}

	return link, true
}
