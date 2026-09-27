package config

import (
	"errors"
	"flag"
	"testing"
	"time"
)

func TestLoadConfigAllFlags(t *testing.T) {
	args := []string{
		"--urls", "https://a.com,http://b.com/x",
		"--depth", "3",
		"--workers", "5",
		"--timeout", "30s",
		"--request-timeout", "2s",
		"--output", "res.json",
		"--log", "my.log",
	}

	cfg, err := LoadConfig(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Urls) != 2 {
		t.Fatalf("len(Urls) = %d, want 2", len(cfg.Urls))
	}
	if got := cfg.Urls[0].String(); got != "https://a.com" {
		t.Errorf("Urls[0] = %q, want %q", got, "https://a.com")
	}
	if got := cfg.Urls[1].String(); got != "http://b.com/x" {
		t.Errorf("Urls[1] = %q, want %q", got, "http://b.com/x")
	}
	if cfg.Depth != 3 {
		t.Errorf("Depth = %d, want 3", cfg.Depth)
	}
	if cfg.Workers != 5 {
		t.Errorf("Workers = %d, want 5", cfg.Workers)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %s, want 30s", cfg.Timeout)
	}
	if cfg.RequestTimeout != 2*time.Second {
		t.Errorf("RequestTimeout = %s, want 2s", cfg.RequestTimeout)
	}
	if cfg.OutputName != "res.json" {
		t.Errorf("OutputName = %q, want %q", cfg.OutputName, "res.json")
	}
	if cfg.LogName != "my.log" {
		t.Errorf("LogName = %q, want %q", cfg.LogName, "my.log")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig([]string{"--urls", "https://a.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Depth != 1 {
		t.Errorf("Depth = %d, want 1", cfg.Depth)
	}
	if cfg.Workers != 10 {
		t.Errorf("Workers = %d, want 10", cfg.Workers)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %s, want 2m", cfg.Timeout)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Errorf("RequestTimeout = %s, want 5s", cfg.RequestTimeout)
	}
	if cfg.OutputName != "output.json" {
		t.Errorf("OutputName = %q, want %q", cfg.OutputName, "output.json")
	}
	if cfg.LogName != "crawler.log" {
		t.Errorf("LogName = %q, want %q", cfg.LogName, "crawler.log")
	}
}

func TestLoadConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantURLs  []string
		wantDepth int
	}{
		{
			name:      "spaces and empty parts",
			args:      []string{"--urls", "https://a.com, ,https://b.com,"},
			wantURLs:  []string{"https://a.com", "https://b.com"},
			wantDepth: 1,
		},
		{
			name:      "depth zero",
			args:      []string{"--urls", "https://a.com", "--depth", "0"},
			wantURLs:  []string{"https://a.com"},
			wantDepth: 0,
		},
		{
			name:      "equals form",
			args:      []string{"--urls=https://a.com", "--depth=2"},
			wantURLs:  []string{"https://a.com"},
			wantDepth: 2,
		},
		{
			name:      "uppercase scheme",
			args:      []string{"--urls", "HTTPS://Example.com"},
			wantURLs:  []string{"https://Example.com"},
			wantDepth: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(cfg.Urls) != len(tc.wantURLs) {
				t.Fatalf("len(Urls) = %d, want %d", len(cfg.Urls), len(tc.wantURLs))
			}
			for i, want := range tc.wantURLs {
				if got := cfg.Urls[i].String(); got != want {
					t.Errorf("Urls[%d] = %q, want %q", i, got, want)
				}
			}
			if cfg.Depth != tc.wantDepth {
				t.Errorf("Depth = %d, want %d", cfg.Depth, tc.wantDepth)
			}
		})
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no urls", []string{}},
		{"only comma", []string{"--urls", ","}},
		{"ftp", []string{"--urls", "ftp://a.com"}},
		{"no scheme", []string{"--urls", "example.com"}},
		{"no host", []string{"--urls", "https://"}},
		{"forgot slashes", []string{"--urls", "http:example.com"}},
		{"one bad among good", []string{"--urls", "https://a.com,ftp://b.com"}},
		{"negative depth", []string{"--urls", "https://a.com", "--depth", "-1"}},
		{"depth not number", []string{"--urls", "https://a.com", "--depth", "abc"}},
		{"zero workers", []string{"--urls", "https://a.com", "--workers", "0"}},
		{"too many workers", []string{"--urls", "https://a.com", "--workers", "11"}},
		{"zero timeout", []string{"--urls", "https://a.com", "--timeout", "0s"}},
		{"timeout without unit", []string{"--urls", "https://a.com", "--timeout", "10"}},
		{"negative request timeout", []string{"--urls", "https://a.com", "--request-timeout", "-1s"}},
		{"empty output", []string{"--urls", "https://a.com", "--output", "  "}},
		{"empty log", []string{"--urls", "https://a.com", "--log", ""}},
		{"extra argument", []string{"--urls", "https://a.com", "oops"}},
		{"unknown flag", []string{"--urls", "https://a.com", "--deep", "3"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadConfig(tc.args); err == nil {
				t.Errorf("expected error for args %q, got nil", tc.args)
			}
		})
	}
}

func TestLoadConfigHelp(t *testing.T) {
	_, err := LoadConfig([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("err = %v, want flag.ErrHelp", err)
	}
}
