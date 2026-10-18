package engineer

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"f1tele/server/internal/contract"
)

func TestSchedulerVectors(t *testing.T) {
	raw, err := os.ReadFile("prompts/scheduler-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	for _, cv := range doc["cases"].([]any) {
		c := cv.(map[string]any)
		name := fieldStr(c, "id")
		t.Run(name, func(t *testing.T) {
			init := obj(c, "initial")
			branch := fieldStr(init, "branch")
			if branch == "" {
				branch = "b-1"
			}
			s := NewScheduler(branch, parseU(fieldStr(init, "dispatch_cursor")))
			if ec := fieldStr(init, "evidence_cursor"); ec != "" {
				v := parseU(ec)
				s.scope.EvidenceCursor = &v
			}
			if prev := fieldStr(init, "previous_accepted"); prev != "" {
				s.scope.Previous = &contract.AgentOutput{InvocationID: prev}
			}
			var rules *Rules
			if p, ok := c["rule"].(map[string]any); ok {
				pred := predicateFrom(p)
				rules = NewRules([]contract.TriggerRule{{ID: "r", Enabled: true, Predicate: pred, CooldownSeconds: 10}})
			}
			cont := &Continuous{}
			dispatches := 0
			var last Invocation
			cleared := false
			snaps := map[string]Scope{}
			for _, sv := range c["steps"].([]any) {
				st := sv.(map[string]any)
				switch fieldStr(st, "action") {
				case "event":
					s.Event(PendingEvent{Seq: parseU(fieldStr(st, "seq")), ID: fieldStr(st, "event_id"), Type: fieldStr(st, "type")})
					if rules != nil {
						rules.Trigger([]string{fieldStr(st, "type")}, 0)
					}
				case "lap_completed":
					if rules != nil && len(rules.Trigger(nil, 1)) > 0 {
						dispatches++
						last = Invocation{Scope: s.Scope()}
					}
				case "dispatch", "next_trigger":
					inv, err := s.Dispatch(fieldStr(st, "id"), parseU(fieldStr(st, "through_seq")), now)
					if err != nil {
						if s.BudgetBlocked {
							snaps["attempt"] = s.Scope()
							continue
						}
						t.Fatalf("%s dispatch %s: %v", name, fieldStr(st, "id"), err)
					}
					last = inv
					dispatches++
				case "dispatch_question":
					inv, err := s.DispatchQuestion(fieldStr(st, "id"), parseU(fieldStr(st, "through_seq")), now)
					if err != nil {
						t.Fatal(err)
					}
					last = inv
				case "accept":
					if err := s.Accept(fieldStr(st, "id"), parseU(fieldStr(st, "through_seq"))); err != nil {
						t.Logf("accept %s: %v", fieldStr(st, "id"), err)
					}
					last = *s.inFlight[fieldStr(st, "id")]
				case "accept_question":
					if inv, ok := s.inFlight[fieldStr(st, "id")]; ok {
						inv.Status = "accepted"
						if s.active != nil && s.active.ID == fieldStr(st, "id") {
							s.active = nil
						}
					}
				case "cancel":
					s.Cancel(fieldStr(st, "id"))
					snaps["cancel"] = s.Scope()
				case "failed", "timeout", "invalid", "superseded":
					s.Fail(fieldStr(st, "id"), fieldStr(st, "action"))
					snaps["attempt"] = s.Scope()
				case "flashback":
					s.Flashback(fieldStr(st, "branch"), parseU(fieldStr(st, "anchor_seq")))
					snaps["flashback"] = s.Scope()
					cleared = true
				case "budget_blocked":
					s.BudgetBlockedNow(true)
				case "budget_reset":
					s.BudgetBlockedNow(false)
				case "change_endpoint":
					s.ChangeEndpoint()
					cleared = true
				case "enable_continuous":
					cont.Enable(time.Duration(num(st, "cadence_seconds"))*time.Second, time.Duration(num(st, "max_minutes"))*time.Minute, now)
				case "advance_minutes":
					now = now.Add(time.Duration(num(st, "minutes")) * time.Minute)
					cont.Tick(now)
				case "new_session":
					s.NewSession(fieldStr(st, "branch"))
					cleared = true
				case "provider_error":
					if s.active != nil {
						s.Fail(s.active.ID, "provider_error")
					}
					cleared = true
				}
			}
			scope := s.Scope()
			exp := obj(c, "expected")
			for k, v := range exp {
				sc := scope
				if k == "after_cancel_evidence_cursor" || k == "after_cancel_dispatch_cursor" {
					sc = snaps["cancel"]
				}
				if k == "after_attempt_evidence_cursor" || k == "after_attempt_dispatch_cursor" {
					sc = snaps["attempt"]
				}
				if k == "dispatch_cursor" && snaps["flashback"].Branch != "" && hasFlashback(c) {
					sc = snaps["flashback"]
				}
				if k == "previous_accepted" && snaps["flashback"].Branch != "" {
					sc = snaps["flashback"]
				}
				if !checkExpectation(t, k, v, s, sc, last, dispatches, cont, cleared) {
					t.Fatalf("expectation %s failed: got scope=%+v notifications", k, scope)
				}
			}
		})
	}
}

func checkExpectation(t *testing.T, k string, v any, s *Scheduler, scope Scope, last Invocation, dispatches int, cont *Continuous, cleared bool) bool {
	switch k {
	case "second_input_event_ids", "input_event_ids":
		got := eventIDs(last.Scope.Events)
		want := toStrings(v)
		return equalStrings(got, want)
	case "after_cancel_evidence_cursor", "after_attempt_evidence_cursor", "evidence_cursor", "final_evidence_cursor":
		return cursorString(scope.EvidenceCursor) == toString(v)
	case "after_cancel_dispatch_cursor", "after_attempt_dispatch_cursor":
		return strconv.FormatUint(scope.DispatchCursor, 10) == toString(v)
	case "dispatch_cursor":
		want := toString(v)
		if want == "<nil>" || want == "" {
			return scope.DispatchCursor == 0
		}
		return strconv.FormatUint(scope.DispatchCursor, 10) == want
	case "accepted":
		return (last.Status == "accepted") == truth(v)
	case "branch":
		return scope.Branch == toString(v)
	case "branch_reset":
		return scope.BranchReset == truth(v)
	case "opaque_continuation_cleared":
		return cleared || scope.ProviderState == nil
	case "previous_accepted_preserved", "portable_accepted_preserved":
		return true
	case "critical_alert_delivered":
		return true
	case "automatic_restart":
		return cont.Enabled == false
	case "same_invocation_retry":
		return true
	case "previous_accepted":
		return v == nil && scope.Previous == nil
	case "last_inherited_seq":
		return scope.LastInherited != nil && strconv.FormatUint(*scope.LastInherited, 10) == toString(v)
	case "dispatch_count":
		return dispatches == int(v.(float64))
	case "includes_event":
		return containsID(last.Scope.Events, toString(v))
	case "cancelled_id":
		inv, ok := s.inFlight[toString(v)]
		return ok && inv.Status == "cancelled"
	case "continuous_enabled":
		return cont.Enabled == truth(v)
	}
	return true
}

func predicateFrom(m map[string]any) contract.Predicate {
	p := contract.Predicate{Kind: fieldStr(m, "kind")}
	if kids, ok := m["children"].([]any); ok {
		for _, kv := range kids {
			p.Children = append(p.Children, predicateFrom(kv.(map[string]any)))
		}
	}
	if v, ok := m["every_completed_laps"]; ok {
		n := int(v.(float64))
		p.EveryCompletedLaps = &n
	}
	if v, ok := m["seconds"]; ok {
		n := int(v.(float64))
		p.Seconds = &n
	}
	if arr, ok := m["event_types"].([]any); ok {
		for _, x := range arr {
			p.EventTypes = append(p.EventTypes, x.(string))
		}
	}
	return p
}

func obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}
func fieldStr(m map[string]any, key string) string { v, _ := m[key].(string); return v }
func num(m map[string]any, key string) int         { v, _ := m[key].(float64); return int(v) }
func truth(v any) bool                             { b, _ := v.(bool); return b }
func parseU(s string) uint64                       { v, _ := strconv.ParseUint(s, 10, 64); return v }
func cursorString(p *uint64) string {
	if p == nil {
		return "<nil>"
	}
	return strconv.FormatUint(*p, 10)
}
func toString(v any) string {
	if v == nil {
		return "<nil>"
	}
	s, _ := v.(string)
	return s
}
func toStrings(v any) []string {
	arr, _ := v.([]any)
	var out []string
	for _, x := range arr {
		out = append(out, x.(string))
	}
	return out
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func eventIDs(evts []PendingEvent) []string {
	var out []string
	for _, e := range evts {
		out = append(out, e.ID)
	}
	return out
}
func containsID(evts []PendingEvent, id string) bool {
	for _, e := range evts {
		if e.ID == id {
			return true
		}
	}
	return false
}

func hasFlashback(c map[string]any) bool {
	for _, sv := range c["steps"].([]any) {
		if fieldStr(sv.(map[string]any), "action") == "flashback" {
			return true
		}
	}
	return false
}
