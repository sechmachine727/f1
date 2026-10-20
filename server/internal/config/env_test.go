package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f1tele/server/internal/contract"
)

func writeConfig(t *testing.T, dir string, mutate func(*contract.Config)) string {
	t.Helper()
	cfg, err := Defaults()
	if err != nil {
		t.Fatal(err)
	}
	mutate(&cfg)
	raw, _ := json.Marshal(cfg)
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvPrecedenceAcrossAllSources(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, func(c *contract.Config) { c.Recording.CapBytes = 11 << 30; c.Recording.ChunkFlushMs = 4321 })
	t.Setenv("F1TELE_DATA_DIR", dir)
	t.Setenv("F1TELE_RECORDING_CAP_BYTES", "22")
	s, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	view := s.View()
	if view.ActiveConfig.Recording.CapBytes != 22 {
		t.Fatalf("env cap = %d", view.ActiveConfig.Recording.CapBytes)
	}
	if view.Sources["recording.cap_bytes"] != "env" {
		t.Fatalf("cap source = %q", view.Sources["recording.cap_bytes"])
	}
	if view.Sources["recording.chunk_bytes"] != "default" {
		t.Fatalf("untouched default source = %q", view.Sources["recording.chunk_bytes"])
	}
	if view.Sources["recording.chunk_flush_ms"] != "file" {
		t.Fatalf("file source = %q", view.Sources["recording.chunk_flush_ms"])
	}
	if _, ok := view.Sources["data_dir"]; !ok || view.Sources["data_dir"] != "env" {
		t.Fatalf("data dir source = %q", view.Sources["data_dir"])
	}

	// CLI wins over the environment.
	s2, err := Load(Overrides{ConfigPath: path, UDPAddress: "127.0.0.1:30999"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("F1TELE_UDP_ADDR", "127.0.0.1:30777")
	s3, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if s3.Config().UDP.Port != 30777 || s3.View().Sources["udp"] != "env" {
		t.Fatalf("env udp = %d source %q", s3.Config().UDP.Port, s3.View().Sources["udp"])
	}
	s4, err := Load(Overrides{ConfigPath: path, UDPAddress: "127.0.0.1:30999"})
	if err != nil {
		t.Fatal(err)
	}
	if s4.Config().UDP.Port != 30999 || s4.View().Sources["udp"] != "cli" {
		t.Fatalf("cli udp = %d source %q", s4.Config().UDP.Port, s4.View().Sources["udp"])
	}
	_ = s2
}

func TestEnvInvalidValuesNameTheVariable(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, func(*contract.Config) {})
	cases := []struct{ name, value string }{
		{"F1TELE_UDP_ADDR", "0.0.0.0"},
		{"F1TELE_RECORDING_CAP_BYTES", "-5"},
		{"F1TELE_RECORDING_ENABLED", "maybe"},
		{"F1TELE_HTTP_ADDR", "127.0.0.1:70000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("F1TELE_DATA_DIR", dir)
			t.Setenv(tc.name, tc.value)
			_, err := Load(Overrides{ConfigPath: path})
			if err == nil {
				t.Fatalf("%s=%s accepted", tc.name, tc.value)
			}
			if !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("error %q does not name %s", err, tc.name)
			}
		})
	}
}

func TestEnvControlledValuesRejectPatch(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, func(*contract.Config) {})
	t.Setenv("F1TELE_DATA_DIR", dir)
	t.Setenv("F1TELE_UDP_ADDR", "0.0.0.0:30777")
	s, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.Config()
	cfg.UDP.Port = 30778
	rev := s.Revision()
	_, err = s.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev)
	if !errors.Is(err, ErrEnvLocked) {
		t.Fatalf("patch to env-controlled udp: %v", err)
	}
	if got := s.Config().UDP.Port; got != 30777 {
		t.Fatalf("effective udp changed to %d", got)
	}
	// Unrelated values remain editable.
	cfg = s.Config()
	cfg.UDP.Port = 30777
	cfg.Recording.CapBytes = 1 << 30
	if _, err := s.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
		t.Fatalf("unrelated patch rejected: %v", err)
	}
}

func TestEnvEndpointIsCreatedAndSecretStaysRedacted(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, func(*contract.Config) {})
	t.Setenv("F1TELE_DATA_DIR", dir)
	t.Setenv("F1TELE_AGENT_ENABLED", "true")
	t.Setenv("F1TELE_AGENT_BASE_URL", "https://example.invalid/v1")
	t.Setenv("F1TELE_AGENT_MODEL", "fixture-model")
	t.Setenv("F1TELE_AGENT_PROTOCOL", "responses")
	t.Setenv("F1TELE_AGENT_API_KEY", "sk-test-secret-value")
	s, err := Load(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.Config()
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil || len(cfg.Agent.Endpoints) == 0 {
		t.Fatalf("agent config %#v", cfg.Agent)
	}
	ep := cfg.Agent.Endpoints[0]
	if ep.BaseURL != "https://example.invalid/v1" || ep.Model != "fixture-model" || ep.Protocol != "responses" {
		t.Fatalf("endpoint %#v", ep)
	}
	if s.Secret(*ep.APIKeyEnv) != "sk-test-secret-value" {
		t.Fatal("environment secret not resolved")
	}
	view := s.View()
	if view.Sources["agent.endpoint"] != "env" {
		t.Fatalf("endpoint source = %q", view.Sources["agent.endpoint"])
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "sk-test-secret-value") {
		t.Fatal("secret leaked into config view")
	}
	if !view.SecretsConfigured[*ep.APIKeyEnv] {
		t.Fatal("secret not reported as configured")
	}
}

func TestAdvertiseHost(t *testing.T) {
	if got := AdvertiseHost("game.example"); got != "game.example" {
		t.Fatalf("explicit advertise host = %q", got)
	}
	got := AdvertiseHost("")
	if got == "" || got == "::" {
		t.Fatalf("discovered advertise host = %q", got)
	}
}

func TestOperatorReferenceDocumentsEveryVariableOnce(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/rework/operator-configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, name := range DocumentedVariables {
		// Count table rows only; prose may mention a variable by name.
		if n := strings.Count(doc, "| `"+name+"` |"); n != 1 {
			t.Errorf("%s appears %d times in the operator reference", name, n)
		}
	}
	for _, spec := range envSpecs {
		seen := false
		for _, name := range DocumentedVariables {
			if name == spec.name {
				seen = true
			}
		}
		if !seen {
			t.Errorf("loader variable %s is missing from DocumentedVariables", spec.name)
		}
	}
	if len(DocumentedVariables) != len(envSpecs)+2 {
		t.Errorf("documented %d variables, loader handles %d plus data dir and config path", len(DocumentedVariables), len(envSpecs))
	}
}
