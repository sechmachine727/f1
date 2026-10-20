import { describe, expect, it } from "vitest";
import { sessionMeta } from "@/lib/recorded";

describe("history row metadata", () => {
  it("names a mapped circuit with date and duration", () => {
    const meta = sessionMeta({ track_id: 3, started_at_ms: Date.UTC(2026, 2, 22, 18, 15), ended_at_ms: Date.UTC(2026, 2, 22, 19, 45) });
    expect(meta.circuit).toBe("Sakhir (Bahrain)");
    expect(meta.date).toContain("2026");
    expect(meta.duration).toBe("90 min");
  });

  it("keeps an unmapped identifier instead of dropping the session", () => {
    const meta = sessionMeta({ track_id: 99, started_at_ms: 0, ended_at_ms: null });
    expect(meta.circuit).toBe("Track 99 (name unknown)");
    expect(meta.date).toBe("date unknown");
    expect(meta.duration).toBe("in progress");
  });

  it("states an unknown track when the id is absent", () => {
    expect(sessionMeta({ track_id: null, started_at_ms: 1, ended_at_ms: 2 }).circuit).toBe("Track unknown");
  });
});

