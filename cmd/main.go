package main

import (
	"context"
	"crawler/internal/config"
	"crawler/internal/crawler"
	"crawler/internal/fetch"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	cfg, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "оштбка закгрузки конфига: %v\n", err)
		return
	}

	sigNotifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctxWTimeout, cancelTimeout := context.WithTimeout(sigNotifyCtx, cfg.Timeout)
	defer cancelTimeout()

	done := make(chan struct{})
	go func() {

		select {
		case <-sigNotifyCtx.Done():
			fmt.Fprintln(os.Stderr, "работа завершается. для принудительного выхода - ctrl + c")
			stop()

		case <-done:
		}
	}()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	f := fetch.NewFetcher(cfg.RequestTimeout)
	c := crawler.NewCrawler(f, cfg.Depth, logger)

	roots := c.Crawl(ctxWTimeout, cfg.Urls)
	close(done)

	switch {
	case errors.Is(ctxWTimeout.Err(), context.DeadlineExceeded):
		fmt.Fprintln(os.Stderr, "истек общий таймаут")
	case errors.Is(ctxWTimeout.Err(), context.Canceled):
		fmt.Fprintln(os.Stderr, "работа прервана")
	}

	data, err := json.MarshalIndent(roots, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "json:", err)
		return
	}

	fmt.Println(string(data))
}
