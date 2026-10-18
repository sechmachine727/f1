package contract

type Career struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CareerCreate struct {
	Name string `json:"name"`
}

type CleanupRequest struct {
	RecordingIDs []string `json:"recording_ids"`
	SessionIDs   []string `json:"session_ids"`
	Confirmed    bool     `json:"confirmed"`
}

type CleanupResult struct {
	DeletedIDs []string         `json:"deleted_ids"`
	Failures   []CleanupFailure `json:"failures"`
}

type CleanupFailure struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Request struct {
	Profile        string  `json:"profile"`
	SessionID      string  `json:"session_id"`
	BranchID       string  `json:"branch_id"`
	ObservationSeq *string `json:"observation_seq"`
	RequestText    *string `json:"request_text"`
}

type SetupCreate struct {
	InvocationID string `json:"invocation_id"`
}

type ExperimentCreate struct {
	BaselineLapIDs []string `json:"baseline_lap_ids"`
	TrialLapIDs    []string `json:"trial_lap_ids"`
	Conditions     string   `json:"conditions"`
	Outcome        string   `json:"outcome"`
}

type Transcript struct {
	Text      string  `json:"text"`
	Language  *string `json:"language"`
	RequestID string  `json:"request_id"`
}

type SpeechRequest struct {
	InvocationID string `json:"invocation_id"`
}

type Ack struct {
	OK bool `json:"ok"`
}
