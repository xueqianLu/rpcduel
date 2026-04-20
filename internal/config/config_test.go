package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpandEnv(t *testing.T) {
	t.Setenv("RPCDUEL_TEST_FOO", "bar")
	cases := []struct {
		in, want string
	}{
		{"${RPCDUEL_TEST_FOO}", "bar"},
		{"prefix-${RPCDUEL_TEST_FOO}-suffix", "prefix-bar-suffix"},
		{"${RPCDUEL_TEST_MISSING:-fallback}", "fallback"},
		{"${RPCDUEL_TEST_MISSING}", ""},
		{"$$literal", "$literal"},
		{"no var here", "no var here"},
	}
	for _, c := range cases {
		got := ExpandEnv(c.in)
		if got != c.want {
			t.Errorf("ExpandEnv(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLoad(t *testing.T) {
	t.Setenv("API_KEY", "deadbeef")
	dir := t.TempDir()
	path := filepath.Join(dir, "rpcduel.yaml")
	yamlText := `version: 1
endpoints:
  - name: a
    url: https://node-a.example/${API_KEY}
    headers:
      X-Auth: ${API_KEY}
  - url: ${RPC_B_URL:-https://default-b.example}
defaults:
  timeout: 15s
  retries: 2
bench:
  concurrency: 50
  duration: 30s
  warmup: 5s
thresholds:
  bench:
    p99_ms: 500
    error_rate: 0.01
reports:
  html: report.html
`
	if err := os.WriteFile(path, []byte(yamlText), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Endpoints) != 2 {
		t.Fatalf("endpoints: %d", len(cfg.Endpoints))
	}
	if cfg.Endpoints[0].URL != "https://node-a.example/deadbeef" {
		t.Errorf("expand: %q", cfg.Endpoints[0].URL)
	}
	if cfg.Endpoints[0].Headers["X-Auth"] != "deadbeef" {
		t.Errorf("header expand: %q", cfg.Endpoints[0].Headers["X-Auth"])
	}
	if cfg.Endpoints[1].URL != "https://default-b.example" {
		t.Errorf("default expand: %q", cfg.Endpoints[1].URL)
	}
	if cfg.Defaults.Timeout != 15*time.Second {
		t.Errorf("timeout: %v", cfg.Defaults.Timeout)
	}
	if cfg.Bench.Concurrency != 50 {
		t.Errorf("bench concurrency: %d", cfg.Bench.Concurrency)
	}
	if cfg.Thresholds.Bench.P99Ms != 500 {
		t.Errorf("threshold p99: %v", cfg.Thresholds.Bench.P99Ms)
	}
	if cfg.Reports.HTML != "report.html" {
		t.Errorf("reports html: %q", cfg.Reports.HTML)
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rpcduel.yaml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for version 2")
	}
}

func TestHeadersFor(t *testing.T) {
	c := &Config{
		Defaults: Defaults{Headers: map[string]string{"User": "rpcduel", "X-A": "default"}},
		Endpoints: []Endpoint{
			{URL: "https://a.example", Headers: map[string]string{"X-A": "endpoint", "X-B": "ep"}},
		},
	}
	h := c.HeadersFor("https://a.example")
	if h["User"] != "rpcduel" || h["X-A"] != "endpoint" || h["X-B"] != "ep" {
		t.Errorf("merged headers wrong: %+v", h)
	}
	if h := c.HeadersFor("https://other.example"); h["User"] != "rpcduel" {
		t.Errorf("default-only headers missing: %+v", h)
	}
}
