package contract

type Query struct {
	SessionID         string   `json:"session_id"`
	BranchID          string   `json:"branch_id"`
	CarIndices        []int    `json:"car_indices"`
	Channels          []string `json:"channels"`
	FromSessionTimeMs int64    `json:"from_session_time_ms"`
	ToSessionTimeMs   int64    `json:"to_session_time_ms"`
	MaxPoints         int      `json:"max_points"`
	Alignment         string   `json:"alignment"`
}

type CompareQuery struct {
	SessionID string   `json:"session_id"`
	BranchID  string   `json:"branch_id"`
	LapIDs    []string `json:"lap_ids"`
	Channels  []string `json:"channels"`
	MaxPoints int      `json:"max_points"`
}

type ExportRequest struct {
	SessionID string   `json:"session_id"`
	BranchID  string   `json:"branch_id"`
	Range     Range    `json:"range"`
	Cars      []int    `json:"cars"`
	Channels  []string `json:"channels"`
	Format    string   `json:"format"`
}

type ReadWindowResult struct {
	Context   Context    `json:"context"`
	Window    Range      `json:"window"`
	Series    []Series   `json:"series"`
	Events    []Event    `json:"events"`
	Estimates []Estimate `json:"estimates"`
	Coverage  Coverage   `json:"coverage"`
}
