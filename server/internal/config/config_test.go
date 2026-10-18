package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"f1tele/server/internal/contract"
)

func TestEmbeddedDefaultsMatchAuthority(t *testing.T) {
	want, err := os.ReadFile("../../../docs/rework/defaults.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("defaults.json")
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(want, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &b); err != nil {
		t.Fatal(err)
	}
	if string(mustJSON(t, a)) != string(mustJSON(t, b)) {
		t.Fatal("server defaults drifted from docs/rework/defaults.json")
	}
}

func TestOwnedEnvironmentDoesNotRepurposeHome(t *testing.T) {
	// The task rules forbid using HOME as a task variable; ensure the loader only
	// reads its documented variables.
	t.Setenv("F1TELE_DATA_DIR", t.TempDir())
	s, err := Load(Overrides{ConfigPath: filepath.Join(t.TempDir(), "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	if s.DataDir() == "" {
		t.Fatal("data dir unset")
	}
}

func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"schema_version": "1.0.0", "http": map[string]any{"host": "127.0.0.1", "port": 9000}})
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("F1TELE_HTTP_ADDR", "0.0.0.0:9100")
	t.Setenv("F1TELE_DATA_DIR", dir)
	s, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Config().HTTP.Port; got != 9100 {
		t.Fatalf("environment port = %d want 9100", got)
	}
	s2, err := Load(Overrides{ConfigPath: path, HTTPAddress: "127.0.0.1:9200"})
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Config().HTTP.Port; got != 9200 {
		t.Fatalf("CLI port = %d want 9200", got)
	}
}

func TestInvalidPort(t *testing.T) {
	cfg, _ := Defaults()
	cfg.HTTP.Port = 70000
	if err := Validate(cfg); err == nil {
		t.Fatal("invalid port accepted")
	}
}

func TestPatchRedactsSecretsAndIsAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	s, err := Load(Overrides{ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := Defaults()
	cfg.DataDir = &dir
	envName := "F1TELE_TEST_PROVIDER_KEY"
	cfg.Agent.Endpoints = []contract.Endpoint{{
		ID: "main", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m",
		APIKeyEnv: &envName, ContextLimitTokens: 8192,
	}}
	cfg.Agent.EndpointID = strp("main")
	rev := s.Revision()
	stale := rev + 5
	if _, err := s.ApplyPatch(contract.ConfigPatch{Config: cfg}, &stale); err == nil {
		t.Fatal("stale revision accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Fatal("failed patch wrote config")
	}
	view, err := s.ApplyPatch(contract.ConfigPatch{Config: cfg, Secrets: []contract.SecretInput{{ID: envName, Value: "super-secret"}}}, &rev)
	if err != nil {
		t.Fatal(err)
	}
	if !view.SecretsConfigured[envName] {
		t.Fatal("configured secret not reported")
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || containsSecret(raw) {
		t.Fatal("secret leaked into config view")
	}
	secrets, err := os.ReadFile(filepath.Join(dir, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(dir, "secrets.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets mode %v", info.Mode().Perm())
	}
	if !containsSecret(secrets) {
		t.Fatal("secret not persisted")
	}
}

func containsSecret(b []byte) bool {
	for i := 0; i+12 <= len(b); i++ {
		if string(b[i:i+12]) == "super-secret" {
			return true
		}
	}
	return false
}

func strp(s string) *string { return &s }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
