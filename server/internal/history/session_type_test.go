package history

import (
	"context"
	"path/filepath"
	"testing"

	"f1tele/server/internal/contract"
)

func TestSessionTypeClassification(t *testing.T) {
	cases := map[int]string{1: "practice", 4: "practice", 5: "qualifying_q1", 6: "qualifying_q2", 7: "qualifying_q3", 10: "race", 12: "race", 13: "time_trial", 14: "sprint", 99: "unknown"}
	for code, want := range cases {
		if got := SessionType(code); got != want {
			t.Fatalf("code %d = %q want %q", code, got, want)
		}
	}
	if SessionGroup("qualifying_q2") != "qualifying" || SessionGroup("practice") != "practice" {
		t.Fatal("qualifying segments must group while practice stays separate")
	}
}

func TestListSessionsGroupsQualifyingWeekend(t *testing.T) {
	cat, err := OpenCatalog(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "src", "ip"); err != nil {
		t.Fatal(err)
	}
	weekend, track := "77", 3
	for i, code := range []int{5, 6} {
		id := "q" + string(rune('1'+i))
		cx := contract.Context{SourceContextID: "src", SessionID: id, WeekendLinkID: &weekend, TrackID: &track, SessionTypeCode: &code, ModeFamily: "qualifying"}
		if err := cat.CreateSession(ctx, id, cx); err != nil {
			t.Fatal(err)
		}
	}
	items, _, err := cat.ListSessions(ctx, SessionFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].WeekendLinkID == nil || *items[0].WeekendLinkID != weekend {
		t.Fatalf("sessions %#v", items)
	}
	if items[0].SessionType != "qualifying_q2" && items[0].SessionType != "qualifying_q1" {
		t.Fatalf("session type %q", items[0].SessionType)
	}
}
