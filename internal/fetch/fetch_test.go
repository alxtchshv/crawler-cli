package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><title>OK Page</title></head><body><a href="/a">a</a></body></html>`)
	})

	mux.HandleFunc("/upper", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "TEXT/HTML; Charset=UTF-8")
		fmt.Fprint(w, `<title>Upper</title>`)
	})

	mux.HandleFunc("/cp1251", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=windows-1251")
		body, _ := charmap.Windows1251.NewEncoder().String("<title>Привет</title>")
		fmt.Fprint(w, body)
	})

	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusMovedPermanently)
	})

	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		fmt.Fprint(w, "not really a png")
	})

	mux.HandleFunc("/notype", func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = nil
		fmt.Fprint(w, `<title>No type</title>`)
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})

	mux.HandleFunc("/exact", func(w http.ResponseWriter, r *http.Request) {
		writeChunked(w, maxBodySize)
	})

	mux.HandleFunc("/huge", func(w http.ResponseWriter, r *http.Request) {
		writeChunked(w, maxBodySize+1)
	})

	mux.HandleFunc("/huge-length", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", fmt.Sprint(maxBodySize+1))
		w.Write(bytes.Repeat([]byte("a"), maxBodySize+1))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func writeChunked(w http.ResponseWriter, size int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	head := []byte("<title>Big</title>")
	w.Write(head)
	w.(http.Flusher).Flush()
	w.Write(bytes.Repeat([]byte("a"), size-len(head)))
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestFetch(t *testing.T) {
	srv := newTestServer(t)
	f := NewFetcher(time.Second)

	tests := []struct {
		name      string
		path      string
		wantErr   error
		wantTitle string
	}{
		{"ok", "/ok", nil, "OK Page"},
		{"uppercase content type", "/upper", nil, "Upper"},
		{"windows-1251", "/cp1251", nil, "Привет"},
		{"redirect", "/redirect", ErrRedirect, ""},
		{"not found", "/missing", ErrStatus, ""},
		{"image", "/image", ErrNotHTML, ""},
		{"no content type", "/notype", ErrNotHTML, ""},
		{"body exactly at limit", "/exact", nil, "Big"},
		{"chunked body over limit", "/huge", ErrTooLarge, ""},
		{"content length over limit", "/huge-length", ErrTooLarge, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := f.Fetch(context.Background(), mustURL(t, srv.URL+tc.path))

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", page.Title, tc.wantTitle)
			}
		})
	}
}

func TestFetchLinks(t *testing.T) {
	srv := newTestServer(t)
	f := NewFetcher(time.Second)

	page, err := f.Fetch(context.Background(), mustURL(t, srv.URL+"/ok"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Links) != 1 {
		t.Fatalf("len(Links) = %d, want 1", len(page.Links))
	}
	if got, want := page.Links[0].String(), srv.URL+"/a"; got != want {
		t.Errorf("Links[0] = %q, want %q", got, want)
	}
}

func TestFetchTimeout(t *testing.T) {
	srv := newTestServer(t)
	f := NewFetcher(100 * time.Millisecond)

	_, err := f.Fetch(context.Background(), mustURL(t, srv.URL+"/slow"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestFetchCanceled(t *testing.T) {
	srv := newTestServer(t)
	f := NewFetcher(time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.Fetch(ctx, mustURL(t, srv.URL+"/ok"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
