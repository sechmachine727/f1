// Package config loads the first-run defaults and applies the documented
// precedence: CLI > environment > config.json > defaults.json.
package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
)

//go:embed defaults.json
var defaultBytes []byte

// Defaults returns a fresh copy of the shipped first-run configuration.
func Defaults() (contract.Config, error) {
	var c contract.Config
	if err := json.Unmarshal(defaultBytes, &c); err != nil {
		return c, fmt.Errorf("decode embedded defaults: %w", err)
	}
	return c, nil
}

// DefaultDataDir follows the documented native layout.
func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "F1Telemetry")
		}
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "f1tele")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "f1tele")
}

// ResolveDataDir applies the documented CLI, environment and default order
// without requiring the rest of the configuration to be valid.
func ResolveDataDir(explicit string) string {
	def, _ := Defaults()
	return firstNonEmpty(explicit, os.Getenv("F1TELE_DATA_DIR"), valueOr(def.DataDir, ""), DefaultDataDir())
}

// Overrides are the explicit CLI values; zero values fall through.
type Overrides struct {
	HTTPAddress string
	UDPAddress  string
	DataDir     string
	ConfigPath  string
}

type Store struct {
	mu        sync.Mutex
	path      string
	revision  int
	active    contract.Config
	desired   contract.Config
	secrets   map[string]string
	sources   map[string]string
	advertise string
}

// Load resolves configuration. It never fails on a missing config file.
func Load(ov Overrides) (*Store, error) {
	def, err := Defaults()
	if err != nil {
		return nil, err
	}
	dataDir := ResolveDataDir(ov.DataDir)
	configPath := firstNonEmpty(ov.ConfigPath, os.Getenv("F1TELE_CONFIG"), filepath.Join(dataDir, "config.json"))

	cfg := def
	fileCfg := def
	if raw, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		fileCfg = cfg
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	cfg.DataDir = &dataDir

	sources := map[string]string{"data_dir": "default"}
	if ov.DataDir != "" {
		sources["data_dir"] = "cli"
	} else if os.Getenv("F1TELE_DATA_DIR") != "" {
		sources["data_dir"] = "env"
	} else if valueOr(def.DataDir, "") != "" {
		sources["data_dir"] = "default"
	}
	if err := ApplyEnv(&cfg, os.Getenv, sources); err != nil {
		return nil, err
	}
	for _, spec := range envSpecs {
		if sources[spec.key] != "" {
			continue
		}
		if configValue(fileCfg, spec.key) != configValue(def, spec.key) {
			sources[spec.key] = "file"
		} else {
			sources[spec.key] = "default"
		}
	}
	if ov.HTTPAddress != "" {
		if err := setAddr(&cfg.HTTP, ov.HTTPAddress); err != nil {
			return nil, fmt.Errorf("--http: %v", err)
		}
		sources["http"] = "cli"
	}
	if ov.UDPAddress != "" {
		if err := setAddr(&cfg.UDP, ov.UDPAddress); err != nil {
			return nil, fmt.Errorf("--udp: %v", err)
		}
		sources["udp"] = "cli"
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	s := &Store{path: configPath, active: cfg, desired: cfg, secrets: map[string]string{}, sources: sources,
		advertise: AdvertiseHost(os.Getenv("F1TELE_UDP_ADVERTISE_HOST"))}
	if raw, err := os.ReadFile(filepath.Join(dataDir, "secrets.json")); err == nil {
		if err := json.Unmarshal(raw, &s.secrets); err != nil {
			return nil, fmt.Errorf("parse secrets: %w", err)
		}
	}
	return s, nil
}

func valueOr(p *string, fallback string) string {
	if p != nil && *p != "" {
		return *p
	}
	return fallback
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func applyAddr(a *contract.Addr, addr string) {
	if addr == "" {
		return
	}
	if i := strings.LastIndex(addr, ":"); i > 0 {
		fmt.Sscanf(addr[i+1:], "%d", &a.Port)
		a.Host = addr[:i]
	}
}

func (s *Store) Config() contract.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.desired
}

func (s *Store) Active() contract.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *Store) DataDir() string {
	c := s.Config()
	if c.DataDir != nil && *c.DataDir != "" {
		return *c.DataDir
	}
	return DefaultDataDir()
}

func (s *Store) Path() string { return s.path }

func (s *Store) Revision() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

// Snapshot returns the desired config and its revision under one lock so a
// concurrent patch cannot pair an old config with a new revision.
func (s *Store) Snapshot() (contract.Config, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.desired, s.revision
}

// View returns the redacted configuration view.
func (s *Store) View() contract.ConfigView {
	s.mu.Lock()
	defer s.mu.Unlock()
	configured := map[string]bool{}
	for _, id := range secretIDs(s.desired) {
		configured[id] = s.resolveSecretLocked(id) != ""
	}
	return contract.ConfigView{
		Revision:          s.revision,
		ActiveConfig:      s.active,
		DesiredConfig:     s.desired,
		SecretsConfigured: configured,
		RestartRequired:   restartFields(s.active, s.desired),
		Sources:           s.sourcesViewLocked(),
		RestartOnly:       append([]string(nil), RestartOnlyKeys...),
		UDPAdvertiseHost:  s.advertise,
	}
}

func (s *Store) sourcesViewLocked() map[string]string {
	out := map[string]string{}
	for k, v := range s.sources {
		out[k] = v
	}
	return out
}

func secretIDs(c contract.Config) []string {
	var ids []string
	for _, e := range c.Agent.Endpoints {
		if e.APIKeyEnv != nil && *e.APIKeyEnv != "" {
			ids = append(ids, *e.APIKeyEnv)
		}
	}
	if c.Search.APIKeyEnv != nil && *c.Search.APIKeyEnv != "" {
		ids = append(ids, *c.Search.APIKeyEnv)
	}
	if c.Voice.TranscriptionAPIKeyEnv != nil && *c.Voice.TranscriptionAPIKeyEnv != "" {
		ids = append(ids, *c.Voice.TranscriptionAPIKeyEnv)
	}
	if c.Voice.SpeechAPIKeyEnv != nil && *c.Voice.SpeechAPIKeyEnv != "" {
		ids = append(ids, *c.Voice.SpeechAPIKeyEnv)
	}
	return ids
}

func (s *Store) resolveSecretLocked(id string) string {
	if v := os.Getenv(id); v != "" {
		return v
	}
	return s.secrets[id]
}

// Secret resolves an environment-provided value first, then the local store.
func (s *Store) Secret(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolveSecretLocked(id)
}

func restartFields(a, b contract.Config) []string {
	// The wire contract requires an array, so an unchanged config reports [] and
	// never null.
	out := []string{}
	if a.HTTP != b.HTTP {
		out = append(out, "http")
	}
	if a.UDP != b.UDP {
		out = append(out, "udp")
	}
	if valueOr(a.DataDir, "") != valueOr(b.DataDir, "") {
		out = append(out, "data_dir")
	}
	if a.Recording != b.Recording {
		out = append(out, "recording")
	}
	return out
}

// ErrRevision reports a stale If-Match precondition.
var ErrRevision = errors.New("configuration revision mismatch")

// ApplyPatch validates and persists a complete desired configuration.
func (s *Store) ApplyPatch(p contract.ConfigPatch, ifMatch *int) (contract.ConfigView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ifMatch == nil {
		return contract.ConfigView{}, ErrRevision
	}
	if *ifMatch != s.revision {
		return contract.ConfigView{}, ErrRevision
	}
	if err := Validate(p.Config); err != nil {
		return contract.ConfigView{}, err
	}
	if err := envLockedDiff(s.sources, s.desired, p.Config); err != nil {
		return contract.ConfigView{}, err
	}
	secrets := make(map[string]string, len(s.secrets)+len(p.Secrets))
	for k, v := range s.secrets {
		secrets[k] = v
	}
	for _, sec := range p.Secrets {
		if sec.ID == "" || sec.Value == "" {
			return contract.ConfigView{}, fmt.Errorf("secret id and value are required")
		}
		secrets[sec.ID] = sec.Value
	}
	for _, id := range p.DeleteSecretIDs {
		delete(secrets, id)
	}
	dataDir := valueOr(p.Config.DataDir, "")
	if dataDir == "" {
		dataDir = valueOr(s.desired.DataDir, DefaultDataDir())
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return contract.ConfigView{}, err
	}
	raw, err := json.MarshalIndent(p.Config, "", "  ")
	if err != nil {
		return contract.ConfigView{}, err
	}
	if err := writeAtomic(filepath.Join(dataDir, "config.json"), raw, 0o600); err != nil {
		return contract.ConfigView{}, err
	}
	secRaw, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return contract.ConfigView{}, err
	}
	if err := writeAtomic(filepath.Join(dataDir, "secrets.json"), secRaw, 0o600); err != nil {
		return contract.ConfigView{}, err
	}
	s.desired = p.Config
	s.secrets = secrets
	s.revision++
	for _, spec := range envSpecs {
		if s.sources[spec.key] != "env" && configValue(s.desired, spec.key) != configValue(s.active, spec.key) {
			s.sources[spec.key] = "file"
		}
	}
	configured := map[string]bool{}
	for _, id := range secretIDs(s.desired) {
		configured[id] = s.resolveSecretLocked(id) != ""
	}
	return contract.ConfigView{
		Revision:          s.revision,
		ActiveConfig:      s.active,
		DesiredConfig:     s.desired,
		SecretsConfigured: configured,
		RestartRequired:   restartFields(s.active, s.desired),
		Sources:           s.sourcesViewLocked(),
		RestartOnly:       append([]string(nil), RestartOnlyKeys...),
		UDPAdvertiseHost:  s.advertise,
	}, nil
}

// ErrEnvLocked reports a change to a value owned by the environment.
var ErrEnvLocked = errors.New("configuration is set by the environment")

func envLockedDiff(sources map[string]string, old, next contract.Config) error {
	for _, spec := range envSpecs {
		if sources[spec.key] != "env" {
			continue
		}
		if configValue(old, spec.key) != configValue(next, spec.key) {
			return fmt.Errorf("%w: %s cannot be changed while %s is set", ErrEnvLocked, spec.key, spec.name)
		}
	}
	return nil
}

func configValue(c contract.Config, key string) string {
	switch key {
	case "http":
		return c.HTTP.Host + ":" + strconv.Itoa(c.HTTP.Port)
	case "udp":
		return c.UDP.Host + ":" + strconv.Itoa(c.UDP.Port)
	case "udp_advertise_host":
		return ""
	case "recording.enabled":
		return strconv.FormatBool(c.Recording.Enabled)
	case "recording.cap_bytes":
		return strconv.FormatInt(c.Recording.CapBytes, 10)
	case "recording.free_reserve_bytes":
		return strconv.FormatInt(c.Recording.FreeReserveBytes, 10)
	case "recording.chunk_bytes":
		return strconv.FormatInt(c.Recording.ChunkBytes, 10)
	case "recording.rotate_bytes":
		return strconv.FormatInt(c.Recording.RotateBytes, 10)
	case "recording.chunk_flush_ms":
		return strconv.Itoa(c.Recording.ChunkFlushMs)
	case "security.origin_check":
		return strconv.FormatBool(c.Security.OriginCheck)
	case "session.inactivity_seconds":
		return strconv.Itoa(c.Session.InactivitySeconds)
	case "agent.enabled":
		return strconv.FormatBool(c.Agent.Enabled)
	case "agent.endpoint_id":
		return valueOr(c.Agent.EndpointID, "")
	case "agent.endpoint.base_url", "agent.endpoint.model", "agent.endpoint.protocol", "agent.endpoint.api_key", "agent.endpoint":
		return endpointValues(c.Agent)
	case "search.provider":
		return c.Search.Provider
	case "search.base_url":
		return valueOr(c.Search.BaseURL, "")
	case "search.api_key":
		return valueOr(c.Search.APIKeyEnv, "")
	case "voice.enabled":
		return strconv.FormatBool(c.Voice.Enabled)
	case "voice.base_url":
		return valueOr(c.Voice.TranscriptionEndpoint, "") + "|" + valueOr(c.Voice.SpeechEndpoint, "")
	case "voice.transcription_model":
		return valueOr(c.Voice.TranscriptionModel, "")
	case "voice.transcription_api_key":
		return valueOr(c.Voice.TranscriptionAPIKeyEnv, "")
	case "voice.speech_model":
		return valueOr(c.Voice.SpeechModel, "")
	case "voice.speech_api_key":
		return valueOr(c.Voice.SpeechAPIKeyEnv, "")
	}
	return ""
}

func endpointValues(a contract.AgentConfig) string {
	id := valueOr(a.EndpointID, "")
	out := id
	for _, e := range a.Endpoints {
		if e.ID == id {
			out += "|" + e.BaseURL + "|" + e.Model + "|" + e.Protocol + "|" + valueOr(e.APIKeyEnv, "")
		}
	}
	return out
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Validate enforces the documented semantic configuration rules.
func Validate(c contract.Config) error {
	if c.SchemaVersion != contract.SchemaVersion {
		return fmt.Errorf("schema_version must be %s", contract.SchemaVersion)
	}
	for name, a := range map[string]contract.Addr{"http": c.HTTP, "udp": c.UDP} {
		if a.Port < 1 || a.Port > 65535 {
			return fmt.Errorf("%s port out of range: %d", name, a.Port)
		}
	}
	r := c.Recording
	if r.CapBytes <= 0 || r.FreeReserveBytes < 0 || r.QueueBytes <= 0 || r.ChunkBytes <= 0 || r.RotateBytes <= 0 {
		return errors.New("recording capacities must be positive")
	}
	if r.WarnFraction <= 0 || r.WarnFraction > 1 {
		return errors.New("recording warn_fraction must be in (0,1]")
	}
	if r.AutomaticCleanup {
		return errors.New("automatic_cleanup cannot be enabled")
	}
	if c.Session.InactivitySeconds < 30 {
		return errors.New("session inactivity_seconds must be at least 30")
	}
	ids := map[string]bool{}
	for _, e := range c.Agent.Endpoints {
		if e.ID == "" || ids[e.ID] {
			return fmt.Errorf("duplicate or empty endpoint id %q", e.ID)
		}
		ids[e.ID] = true
		if e.Protocol != "chat_completions" && e.Protocol != "responses" {
			return fmt.Errorf("endpoint %s: unsupported protocol %q", e.ID, e.Protocol)
		}
		if e.Model == "" || e.BaseURL == "" {
			return fmt.Errorf("endpoint %s: model and base_url are required", e.ID)
		}
	}
	if c.Agent.EndpointID != nil && !ids[*c.Agent.EndpointID] {
		return fmt.Errorf("agent.endpoint_id %q is not configured", *c.Agent.EndpointID)
	}
	rIDs := map[string]bool{}
	for _, rule := range c.Agent.Rules {
		if rule.ID == "" || rIDs[rule.ID] {
			return fmt.Errorf("duplicate or empty rule id %q", rule.ID)
		}
		rIDs[rule.ID] = true
		depth, nodes, err := predicateSize(rule.Predicate, 0)
		if err != nil {
			return fmt.Errorf("rule %s: %w", rule.ID, err)
		}
		if depth > 4 || nodes > 64 {
			return fmt.Errorf("rule %s: predicate depth %d nodes %d exceeds limits", rule.ID, depth, nodes)
		}
	}
	if c.Agent.MaxCallsPerHour <= 0 || c.Agent.MaxTokensPerHour <= 0 || c.Agent.MaxSteps <= 0 || c.Agent.MaxToolCalls < 0 {
		return errors.New("agent budgets must be positive")
	}
	if c.Agent.Continuous.CadenceSeconds <= 0 || c.Agent.Continuous.MaxMinutes <= 0 {
		return errors.New("continuous cadence and duration must be positive")
	}
	if c.Agent.Compaction.TriggerPercent < 10 || c.Agent.Compaction.TriggerPercent > 99 {
		return errors.New("compaction trigger_percent must be between 10 and 99")
	}
	if c.Agent.Compaction.RecentTailPercent < 5 || c.Agent.Compaction.RecentTailPercent > 50 {
		return errors.New("compaction recent_tail_percent must be between 5 and 50")
	}
	if c.Agent.Compaction.RecentTailPercent >= c.Agent.Compaction.TriggerPercent {
		return errors.New("compaction recent tail must be smaller than the trigger")
	}
	if c.Agent.Generation.Temperature != nil && (*c.Agent.Generation.Temperature < 0 || *c.Agent.Generation.Temperature > 2) {
		return errors.New("agent temperature must be between 0 and 2")
	}
	if c.Agent.Generation.TopP != nil && (*c.Agent.Generation.TopP < 0 || *c.Agent.Generation.TopP > 1) {
		return errors.New("agent top_p must be between 0 and 1")
	}
	if c.Notifications.RoutineGapSeconds < 0 {
		return errors.New("routine gap must not be negative")
	}
	pids := map[string]bool{}
	for _, p := range c.ReferenceProfiles {
		if p.ID == "" || pids[p.ID] {
			return fmt.Errorf("duplicate or empty profile id %q", p.ID)
		}
		pids[p.ID] = true
		if p.Enabled && len(p.Channels) == 0 {
			return fmt.Errorf("profile %s: channels are required", p.ID)
		}
		if p.CautionMin != nil && p.CriticalMin != nil && *p.CriticalMin < *p.CautionMin {
			return fmt.Errorf("profile %s: critical_min below caution_min", p.ID)
		}
		if p.TargetLow != nil && p.TargetHigh != nil && *p.TargetLow > *p.TargetHigh {
			return fmt.Errorf("profile %s: target range inverted", p.ID)
		}
		if p.ColdBelow != nil && p.TargetLow != nil && *p.ColdBelow > *p.TargetLow {
			return fmt.Errorf("profile %s: cold_below above target_low", p.ID)
		}
		if p.TargetHigh != nil && p.CautionMin != nil && *p.TargetHigh > *p.CautionMin {
			return fmt.Errorf("profile %s: target_high above caution_min", p.ID)
		}
		for _, ch := range p.Channels {
			if spec, ok := lookupChannel(ch); ok && spec.Unit != p.Unit {
				return fmt.Errorf("profile %s: unit %s does not match channel %s unit %s", p.ID, p.Unit, ch, spec.Unit)
			}
		}
	}
	for _, p := range c.CapacityProfiles {
		if p.ID == "" || pids[p.ID] {
			return fmt.Errorf("duplicate or empty capacity profile id %q", p.ID)
		}
		pids[p.ID] = true
		if p.CapacityJ <= 0 {
			return fmt.Errorf("capacity profile %s must be positive", p.ID)
		}
	}
	for _, p := range c.TrackProfiles {
		if p.ID == "" || pids[p.ID] {
			return fmt.Errorf("duplicate or empty track profile id %q", p.ID)
		}
		pids[p.ID] = true
		for _, corner := range p.Corners {
			if corner.ExitDistanceM <= corner.EntryDistanceM {
				return fmt.Errorf("track profile %s corner %s has inverted bounds", p.ID, corner.ID)
			}
		}
	}
	if c.Search.Provider != "disabled" && c.Search.Provider != "tavily" && c.Search.Provider != "searxng" {
		return fmt.Errorf("unsupported search provider %q", c.Search.Provider)
	}
	return nil
}

func predicateSize(p contract.Predicate, depth int) (int, int, error) {
	if depth > 8 {
		return depth, 0, errors.New("predicate nesting too deep")
	}
	maxDepth := depth
	nodes := 1
	switch p.Kind {
	case "any", "all":
		if len(p.Children) == 0 || len(p.Children) > 16 {
			return depth, nodes, errors.New("any/all requires 1-16 children")
		}
		for _, ch := range p.Children {
			d, n, err := predicateSize(ch, depth+1)
			if err != nil {
				return depth, nodes, err
			}
			if d > maxDepth {
				maxDepth = d
			}
			nodes += n
		}
	case "lap":
		if p.EveryCompletedLaps == nil || *p.EveryCompletedLaps < 1 {
			return depth, nodes, errors.New("lap predicate requires every_completed_laps")
		}
	case "interval":
		if p.Seconds == nil || *p.Seconds < 1 {
			return depth, nodes, errors.New("interval predicate requires seconds")
		}
	case "event":
		if len(p.EventTypes) == 0 {
			return depth, nodes, errors.New("event predicate requires event_types")
		}
	case "manual":
	default:
		return depth, nodes, fmt.Errorf("unknown predicate kind %q", p.Kind)
	}
	return maxDepth, nodes, nil
}

func lookupChannel(name string) (protocol.ChannelSpec, bool) { return protocol.Lookup(name) }
