package main

import (
	"context"
	"crawler/internal/config"
	"crawler/internal/crawler"
	"crawler/internal/fetch"
	"crawler/internal/output"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {

	cfg, err := config.LoadConfig(os.Args[1:])
	if err != nil {

		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}

		fmt.Fprintf(os.Stderr, "оштбка закгрузки конфига: %v\n", err)
		os.Exit(2)
	}

	logFile, err := os.Create(cfg.LogName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка создания файла логов: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	if _, err := os.Stat(filepath.Dir(cfg.OutputName)); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка доступа к директории вывода: %v\n", err)
		os.Exit(1)
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

	logger.Printf("начало обхода:\nurls=%v,\ndepth=%d,\n, workers=%d,\ntimeout=%s,\nrequest-timeout=%s",
		cfg.Urls, cfg.Depth, cfg.Workers, cfg.Timeout, cfg.RequestTimeout)

	f := fetch.NewFetcher(cfg.RequestTimeout)
	c := crawler.NewCrawler(f, cfg.Depth, cfg.Workers, logger)

	roots := c.Crawl(ctxWTimeout, cfg.Urls)
	close(done)

	switch {
	case errors.Is(ctxWTimeout.Err(), context.DeadlineExceeded):
		fmt.Fprintln(os.Stderr, "истек общий таймаут")
		logger.Println("истек общий таймаут")
	case errors.Is(ctxWTimeout.Err(), context.Canceled):
		fmt.Fprintln(os.Stderr, "работа прервана")
		logger.Println("работа прервана")
	default:
		fmt.Fprintln(os.Stderr, "обход завершен успешно")
		logger.Println("обход завершен успешно")
	}

	if err := output.WriteJSON(cfg.OutputName, roots); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка записи в файл: %v\n", err)
		logger.Printf("ошибка записи в файл: %v", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "результат: %s, лог: %s\n", cfg.OutputName, cfg.LogName)
}
