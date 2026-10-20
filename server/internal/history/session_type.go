package history

// SessionType maps the game's session type code to a display class.
func SessionType(code int) string {
	switch {
	case code >= 1 && code <= 4:
		return "practice"
	case code == 5:
		return "qualifying_q1"
	case code == 6:
		return "qualifying_q2"
	case code == 7:
		return "qualifying_q3"
	case code >= 8 && code <= 9:
		return "qualifying"
	case code >= 10 && code <= 12:
		return "race"
	case code == 13:
		return "time_trial"
	case code == 14:
		return "sprint"
	case code == 15:
		return "time_trial"
	}
	return "unknown"
}

// SessionGroup collapses qualifying segments for display.
func SessionGroup(kind string) string {
	if kind == "qualifying_q1" || kind == "qualifying_q2" || kind == "qualifying_q3" || kind == "qualifying" {
		return "qualifying"
	}
	return kind
}
