// Package contract holds the v1 wire shapes from docs/rework/contracts/v1.schema.json.
// Field names are the normative snake_case names; omitempty is never used because
// the schema requires every listed property.
package contract

const SchemaVersion = "1.0.0"

type Measurement struct {
	Value        any     `json:"value"`
	Availability string  `json:"availability"`
	Unit         string  `json:"unit"`
	AsOfMs       *int64  `json:"as_of_ms"`
	Basis        string  `json:"basis"`
	SourceID     string  `json:"source_id"`
	Confidence   string  `json:"confidence"`
	Reason       *string `json:"reason"`
}

func Available(v any, unit, source string) Measurement {
	return Measurement{Value: v, Availability: "available", Unit: unit, Basis: "observed", SourceID: source, Confidence: "high"}
}

func Unavailable(kind, unit, source, reason string) Measurement {
	return Measurement{Availability: kind, Unit: unit, Basis: "observed", SourceID: source, Confidence: "none", Reason: &reason}
}

func Derived(v any, unit, method string) Measurement {
	return Measurement{Value: v, Availability: "available", Unit: unit, Basis: "derived", SourceID: method, Confidence: "medium"}
}

type Context struct {
	SessionID        string   `json:"session_id"`
	FragmentID       string   `json:"fragment_id"`
	BranchID         string   `json:"branch_id"`
	GameSessionUID   *string  `json:"game_session_uid"`
	SourceKind       string   `json:"source_kind"`
	UDPFormat        *int     `json:"udp_format"`
	GameYear         *int     `json:"game_year"`
	RegulationSeason *int     `json:"regulation_season"`
	SeasonEvidence   []string `json:"season_evidence"`
	SeasonLinkID     *string  `json:"season_link_id"`
	WeekendLinkID    *string  `json:"weekend_link_id"`
	SessionLinkID    *string  `json:"session_link_id"`
	GameModeCode     *int     `json:"game_mode_code"`
	ModeFamily       string   `json:"mode_family"`
	SessionTypeCode  *int     `json:"session_type_code"`
	Online           *bool    `json:"online"`
	TrackID          *int     `json:"track_id"`
	PlayerCarIndex   *int     `json:"player_car_index"`
	Revision         int      `json:"revision"`
	SourceContextID  string   `json:"source_context_id"`
	FormulaCode      *int     `json:"formula_code"`
	GameMajorVersion *int     `json:"game_major_version"`
	GameMinorVersion *int     `json:"game_minor_version"`
}

type Metric struct {
	ID          string      `json:"id"`
	CarIndex    *int        `json:"car_index"`
	Channel     string      `json:"channel"`
	Measurement Measurement `json:"measurement"`
}

type Event struct {
	ID            string   `json:"id"`
	Seq           string   `json:"seq"`
	Type          string   `json:"type"`
	Severity      string   `json:"severity"`
	SessionTimeMs *int64   `json:"session_time_ms"`
	CarIndex      *int     `json:"car_index"`
	Summary       string   `json:"summary"`
	EvidenceIDs   []string `json:"evidence_ids"`
}

type Estimate struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
	Measurement   Measurement `json:"measurement"`
	Assumptions   []string    `json:"assumptions"`
	EvidenceIDs   []string    `json:"evidence_ids"`
	MethodVersion string      `json:"method_version"`
}

type Coverage struct {
	Truncated         bool     `json:"truncated"`
	OmittedEventCount int      `json:"omitted_event_count"`
	Gaps              []string `json:"gaps"`
	StaleChannels     []string `json:"stale_channels"`
}

type Range struct {
	FromSeq string `json:"from_seq"`
	ToSeq   string `json:"to_seq"`
}

type Observation struct {
	SchemaVersion      string              `json:"schema_version"`
	Context            Context             `json:"context"`
	ObservationSeq     string              `json:"observation_seq"`
	CapturedAtMs       int64               `json:"captured_at_ms"`
	SessionTimeMs      *int64              `json:"session_time_ms"`
	Lap                *int                `json:"lap"`
	Window             Range               `json:"window"`
	Metrics            []Metric            `json:"metrics"`
	Events             []Event             `json:"events"`
	Estimates          []Estimate          `json:"estimates"`
	Coverage           Coverage            `json:"coverage"`
	StrategyEvaluation *StrategyEvaluation `json:"strategy_evaluation"`
}

type StreamSubscription struct {
	Type       string   `json:"type"`
	SchemaVer  string   `json:"schema_version"`
	RequestID  string   `json:"request_id"`
	SourceID   string   `json:"source_id"`
	CarIndices []int    `json:"car_indices"`
	Channels   []string `json:"channels"`
	MaxHz      int      `json:"max_hz"`
	AfterSeq   *string  `json:"after_seq"`
}

type StreamMessage struct {
	SchemaVersion    string        `json:"schema_version"`
	Type             string        `json:"type"`
	SourceID         string        `json:"source_id"`
	Seq              string        `json:"seq"`
	BaseSeq          *string       `json:"base_seq"`
	Context          *Context      `json:"context"`
	Metrics          []Metric      `json:"metrics"`
	RemovedMetricIDs []string      `json:"removed_metric_ids"`
	Events           []Event       `json:"events"`
	Status           *string       `json:"status"`
	EngineerOutputs  []AgentOutput `json:"engineer_outputs"`
	Alerts           []Alert       `json:"alerts"`
	Jobs             []Job         `json:"jobs"`
	ObservedAtMs     int64         `json:"observed_at_ms,omitempty"`
	SentAtMs         int64         `json:"sent_at_ms,omitempty"`
}

type AgentOutput struct {
	SchemaVersion    string        `json:"schema_version"`
	InvocationID     string        `json:"invocation_id"`
	SessionID        string        `json:"session_id"`
	BranchID         string        `json:"branch_id"`
	ObservationSeq   string        `json:"observation_seq"`
	Decision         string        `json:"decision"`
	Confidence       string        `json:"confidence"`
	Notify           bool          `json:"notify"`
	Urgency          string        `json:"urgency"`
	DriverMessage    *string       `json:"driver_message"`
	Plan             Plan          `json:"plan"`
	RationaleSummary string        `json:"rationale_summary"`
	EvidenceIDs      []string      `json:"evidence_ids"`
	Assumptions      []string      `json:"assumptions"`
	Predictions      []Prediction  `json:"predictions"`
	Recheck          Recheck       `json:"recheck"`
	Analysis         string        `json:"analysis"`
	Limitations      []string      `json:"limitations"`
	Sources          []Source      `json:"sources"`
	SetupChanges     []SetupChange `json:"setup_changes"`
}

type Plan struct {
	Action       string   `json:"action"`
	PitLapStart  *int     `json:"pit_lap_start"`
	PitLapEnd    *int     `json:"pit_lap_end"`
	Compound     *string  `json:"compound"`
	Alternatives []string `json:"alternatives"`
	CandidateID  *string  `json:"candidate_id"`
}

type Prediction struct {
	MetricID       string   `json:"metric_id"`
	ExpectedChange string   `json:"expected_change"`
	CheckAfterLaps *int     `json:"check_after_laps"`
	EvidenceIDs    []string `json:"evidence_ids"`
}

type Recheck struct {
	AfterCompletedLaps *int     `json:"after_completed_laps"`
	EventTypes         []string `json:"event_types"`
}

type SetupChange struct {
	Parameter      string   `json:"parameter"`
	FromValue      *string  `json:"from_value"`
	ToValue        string   `json:"to_value"`
	Unit           string   `json:"unit"`
	HandlingEffect string   `json:"handling_effect"`
	Tradeoff       string   `json:"tradeoff"`
	SourceIDs      []string `json:"source_ids"`
	Experiment     string   `json:"experiment"`
}

type AgentInput struct {
	SchemaVersion  string      `json:"schema_version"`
	InvocationID   string      `json:"invocation_id"`
	ConversationID string      `json:"conversation_id"`
	Profile        string      `json:"profile"`
	EndpointID     string      `json:"endpoint_id"`
	Model          string      `json:"model"`
	Observation    Observation `json:"observation"`
	Continuity     Continuity  `json:"continuity"`
	RequestText    *string     `json:"request_text"`
	Limits         Limits      `json:"limits"`
}

type Limits struct {
	MaxSteps         int `json:"max_steps"`
	MaxToolCalls     int `json:"max_tool_calls"`
	DeadlineMs       int `json:"deadline_ms"`
	MaxOutputTokens  int `json:"max_output_tokens"`
	MaxContextTokens int `json:"max_context_tokens"`
}

type Continuity struct {
	PreviousAccepted         *AgentOutput `json:"previous_accepted"`
	ProviderRationaleSummary *string      `json:"provider_rationale_summary"`
	EvidenceCursor           *string      `json:"evidence_cursor"`
	LastAttempt              *Attempt     `json:"last_attempt"`
	MeasuredOutcomes         []string     `json:"measured_outcomes"`
	BranchReset              bool         `json:"branch_reset"`
}

type Attempt struct {
	InvocationID string  `json:"invocation_id"`
	Status       string  `json:"status"`
	EndedAtMs    int64   `json:"ended_at_ms"`
	ErrorCode    *string `json:"error_code"`
}

type ToolResult struct {
	Status      string     `json:"status"`
	Summary     string     `json:"summary"`
	NextActions []string   `json:"next_actions"`
	Artifacts   []Ref      `json:"artifacts"`
	Data        any        `json:"data"`
	Error       *ToolError `json:"error"`
}

type ToolError struct {
	Code          string `json:"code"`
	CauseHint     string `json:"cause_hint"`
	Retry         string `json:"retry"`
	StopCondition string `json:"stop_condition"`
}

type Ref struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type ExportRow struct {
	SchemaVersion    string   `json:"schema_version"`
	SessionID        string   `json:"session_id"`
	BranchID         string   `json:"branch_id"`
	FragmentID       string   `json:"fragment_id"`
	Seq              string   `json:"seq"`
	ReceivedAtMs     int64    `json:"received_at_ms"`
	SessionTimeMs    *int64   `json:"session_time_ms"`
	Lap              *int     `json:"lap"`
	CarIndex         *int     `json:"car_index"`
	Channel          string   `json:"channel"`
	ValueNumber      *float64 `json:"value_number"`
	ValueText        *string  `json:"value_text"`
	ValueBoolean     *bool    `json:"value_boolean"`
	Unit             string   `json:"unit"`
	Availability     string   `json:"availability"`
	Basis            string   `json:"basis"`
	SourceID         string   `json:"source_id"`
	Confidence       string   `json:"confidence"`
	Reason           *string  `json:"reason"`
	RegulationSeason *int     `json:"regulation_season"`
	GameModeCode     *int     `json:"game_mode_code"`
	Conditions       string   `json:"conditions"`
}

type ExportManifest struct {
	SchemaVersion   string            `json:"schema_version"`
	ExportID        string            `json:"export_id"`
	Format          string            `json:"format"`
	CreatedAtMs     int64             `json:"created_at_ms"`
	SessionID       string            `json:"session_id"`
	BranchID        string            `json:"branch_id"`
	Range           Range             `json:"range"`
	Cars            []int             `json:"cars"`
	Channels        []string          `json:"channels"`
	RowCount        int               `json:"row_count"`
	Units           map[string]string `json:"units"`
	Conditions      []string          `json:"conditions"`
	Gaps            []string          `json:"gaps"`
	SourceFiles     []string          `json:"source_files"`
	DecoderVersion  string            `json:"decoder_version"`
	MethodVersions  []string          `json:"method_versions"`
	SecretsIncluded bool              `json:"secrets_included"`
}

type Job struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	State            string    `json:"state"`
	ProgressFraction *float64  `json:"progress_fraction"`
	DownloadIDs      []string  `json:"download_ids"`
	Error            *JobError `json:"error"`
	CreatedAtMs      int64     `json:"created_at_ms"`
}

type JobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SeriesPoint struct {
	Seq           string      `json:"seq"`
	SessionTimeMs *int64      `json:"session_time_ms"`
	DistanceM     *float64    `json:"distance_m"`
	Measurement   Measurement `json:"measurement"`
}

type Series struct {
	LapID    *string       `json:"lap_id"`
	CarIndex *int          `json:"car_index"`
	Channel  string        `json:"channel"`
	Points   []SeriesPoint `json:"points"`
	ID       string        `json:"id"`
}

type AnalysisResult struct {
	Context     Context      `json:"context"`
	Series      []Series     `json:"series"`
	Laps        []LapSummary `json:"laps"`
	Comparable  bool         `json:"comparable"`
	Differences []string     `json:"differences"`
	Coverage    Coverage     `json:"coverage"`
}

type LapSummary struct {
	ID          string      `json:"id"`
	SessionID   string      `json:"session_id"`
	BranchID    string      `json:"branch_id"`
	CarIndex    int         `json:"car_index"`
	Lap         int         `json:"lap"`
	StartSeq    string      `json:"start_seq"`
	EndSeq      string      `json:"end_seq"`
	LapTimeMs   Measurement `json:"lap_time_ms"`
	DistanceM   Measurement `json:"distance_m"`
	Compound    Measurement `json:"compound"`
	Valid       bool        `json:"valid"`
	PitLap      bool        `json:"pit_lap"`
	Neutralized bool        `json:"neutralized"`
	Complete    bool        `json:"complete"`
	Gaps        []string    `json:"gaps"`
	Conditions  string      `json:"conditions"`
}

type StrategyQuery struct {
	SessionID      string        `json:"session_id"`
	BranchID       string        `json:"branch_id"`
	ObservationSeq string        `json:"observation_seq"`
	PitWindows     []PitWindowIn `json:"pit_windows"`
}

type PitWindowIn struct {
	FirstLap int    `json:"first_lap"`
	LastLap  int    `json:"last_lap"`
	Compound string `json:"compound"`
}

type StrategyCandidate struct {
	ID             string        `json:"id"`
	Stops          []PlannedStop `json:"stops"`
	TotalTimeMs    Measurement   `json:"total_time_ms"`
	PitLossMs      Measurement   `json:"pit_loss_ms"`
	RejoinPosition Measurement   `json:"rejoin_position"`
	ErrorBandMs    *float64      `json:"error_band_ms"`
	Legality       string        `json:"legality"`
	Reasons        []string      `json:"reasons"`
	Assumptions    []string      `json:"assumptions"`
	EvidenceIDs    []string      `json:"evidence_ids"`
	MethodVersion  string        `json:"method_version"`
}

type PlannedStop struct {
	Lap      int    `json:"lap"`
	Compound string `json:"compound"`
}

type StrategyEvaluation struct {
	Context        Context             `json:"context"`
	ObservationSeq string              `json:"observation_seq"`
	Candidates     []StrategyCandidate `json:"candidates"`
	Estimates      []Estimate          `json:"estimates"`
	Limitations    []string            `json:"limitations"`
}

type Channel struct {
	Name             string   `json:"name"`
	Unit             string   `json:"unit"`
	ValueType        string   `json:"value_type"`
	SourceFields     []string `json:"source_fields"`
	AvailabilityRule string   `json:"availability_rule"`
	FrequencyClass   string   `json:"frequency_class"`
}

type Health struct {
	InstanceID      string          `json:"instance_id"`
	ServerVersion   string          `json:"server_version"`
	SchemaVersion   string          `json:"schema_version"`
	State           string          `json:"state"`
	ActiveSessionID *string         `json:"active_session_id"`
	RecordingState  string          `json:"recording_state"`
	LastPacketAtMs  *int64          `json:"last_packet_at_ms"`
	Features        map[string]bool `json:"features"`
}

type Invocation struct {
	ID                      string       `json:"id"`
	Profile                 string       `json:"profile"`
	SessionID               string       `json:"session_id"`
	BranchID                string       `json:"branch_id"`
	Status                  string       `json:"status"`
	StartedAtMs             *int64       `json:"started_at_ms"`
	EndedAtMs               *int64       `json:"ended_at_ms"`
	TriggerIDs              []string     `json:"trigger_ids"`
	Output                  *AgentOutput `json:"output"`
	Usage                   Usage        `json:"usage"`
	NotificationDisposition string       `json:"notification_disposition"`
	Error                   *JobError    `json:"error"`
	PromptVersion           string       `json:"prompt_version"`
	ContinuationResetReason *string      `json:"continuation_reset_reason"`
}

type Usage struct {
	ProviderRequests int      `json:"provider_requests"`
	InputTokens      *int64   `json:"input_tokens"`
	OutputTokens     *int64   `json:"output_tokens"`
	TotalTokens      *int64   `json:"total_tokens"`
	CostUSD          *float64 `json:"cost_usd"`
	UsageEstimated   bool     `json:"usage_estimated"`
}

type Alert struct {
	ID                 string   `json:"id"`
	ConditionID        string   `json:"condition_id"`
	SessionID          string   `json:"session_id"`
	BranchID           string   `json:"branch_id"`
	Severity           string   `json:"severity"`
	State              string   `json:"state"`
	Summary            string   `json:"summary"`
	EvidenceIDs        []string `json:"evidence_ids"`
	FirstSeenMs        int64    `json:"first_seen_ms"`
	LastSeenMs         int64    `json:"last_seen_ms"`
	Acknowledged       bool     `json:"acknowledged"`
	ReferenceProfileID *string  `json:"reference_profile_id"`
}

type Source struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	URL              string   `json:"url"`
	RetrievedAtMs    int64    `json:"retrieved_at_ms"`
	GameBuild        *string  `json:"game_build"`
	RegulationSeason *int     `json:"regulation_season"`
	TrackID          *int     `json:"track_id"`
	Conditions       string   `json:"conditions"`
	Assists          string   `json:"assists"`
	UseCase          string   `json:"use_case"`
	Quality          string   `json:"quality"`
	Limitations      []string `json:"limitations"`
}

type SourceInspection struct {
	Source        Source  `json:"source"`
	Inspected     bool    `json:"inspected"`
	Text          string  `json:"text"`
	Truncated     bool    `json:"truncated"`
	ContentSHA256 *string `json:"content_sha256"`
}

type SearchResult struct {
	Sources   []Source     `json:"sources"`
	Leads     []SearchLead `json:"leads"`
	Truncated bool         `json:"truncated"`
}

type SearchLead struct {
	SourceID string `json:"source_id"`
	Snippet  string `json:"snippet"`
}

type SetupRecord struct {
	ID           string             `json:"id"`
	SessionID    string             `json:"session_id"`
	BranchID     string             `json:"branch_id"`
	InvocationID string             `json:"invocation_id"`
	Output       AgentOutput        `json:"output"`
	Sources      []SourceInspection `json:"sources"`
	Experiments  []Experiment       `json:"experiments"`
	CreatedAtMs  int64              `json:"created_at_ms"`
}

type Experiment struct {
	ID             string   `json:"id"`
	RecordID       string   `json:"record_id"`
	BaselineLapIDs []string `json:"baseline_lap_ids"`
	TrialLapIDs    []string `json:"trial_lap_ids"`
	Conditions     string   `json:"conditions"`
	Outcome        string   `json:"outcome"`
	CreatedAtMs    int64    `json:"created_at_ms"`
}

type SessionSummary struct {
	ID               string  `json:"id"`
	SourceContextID  string  `json:"source_context_id"`
	RegulationSeason *int    `json:"regulation_season"`
	ModeFamily       string  `json:"mode_family"`
	CareerID         *string `json:"career_id"`
	DisplayName      string  `json:"display_name"`
	TrackID          *int    `json:"track_id"`
	StartedAtMs      int64   `json:"started_at_ms"`
	EndedAtMs        *int64  `json:"ended_at_ms"`
	CompletedLaps    int     `json:"completed_laps"`
	RecordingState   string  `json:"recording_state"`
	Completeness     string  `json:"completeness"`
}

type SessionDetail struct {
	Summary           SessionSummary `json:"summary"`
	Context           Context        `json:"context"`
	Fragments         []Fragment     `json:"fragments"`
	Branches          []Branch       `json:"branches"`
	Recordings        []Recording    `json:"recordings"`
	SurvivingBranchID string         `json:"surviving_branch_id"`
}

type Fragment struct {
	ID              string  `json:"id"`
	SessionID       string  `json:"session_id"`
	SourceContextID string  `json:"source_context_id"`
	GameSessionUID  *string `json:"game_session_uid"`
	StartedAtMs     int64   `json:"started_at_ms"`
	EndedAtMs       *int64  `json:"ended_at_ms"`
	Range           Range   `json:"range"`
}

type Branch struct {
	ID               string  `json:"id"`
	SessionID        string  `json:"session_id"`
	ParentID         *string `json:"parent_id"`
	AnchorSeq        *string `json:"anchor_seq"`
	AnchorTimeMs     *int64  `json:"anchor_time_ms"`
	LastInheritedSeq *string `json:"last_inherited_seq"`
	IsSurviving      bool    `json:"is_surviving"`
}

type Recording struct {
	ID         string   `json:"id"`
	FragmentID string   `json:"fragment_id"`
	State      string   `json:"state"`
	Bytes      int64    `json:"bytes"`
	SHA256     *string  `json:"sha256"`
	Range      Range    `json:"range"`
	Gaps       []string `json:"gaps"`
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Details   any    `json:"details"`
}
