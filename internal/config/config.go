package config

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Urls           []*url.URL
	Depth          int
	Timeout        time.Duration
	RequestTimeout time.Duration
	OutputName     string
	LogName        string
}

type rawArgs struct {
	Urls           string
	Depth          int
	Timeout        time.Duration
	RequestTimeout time.Duration
	OutputName     string
	LogName        string
}

func LoadConfig(args []string) (Config, error) {

	rawConfig, err := parseArgs(args)
	if err != nil {
		return Config{}, err
	}

	config, err := newConfig(rawConfig)
	if err != nil {
		return Config{}, err
	}

	return config, nil

}

func parseArgs(args []string) (rawArgs, error) {

	var rawConfig rawArgs

	fs := flag.NewFlagSet("crawler", flag.ContinueOnError)
	fs.StringVar(&rawConfig.Urls, "urls", "", "список урлов через запятую")
	fs.IntVar(&rawConfig.Depth, "depth", 1, "глубина обхода")
	fs.DurationVar(&rawConfig.Timeout, "timeout", 2*time.Minute, "время на работу кравлера")
	fs.DurationVar(&rawConfig.RequestTimeout, "request-timeout", 5*time.Second, "максимальное время на 1 запрос")
	fs.StringVar(&rawConfig.OutputName, "output", "output.json", "имя файла для результата")
	fs.StringVar(&rawConfig.LogName, "log", "crawler.log", "имя файла для логов")

	if err := fs.Parse(args); err != nil {
		return rawArgs{}, err
	}

	if fs.NArg() > 0 {
		return rawArgs{}, fmt.Errorf("неожидаемый аргумент %v", fs.Args())
	}

	return rawConfig, nil
}

func newConfig(raw rawArgs) (Config, error) {

	tmpUrls := strings.Split(raw.Urls, ",")
	urls := make([]*url.URL, 0, len(tmpUrls))
	for _, strUrl := range tmpUrls {
		strUrl = strings.TrimSpace(strUrl)

		if strUrl == "" {
			continue
		}

		parsedUrl, err := url.Parse(strUrl)
		if err != nil {
			return Config{}, fmt.Errorf("некорректный url %q: %w", strUrl, err)
		}

		scheme := parsedUrl.Scheme
		if scheme != "http" && scheme != "https" {
			return Config{}, fmt.Errorf("недопустимый протокол %q в url %q", scheme, strUrl)
		}

		if parsedUrl.Host == "" {
			return Config{}, fmt.Errorf("отсутствует хост в url %q", strUrl)
		}

		urls = append(urls, parsedUrl)
	}

	if len(urls) == 0 {
		return Config{}, fmt.Errorf("указано 0 urls, необходим хотя бы 1")
	}

	if raw.Depth < 0 {
		return Config{}, fmt.Errorf("глубина обхода должна быть >= 0")
	}

	if raw.Timeout <= 0 {
		return Config{}, fmt.Errorf("время работы кравлера должно быть больше 0")
	}

	if raw.RequestTimeout <= 0 {
		return Config{}, fmt.Errorf("время на запрос должно быть больше 0")
	}

	raw.OutputName = strings.TrimSpace(raw.OutputName)
	if raw.OutputName == "" {
		return Config{}, fmt.Errorf("не допустимо пустое имя файла(результата)")
	}

	raw.LogName = strings.TrimSpace(raw.LogName)
	if raw.LogName == "" {
		return Config{}, fmt.Errorf("не допустимо пустое имя файла(логов)")
	}

	return Config{
		Urls:           urls,
		Depth:          raw.Depth,
		Timeout:        raw.Timeout,
		RequestTimeout: raw.RequestTimeout,
		OutputName:     raw.OutputName,
		LogName:        raw.LogName,
	}, nil

}
