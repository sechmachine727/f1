package engineer

import (
	"errors"
	"sort"

	"f1tele/server/internal/contract"
)

var errPromptTooLarge = errors.New("prompt and schema exceed the endpoint context limit")

const (
	maxMetrics      = 512
	maxRecentEvents = 64
	maxCritical     = 64
)

// BoundObservation trims an observation to the documented context budget. The
// selected car and its tyre/energy/fuel channels survive first; truncation is
// reported in coverage so a model is never told it saw everything.
func BoundObservation(obs contract.Observation, playerCar int) contract.Observation {
	if len(obs.Metrics) > maxMetrics {
		sort.SliceStable(obs.Metrics, func(i, j int) bool {
			return metricRank(obs.Metrics[i], playerCar) < metricRank(obs.Metrics[j], playerCar)
		})
		obs.Metrics = obs.Metrics[:maxMetrics]
		obs.Coverage.Truncated = true
	}
	if len(obs.Events) > maxRecentEvents+maxCritical {
		recent := obs.Events
		if len(recent) > maxRecentEvents {
			recent = recent[len(recent)-maxRecentEvents:]
		}
		var critical []contract.Event
		for _, e := range obs.Events {
			if e.Severity == "critical" {
				critical = append(critical, e)
			}
		}
		if len(critical) > maxCritical {
			critical = critical[len(critical)-maxCritical:]
		}
		seen := map[string]bool{}
		merged := make([]contract.Event, 0, len(recent)+len(critical))
		for _, e := range append(critical, recent...) {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			merged = append(merged, e)
		}
		obs.Coverage.OmittedEventCount = len(obs.Events) - len(merged)
		obs.Coverage.Truncated = true
		obs.Events = merged
	}
	if obs.Events == nil {
		obs.Events = []contract.Event{}
	}
	if obs.Metrics == nil {
		obs.Metrics = []contract.Metric{}
	}
	return obs
}

// metricRank orders metrics by how much a race engineer needs them: the
// selected car first, then observed values over unavailable ones.
func metricRank(m contract.Metric, player int) int {
	rank := 4
	if m.CarIndex != nil && *m.CarIndex == player {
		rank = 0
	} else if m.CarIndex == nil {
		rank = 1
	}
	if m.Measurement.Availability != "available" {
		rank += 2
	}
	return rank
}

// CheckContextFit reports whether prompts, schema and the bounded input can fit
// the endpoint limit. A prompt that cannot fit must fail the invocation rather
// than silently dropping the previous plan.
func CheckContextFit(promptTokens, contextLimit int) error {
	if promptTokens <= 0 || contextLimit <= 0 {
		return nil
	}
	if promptTokens > contextLimit {
		return errPromptTooLarge
	}
	return nil
}
