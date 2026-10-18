/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from "react";
import { api, metric, text, useLive } from "@/lib/live";

type Invocation = {
  id: string; profile: string; status: string; notification_disposition: string;
  output: { driver_message: string | null; decision: string; urgency: string } | null;
  error: { code: string; message: string } | null;
};

/** Live engineer requests, recent invocations and alert acknowledgment. */
export function EngineerCard() {
  const s = useLive();
  const [draft, setDraft] = useState("");
  const [items, setItems] = useState<Invocation[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!s.context) return;
    const load = () =>
      api<any>("/engineer/invocations?limit=5")
        .then((d) => setItems(d.items ?? []))
        .catch(() => null);
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, [s.context?.session_id]);

  async function ask() {
    if (!s.context || !draft.trim()) return;
    setBusy(true);
    setError("");
    try {
      await api<any>("/engineer/requests", {
        method: "POST",
        body: JSON.stringify({
          profile: "live_question",
          session_id: s.context.session_id,
          branch_id: s.context.branch_id,
          observation_seq: String(s.seq),
          request_text: draft.trim(),
        }),
      });
      setDraft("");
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function acknowledge(id: string) {
    const key = id.startsWith("alert-") ? id.slice(6) : id;
    await api<any>("/alerts/" + encodeURIComponent(key) + "/acknowledge", { method: "POST" }).catch((e) => setError(String(e)));
  }

  const active = (s.alerts ?? []).filter((a) => a.state === "active");

  return (
    <section className="border border-[#232830] bg-[#11141a] p-4 space-y-3">
      <div className="flex items-baseline justify-between">
        <div className="text-[11px] uppercase tracking-wider text-neutral-500">Race engineer</div>
        <span className="text-[11px] text-neutral-500">
          {text(metric(s, "car.speed_mps")) === "—" ? "Waiting for telemetry" : "Live"}
        </span>
      </div>

      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void ask();
        }}
      >
        <label className="sr-only" htmlFor="engineer-question">Question</label>
        <input
          id="engineer-question"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Ask a question"
          maxLength={4000}
          className="min-w-0 flex-1 border border-[#2a3038] bg-[#151920] px-2 py-1.5 text-sm outline-none focus-visible:border-[#5fd68f]"
        />
        <button type="submit" disabled={busy || !s.context || !draft.trim()}
          className="border border-[#2f5a3d] bg-[#1d3b2c] px-3 py-1.5 text-sm text-[#8ef0b4] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#5fd68f] disabled:opacity-40">
          {busy ? "Sending…" : "Ask"}
        </button>
      </form>
      {error ? <p role="alert" className="text-[12px] text-[#f25a5c]">{error}</p> : null}
      {!s.context ? <p className="text-[12px] text-neutral-500">No active session yet.</p> : null}

      {active.length ? (
        <ul className="space-y-1">
          {active.map((a) => (
            <li key={a.id} className="flex items-start gap-2 text-[13px]">
              <span className={a.severity === "critical" ? "text-[#f25a5c]" : "text-[#f0a03c]"}>
                {a.severity === "critical" ? "Critical" : "Caution"}
              </span>
              <span className="min-w-0 flex-1 text-neutral-200">{a.summary}</span>
              <button className="text-[12px] text-neutral-400 underline" onClick={() => acknowledge(a.id)}>
                Acknowledge
              </button>
            </li>
          ))}
        </ul>
      ) : null}

      {items.length ? (
        <ul className="divide-y divide-[#232830] text-[12px]">
          {items.map((i) => (
            <li key={i.id} className="py-1.5">
              <span className="text-neutral-500">{i.profile.replace("_", " ")} · {i.status}</span>
              {i.output?.driver_message ? <div className="text-neutral-200">{i.output.driver_message}</div> : null}
              {i.error ? <div className="text-[#f25a5c]">{i.error.code}: {i.error.message}</div> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
