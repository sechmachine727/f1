package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"f1tele/server/internal/contract"
)

// envSpec is one documented environment override. Keys use the dotted paths
// reported by GET /config so the UI can show where a value came from.
type envSpec struct {
	name    string
	key     string
	restart bool
	apply   func(cfg *contract.Config, v string) error
}

var envSpecs = []envSpec{
	{"F1TELE_HTTP_ADDR", "http", true, func(c *contract.Config, v string) error { return setAddr(&c.HTTP, v) }},
	{"F1TELE_UDP_ADDR", "udp", true, func(c *contract.Config, v string) error { return setAddr(&c.UDP, v) }},
	{"F1TELE_UDP_ADVERTISE_HOST", "udp_advertise_host", false, func(*contract.Config, string) error { return nil }},
	{"F1TELE_RECORDING_ENABLED", "recording.enabled", false, func(c *contract.Config, v string) error { b, err := parseBool(v); c.Recording.Enabled = b; return err }},
	{"F1TELE_RECORDING_CAP_BYTES", "recording.cap_bytes", false, func(c *contract.Config, v string) error { n, err := parseBytes(v); c.Recording.CapBytes = n; return err }},
	{"F1TELE_RECORDING_FREE_RESERVE_BYTES", "recording.free_reserve_bytes", false, func(c *contract.Config, v string) error { n, err := parseBytes(v); c.Recording.FreeReserveBytes = n; return err }},
	{"F1TELE_RECORDING_CHUNK_BYTES", "recording.chunk_bytes", false, func(c *contract.Config, v string) error { n, err := parseBytes(v); c.Recording.ChunkBytes = n; return err }},
	{"F1TELE_RECORDING_ROTATE_BYTES", "recording.rotate_bytes", false, func(c *contract.Config, v string) error { n, err := parseBytes(v); c.Recording.RotateBytes = n; return err }},
	{"F1TELE_RECORDING_CHUNK_FLUSH_MS", "recording.chunk_flush_ms", false, func(c *contract.Config, v string) error {
		n, err := strconv.Atoi(v)
		if err == nil {
			c.Recording.ChunkFlushMs = n
		}
		return err
	}},
	{"F1TELE_SECURITY_ORIGIN_CHECK", "security.origin_check", false, func(c *contract.Config, v string) error {
		b, err := parseBool(v)
		c.Security.OriginCheck = b
		return err
	}},
	{"F1TELE_AGENT_ENABLED", "agent.enabled", false, func(c *contract.Config, v string) error { b, err := parseBool(v); c.Agent.Enabled = b; return err }},
	{"F1TELE_AGENT_ENDPOINT_ID", "agent.endpoint_id", false, func(c *contract.Config, v string) error { c.Agent.EndpointID = &v; return nil }},
	{"F1TELE_AGENT_BASE_URL", "agent.endpoint.base_url", false, nil},
	{"F1TELE_AGENT_MODEL", "agent.endpoint.model", false, nil},
	{"F1TELE_AGENT_PROTOCOL", "agent.endpoint.protocol", false, nil},
	{"F1TELE_AGENT_API_KEY", "agent.endpoint.api_key", false, nil},
	{"F1TELE_SEARCH_PROVIDER", "search.provider", false, func(c *contract.Config, v string) error { c.Search.Provider = v; return nil }},
	{"F1TELE_SEARCH_BASE_URL", "search.base_url", false, func(c *contract.Config, v string) error { c.Search.BaseURL = &v; return nil }},
	{"F1TELE_SEARCH_API_KEY", "search.api_key", false, func(c *contract.Config, v string) error { name := "F1TELE_SEARCH_API_KEY"; c.Search.APIKeyEnv = &name; return nil }},
	{"F1TELE_VOICE_ENABLED", "voice.enabled", false, func(c *contract.Config, v string) error { b, err := parseBool(v); c.Voice.Enabled = b; return err }},
	{"F1TELE_VOICE_BASE_URL", "voice.base_url", false, func(c *contract.Config, v string) error { c.Voice.TranscriptionEndpoint = &v; c.Voice.SpeechEndpoint = &v; return nil }},
	{"F1TELE_VOICE_TRANSCRIPTION_MODEL", "voice.transcription_model", false, func(c *contract.Config, v string) error { c.Voice.TranscriptionModel = &v; return nil }},
	{"F1TELE_VOICE_TRANSCRIPTION_API_KEY", "voice.transcription_api_key", false, func(c *contract.Config, v string) error { name := "F1TELE_VOICE_TRANSCRIPTION_API_KEY"; c.Voice.TranscriptionAPIKeyEnv = &name; return nil }},
	{"F1TELE_VOICE_SPEECH_MODEL", "voice.speech_model", false, func(c *contract.Config, v string) error { c.Voice.SpeechModel = &v; return nil }},
	{"F1TELE_VOICE_SPEECH_API_KEY", "voice.speech_api_key", false, func(c *contract.Config, v string) error { name := "F1TELE_VOICE_SPEECH_API_KEY"; c.Voice.SpeechAPIKeyEnv = &name; return nil }},
}

// ApplyEnv merges documented environment variables into cfg and records the
// source of every value it sets. Unknown F1TELE_* variables are ignored.
func ApplyEnv(cfg *contract.Config, getenv func(string) string, sources map[string]string) error {
	endpointTouched := false
	for _, spec := range envSpecs {
		v := strings.TrimSpace(getenv(spec.name))
		if v == "" {
			continue
		}
		sources[spec.key] = "env"
		if spec.apply != nil {
			if err := spec.apply(cfg, v); err != nil {
				return fmt.Errorf("%s: %v (expected %s)", spec.name, err, expectedFor(spec.name))
			}
		}
		if spec.key == "agent.endpoint.base_url" || spec.key == "agent.endpoint.model" || spec.key == "agent.endpoint.protocol" || spec.key == "agent.endpoint.api_key" {
			endpointTouched = true
		}
	}
	if endpointTouched {
		applyEnvEndpoint(cfg, getenv, sources)
	}
	if getenv("F1TELE_AGENT_API_KEY") != "" {
		if err := ensureEnvEndpoint(cfg, getenv); err != nil {
			return err
		}
		name := "F1TELE_AGENT_API_KEY"
		for i := range cfg.Agent.Endpoints {
			if ep := &cfg.Agent.Endpoints[i]; cfg.Agent.EndpointID != nil && ep.ID == *cfg.Agent.EndpointID {
				ep.APIKeyEnv = &name
			}
		}
	}
	return nil
}

func expectedFor(name string) string {
	switch {
	case strings.HasSuffix(name, "_ADDR") || strings.HasSuffix(name, "_BASE_URL"):
		return "host:port or URL"
	case strings.HasSuffix(name, "_BYTES"):
		return "a non-negative byte count"
	case strings.HasSuffix(name, "_MS"):
		return "an integer number of milliseconds"
	case strings.Contains(name, "ENABLED"):
		return "true or false"
	}
	return "a non-empty value"
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q", v)
}

func parseBytes(v string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte count %q", v)
	}
	if n < 0 {
		return 0, fmt.Errorf("byte count must not be negative: %d", n)
	}
	return n, nil
}

func setAddr(a *contract.Addr, v string) error {
	host, port, err := net.SplitHostPort(v)
	if err != nil {
		return fmt.Errorf("invalid address %q", v)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid port in %q", v)
	}
	a.Host, a.Port = host, n
	return nil
}

func ensureEnvEndpoint(cfg *contract.Config, getenv func(string) string) error {
	id := "env"
	if cfg.Agent.EndpointID != nil && *cfg.Agent.EndpointID != "" {
		id = *cfg.Agent.EndpointID
	}
	for i := range cfg.Agent.Endpoints {
		if cfg.Agent.Endpoints[i].ID == id {
			cfg.Agent.EndpointID = &id
			return nil
		}
	}
	if cfg.Agent.EndpointID == nil {
		cfg.Agent.EndpointID = &id
	}
	if getenv("F1TELE_AGENT_BASE_URL") == "" {
		return fmt.Errorf("F1TELE_AGENT_API_KEY is set but F1TELE_AGENT_BASE_URL is empty")
	}
	for i := range cfg.Agent.Endpoints {
		if cfg.Agent.Endpoints[i].ID == *cfg.Agent.EndpointID {
			return nil
		}
	}
	return fmt.Errorf("agent endpoint %q is not defined", *cfg.Agent.EndpointID)
}

// applyEnvEndpoint creates or updates the endpoint named by the environment.
func applyEnvEndpoint(cfg *contract.Config, getenv func(string) string, sources map[string]string) {
	id := "env"
	if cfg.Agent.EndpointID != nil && *cfg.Agent.EndpointID != "" {
		id = *cfg.Agent.EndpointID
	}
	idx := -1
	for i := range cfg.Agent.Endpoints {
		if cfg.Agent.Endpoints[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		cfg.Agent.Endpoints = append(cfg.Agent.Endpoints, contract.Endpoint{ID: id})
		idx = len(cfg.Agent.Endpoints) - 1
	}
	ep := &cfg.Agent.Endpoints[idx]
	if v := getenv("F1TELE_AGENT_BASE_URL"); v != "" {
		ep.BaseURL = v
	}
	if v := getenv("F1TELE_AGENT_MODEL"); v != "" {
		ep.Model = v
	}
	if v := getenv("F1TELE_AGENT_PROTOCOL"); v != "" {
		ep.Protocol = v
	}
	if ep.Protocol == "" {
		ep.Protocol = "chat_completions"
	}
	if ep.ContextLimitTokens == 0 {
		ep.ContextLimitTokens = 128000
	}
	cfg.Agent.EndpointID = &id
	sources["agent.endpoint"] = "env"
}

// AdvertiseHost returns the host a game on another machine should target.
func AdvertiseHost(explicit string) string {
	if explicit != "" {
		return explicit
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ip, ok := a.(*net.IPNet); ok && !ip.IP.IsLoopback() {
				if v4 := ip.IP.To4(); v4 != nil {
					return v4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

// RestartOnlyKeys is the static restart classification shown by Settings.
var RestartOnlyKeys = []string{"http", "udp", "data_dir"}

// DocumentedVariables is the operator-facing list. It exists so tests can keep
// the reference table and the loader in step.
var DocumentedVariables = []string{
	"F1TELE_DATA_DIR", "F1TELE_CONFIG", "F1TELE_HTTP_ADDR", "F1TELE_UDP_ADDR", "F1TELE_UDP_ADVERTISE_HOST",
	"F1TELE_RECORDING_ENABLED", "F1TELE_RECORDING_CAP_BYTES", "F1TELE_RECORDING_FREE_RESERVE_BYTES",
	"F1TELE_RECORDING_CHUNK_BYTES", "F1TELE_RECORDING_ROTATE_BYTES", "F1TELE_RECORDING_CHUNK_FLUSH_MS",
	"F1TELE_SECURITY_ORIGIN_CHECK",
	"F1TELE_AGENT_ENABLED", "F1TELE_AGENT_ENDPOINT_ID", "F1TELE_AGENT_BASE_URL", "F1TELE_AGENT_MODEL",
	"F1TELE_AGENT_PROTOCOL", "F1TELE_AGENT_API_KEY", "F1TELE_SEARCH_PROVIDER", "F1TELE_SEARCH_BASE_URL",
	"F1TELE_SEARCH_API_KEY", "F1TELE_VOICE_ENABLED", "F1TELE_VOICE_BASE_URL", "F1TELE_VOICE_TRANSCRIPTION_MODEL",
	"F1TELE_VOICE_TRANSCRIPTION_API_KEY", "F1TELE_VOICE_SPEECH_MODEL", "F1TELE_VOICE_SPEECH_API_KEY",
}
