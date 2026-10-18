package contract

type Replay struct {
	ID                    string  `json:"id"`
	SessionID             string  `json:"session_id"`
	BranchID              string  `json:"branch_id"`
	SourceID              string  `json:"source_id"`
	Speed                 float64 `json:"speed"`
	Paused                bool    `json:"paused"`
	PlayheadSessionTimeMs int     `json:"playhead_session_time_ms"`
	SkipLongPauses        bool    `json:"skip_long_pauses"`
}

type ReplayCreate struct {
	SessionID      string  `json:"session_id"`
	BranchID       string  `json:"branch_id"`
	Speed          float64 `json:"speed"`
	SkipLongPauses bool    `json:"skip_long_pauses"`
}

type ReplayPatch struct {
	PositionMs *int     `json:"position_ms"`
	Speed      *float64 `json:"speed"`
	Paused     *bool    `json:"paused"`
}
