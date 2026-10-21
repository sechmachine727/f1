import { describe, expect, it } from "vitest";
import { eventStage, runningKind } from "@/lib/activity";

describe("agent activity labels", () => {
  it("labels live and terminal stages", () => {
    expect(eventStage("provider_request")).toBe("Waiting for provider");
    expect(eventStage("compaction_completed")).toBe("Context compacted");
    expect(eventStage("accepted")).toBe("Accepted");
    expect(eventStage("interrupted")).toBe("Interrupted");
  });

  it("counts only in-flight stages as running", () => {
    expect(runningKind("queued")).toBe(true);
    expect(runningKind("tool_call")).toBe(true);
    expect(runningKind("accepted")).toBe(false);
    expect(runningKind("error")).toBe(false);
  });
});
