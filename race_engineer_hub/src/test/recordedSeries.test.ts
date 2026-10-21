import { describe, expect, it } from "vitest";
import { seriesKeys } from "@/lib/recorded";

describe("recorded chart series", () => {
  it("unions series keys across rows so a later-starting driver is kept", () => {
    expect(seriesKeys([
      { lap: 1, LECLERC: 90 },
      { lap: 2, LECLERC: 91, HAMILTON: 92 },
    ])).toEqual(["LECLERC", "HAMILTON"]);
    expect(seriesKeys([])).toEqual([]);
  });
});
