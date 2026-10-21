package contract

type Config struct {
	SchemaVersion     string             `json:"schema_version"`
	HTTP              Addr               `json:"http"`
	UDP               Addr               `json:"udp"`
	Recording         RecordingConfig    `json:"recording"`
	Security          SecurityConfig     `json:"security"`
	Agent             AgentConfig        `json:"agent"`
	Notifications     NotificationConfig `json:"notifications"`
	Search            SearchConfig       `json:"search"`
	Voice             VoiceConfig        `json:"voice"`
	DataDir           *string            `json:"data_dir"`
	ReferenceProfiles []ReferenceProfile `json:"reference_profiles"`
	CapacityProfiles  []CapacityProfile  `json:"capacity_profiles"`
	TrackProfiles     []TrackProfile     `json:"track_profiles"`
}

type Addr struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type RecordingConfig struct {
	Enabled          bool    `json:"enabled"`
	CapBytes         int64   `json:"cap_bytes"`
	FreeReserveBytes int64   `json:"free_reserve_bytes"`
	WarnFraction     float64 `json:"warn_fraction"`
	QueueBytes       int64   `json:"queue_bytes"`
	ChunkBytes       int64   `json:"chunk_bytes"`
	ChunkFlushMs     int     `json:"chunk_flush_ms"`
	RotateBytes      int64   `json:"rotate_bytes"`
	AutomaticCleanup bool    `json:"automatic_cleanup"`
}

type SecurityConfig struct {
	OriginCheck bool `json:"origin_check"`
}

type AgentConfig struct {
	Enabled                bool             `json:"enabled"`
	EndpointID             *string          `json:"endpoint_id"`
	Endpoints              []Endpoint       `json:"endpoints"`
	Rules                  []TriggerRule    `json:"rules"`
	MinimumDispatchGapSecs int              `json:"minimum_dispatch_gap_seconds"`
	MaxCallsPerHour        int              `json:"max_calls_per_hour"`
	MaxTokensPerHour       int              `json:"max_tokens_per_hour"`
	MaxCostUSDPerHour      *float64         `json:"max_cost_usd_per_hour"`
	LiveDeadlineMs         int              `json:"live_deadline_ms"`
	AnalysisDeadlineMs     int              `json:"analysis_deadline_ms"`
	ContextTokens          int              `json:"context_tokens"`
	MaxOutputTokens        int              `json:"max_output_tokens"`
	MaxSteps               int              `json:"max_steps"`
	MaxToolCalls           int              `json:"max_tool_calls"`
	Continuous             ContinuousConfig `json:"continuous"`
	Compaction             CompactionConfig `json:"compaction"`
	Generation             GenerationConfig `json:"generation"`
}

type CompactionConfig struct {
	Enabled           bool `json:"enabled"`
	TriggerPercent    int  `json:"trigger_percent"`
	RecentTailPercent int  `json:"recent_tail_percent"`
}

type GenerationConfig struct {
	Temperature       *float64 `json:"temperature"`
	TopP              *float64 `json:"top_p"`
	ParallelToolCalls *bool    `json:"parallel_tool_calls"`
}

type ContinuousConfig struct {
	Enabled        bool `json:"enabled"`
	CadenceSeconds int  `json:"cadence_seconds"`
	MaxMinutes     int  `json:"max_minutes"`
}

type Endpoint struct {
	ID                  string   `json:"id"`
	Protocol            string   `json:"protocol"`
	BaseURL             string   `json:"base_url"`
	Model               string   `json:"model"`
	APIKeyEnv           *string  `json:"api_key_env"`
	ContextLimitTokens  int      `json:"context_limit_tokens"`
	InputUSDPerMillion  *float64 `json:"input_usd_per_million"`
	OutputUSDPerMillion *float64 `json:"output_usd_per_million"`
}

type TriggerRule struct {
	ID              string      `json:"id"`
	Enabled         bool        `json:"enabled"`
	Predicate       Predicate   `json:"predicate"`
	Filters         RuleFilters `json:"filters"`
	CooldownSeconds int         `json:"cooldown_seconds"`
}

type RuleFilters struct {
	ModeFamilies    []string `json:"mode_families"`
	CarScope        string   `json:"car_scope"`
	MinimumSeverity string   `json:"minimum_severity"`
	Validity        string   `json:"validity"`
}

type Predicate struct {
	Kind               string      `json:"kind"`
	Children           []Predicate `json:"children,omitempty"`
	EveryCompletedLaps *int        `json:"every_completed_laps,omitempty"`
	Seconds            *int        `json:"seconds,omitempty"`
	EventTypes         []string    `json:"event_types,omitempty"`
}

type NotificationConfig struct {
	RoutineGapSeconds     int `json:"routine_gap_seconds"`
	CautionPersistenceMs  int `json:"caution_persistence_ms"`
	CriticalPersistenceMs int `json:"critical_persistence_ms"`
	ClearPersistenceMs    int `json:"clear_persistence_ms"`
}

type SearchConfig struct {
	Provider   string  `json:"provider"`
	BaseURL    *string `json:"base_url"`
	APIKeyEnv  *string `json:"api_key_env"`
	MaxResults int     `json:"max_results"`
}

type VoiceConfig struct {
	Enabled                bool    `json:"enabled"`
	PushToTalk             string  `json:"push_to_talk"`
	TranscriptionEndpoint  *string `json:"transcription_endpoint"`
	TranscriptionModel     *string `json:"transcription_model"`
	SpeechEndpoint         *string `json:"speech_endpoint"`
	SpeechModel            *string `json:"speech_model"`
	SpeechVoice            *string `json:"speech_voice"`
	MaxUtteranceSeconds    int     `json:"max_utterance_seconds"`
	TranscriptionAPIKeyEnv *string `json:"transcription_api_key_env"`
	SpeechAPIKeyEnv        *string `json:"speech_api_key_env"`
}

type ReferenceProfile struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Channels          []string `json:"channels"`
	RegulationSeasons []int    `json:"regulation_seasons"`
	FormulaCodes      []int    `json:"formula_codes"`
	ActualCompounds   []string `json:"actual_compounds"`
	ColdBelow         *float64 `json:"cold_below"`
	TargetLow         *float64 `json:"target_low"`
	TargetHigh        *float64 `json:"target_high"`
	CautionMin        *float64 `json:"caution_min"`
	CriticalMin       *float64 `json:"critical_min"`
	Unit              string   `json:"unit"`
	CalibrationStatus string   `json:"calibration_status"`
	SourceIDs         []string `json:"source_ids"`
	Notes             string   `json:"notes"`
	Enabled           bool     `json:"enabled"`
}

type CapacityProfile struct {
	ID                string   `json:"id"`
	RegulationSeasons []int    `json:"regulation_seasons"`
	FormulaCodes      []int    `json:"formula_codes"`
	CapacityJ         float64  `json:"capacity_j"`
	SourceIDs         []string `json:"source_ids"`
	CalibrationStatus string   `json:"calibration_status"`
	Enabled           bool     `json:"enabled"`
}

type TrackProfile struct {
	ID                string          `json:"id"`
	TrackID           int             `json:"track_id"`
	RegulationSeasons []int           `json:"regulation_seasons"`
	GameBuild         *string         `json:"game_build"`
	PitLossMs         *float64        `json:"pit_loss_ms"`
	Corners           []CornerProfile `json:"corners"`
	SourceIDs         []string        `json:"source_ids"`
	CalibrationStatus string          `json:"calibration_status"`
	Enabled           bool            `json:"enabled"`
}

type CornerProfile struct {
	ID             string  `json:"id"`
	EntryDistanceM float64 `json:"entry_distance_m"`
	ExitDistanceM  float64 `json:"exit_distance_m"`
	Validated      bool    `json:"validated"`
}

type ConfigView struct {
	Revision          int               `json:"revision"`
	ActiveConfig      Config            `json:"active_config"`
	DesiredConfig     Config            `json:"desired_config"`
	SecretsConfigured map[string]bool   `json:"secrets_configured"`
	RestartRequired   []string          `json:"restart_required"`
	Sources           map[string]string `json:"sources"`
	RestartOnly       []string          `json:"restart_only"`
	UDPAdvertiseHost  string            `json:"udp_advertise_host"`
}

type ConfigPatch struct {
	Config          Config        `json:"config"`
	Secrets         []SecretInput `json:"secrets"`
	DeleteSecretIDs []string      `json:"delete_secret_ids"`
}

type SecretInput struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}
