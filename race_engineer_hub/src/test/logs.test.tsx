import { describe, expect, it, vi, afterEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Logs } from "@/components/Logs";

const log = {
  id: "i1", session_id: "s", branch_id: "b", profile: "post_race", status: "accepted", trigger_ids: [],
  model: "", endpoint_id: "", conversation_id: "", compaction_count: 0, notification_disposition: "",
  usage: { provider_requests: 1 }, event_count: 0,
};

afterEach(() => { vi.unstubAllGlobals(); });

describe("agent logs", () => {
  it("renders stored output for a legacy invocation without an outcome event", async () => {
    vi.stubGlobal("EventSource", class { onmessage = null; close() {} });
    vi.stubGlobal("fetch", vi.fn(async (url: unknown) => {
      const body = String(url).endsWith("/engineer/logs/i1")
        ? { invocation: { ...log, output_json: { decision: "hold position", driver_message: "Box this lap" } }, events: [], compactions: [], messages: [] }
        : { items: [log], next_cursor: null };
      return { ok: true, status: 200, json: async () => body } as Response;
    }));

    render(<Logs />);
    fireEvent.click(await screen.findByRole("button", { name: "Open invocation i1" }));
    fireEvent.click(await screen.findByRole("tab", { name: "Output" }));

    expect(screen.getByText(/Legacy invocation: stored output/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(/hold position/)).toBeInTheDocument());
  });
});
