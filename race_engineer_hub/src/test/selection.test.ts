import { describe, expect, it } from "vitest";
import { clearSelection, select, toggleCarSelection } from "@/lib/selection";

describe("pit wall car selection", () => {
  it("adds a second car", () => {
    expect(toggleCarSelection([0], 3)).toEqual([0, 3]);
  });

  it("replaces the earliest selection when a third car is picked", () => {
    expect(toggleCarSelection([0, 3], 7)).toEqual([3, 7]);
    expect(toggleCarSelection([0, 3], 7)).toHaveLength(2);
  });

  it("keeps the table row selected state in step with the picks", () => {
    const selected = toggleCarSelection([0], 3);
    expect(selected.includes(0)).toBe(true);
    expect(selected.includes(3)).toBe(true);
    expect(selected.includes(5)).toBe(false);
  });

  it("deselects a car but never empties the selection", () => {
    expect(toggleCarSelection([0, 3], 3)).toEqual([0]);
    expect(toggleCarSelection([0], 0)).toEqual([0]);
  });

  it("keeps a recorded session across a reload", () => {
    select({ sessionId: "ses-1", branchId: "br-1", label: "Race", season: 2025, recorded: true });
    expect(JSON.parse(sessionStorage.getItem("f1tele.selection") ?? "{}")).toMatchObject({
      sessionId: "ses-1", branchId: "br-1", label: "Race", season: 2025, recorded: true,
    });
    clearSelection();
    expect(sessionStorage.getItem("f1tele.selection")).toBeNull();
  });
});

