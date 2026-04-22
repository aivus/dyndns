package config

import (
	"os"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

const validYAML = `
update_token: "secret"
cloudflare:
  api_token: "cf_token"
records:
  - zone_id: "zone1"
    name: "home.example.com"
    suffix: "::1"
`

func TestLoad_Valid(t *testing.T) {
	path := writeTemp(t, validYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UpdateToken != "secret" {
		t.Errorf("UpdateToken = %q, want %q", cfg.UpdateToken, "secret")
	}
	if cfg.Cloudflare.APIToken != "cf_token" {
		t.Errorf("APIToken = %q, want %q", cfg.Cloudflare.APIToken, "cf_token")
	}
	if len(cfg.Records) != 1 {
		t.Errorf("len(Records) = %d, want 1", len(cfg.Records))
	}
	if cfg.Records[0].Suffix != "::1" {
		t.Errorf("Suffix = %q, want %q", cfg.Records[0].Suffix, "::1")
	}
}

func TestLoad_EnvOverridesAPIToken(t *testing.T) {
	path := writeTemp(t, validYAML)
	t.Setenv("CLOUDFLARE_API_TOKEN", "from_env")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cloudflare.APIToken != "from_env" {
		t.Errorf("APIToken = %q, want %q", cfg.Cloudflare.APIToken, "from_env")
	}
}

func TestLoad_MissingUpdateToken(t *testing.T) {
	path := writeTemp(t, `
cloudflare:
  api_token: "cf_token"
records:
  - zone_id: "z"
    name: "h.example.com"
    suffix: "::1"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_MissingAPIToken(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	path := writeTemp(t, `
update_token: "secret"
records:
  - zone_id: "z"
    name: "h.example.com"
    suffix: "::1"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_EmptyRecords(t *testing.T) {
	path := writeTemp(t, `
update_token: "secret"
cloudflare:
  api_token: "cf_token"
records: []
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
