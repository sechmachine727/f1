/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/live";
import { type AgentEvent, type AgentLog, eventStage, runningKind, useAgentActivity } from "@/lib/activity";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";

export function AgentActivityDrawer({ session, branch, onOpenLogs }: { session: string; branch: string; onOpenLogs?: (session?: string, branch?: string) => void }) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<AgentLog[]>([]);
  const [live, setLive] = useState<Record<string, AgentEvent>>({});
  const [error, setError] = useState("");

  async function load() {
    if (!session) {
      setItems([]);
      return;
    }
    try {
      const q = new URLSearchParams({ session, limit: "12" });
      if (branch) q.set("branch", branch);
      const res = await api<{ items: AgentLog[] }>("/engineer/logs?" + q.toString());
      setItems(res.items ?? []);
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }

  useEffect(() => { void load(); }, [session, branch]);
  useAgentActivity(session, branch, (e) => {
    setLive((prev) => ({ ...prev, [e.invocation_id]: e }));
    if (["accepted", "rejected", "cancelled", "interrupted", "compaction_completed", "notification"].includes(e.kind)) void load();
  });
  const running = useMemo(() => Object.values(live).filter((e) => runningKind(e.kind)), [live]);
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <button
          disabled={!session}
          className="relative rounded-sm border border-[#2f5a3d] bg-[#1d3b2c] px-2.5 py-1 text-[12px] text-[#8ef0b4] disabled:opacity-40"
        >
          Agent activity
          {running.length ? <span className="ml-1.5 rounded-full bg-[#8ef0b4] px-1.5 text-[10px] font-semibold text-[#0b0d11]">{running.length}</span> : null}
        </button>
      </SheetTrigger>
      <SheetContent side="right" className="w-full max-w-[440px] overflow-auto border-l border-[#2a3038] bg-[#0d1015] p-4 text-neutral-100 sm:max-w-[440px]">
        <SheetHeader className="pr-6 text-left">
          <SheetTitle className="text-sm font-semibold text-neutral-100">Agent activity</SheetTitle>
          <SheetDescription className="text-[11px] text-neutral-400">Live stages for this session.</SheetDescription>
        </SheetHeader>
            {error ? <p role="alert" className="mt-3 text-[12px] text-[#f25a5c]">{error}</p> : null}
            {!session ? <p className="mt-4 text-[12px] text-neutral-400">No active session.</p> : null}
            {running.length ? <section className="mt-4 space-y-2">{running.map((e) => <LiveCard key={e.invocation_id} event={e} log={items.find((i) => i.id === e.invocation_id)} />)}</section> : null}
            <section className="mt-5">
              <div className="text-[11px] uppercase tracking-wider text-neutral-400">Recent</div>
              <ul className="mt-2 divide-y divide-[#232830] border border-[#232830]">
                {items.length === 0 ? <li className="p-4 text-[12px] text-neutral-400">No agent calls recorded for this session yet.</li> : null}
                {items.map((i) => (
                  <li key={i.id} className="p-2 text-[12px]">
                    <div className="flex gap-2"><span className={i.status === "accepted" ? "text-[#5fd68f]" : i.status === "running" ? "text-[#f0a03c]" : "text-neutral-400"}>{i.status}</span><span className="text-neutral-300">{i.profile.replaceAll("_", " ")}</span><span className="ml-auto text-neutral-500">{i.latency_ms != null ? i.latency_ms + " ms" : ""}</span></div>
                    {i.driver_message ? <div className="mt-0.5 text-neutral-200">{i.driver_message}</div> : null}
                    {i.error_message ? <div className="mt-0.5 text-[#f25a5c]">{i.error_message}</div> : null}
                  </li>
                ))}
              </ul>
            </section>
            <button onClick={() => { setOpen(false); onOpenLogs?.(session, branch); }} className="mt-4 w-full border border-[#2a3038] bg-[#151920] px-3 py-2 text-[12px] text-neutral-200">Open full logs</button>
      </SheetContent>
    </Sheet>
  );
}

function LiveCard({ event, log }: { event: AgentEvent; log?: AgentLog }) {
  const p = (event.payload ?? {}) as any;
  return <div className="border border-[#2a4a33] bg-[#16201a] p-3 text-[12px]">
    <div className="flex gap-2"><span className="font-semibold text-[#8ef0b4]">{eventStage(event.kind)}</span><span className="text-neutral-400">{event.profile?.replaceAll("_", " ")}</span></div>
    <div className="mt-1 text-neutral-300">{log?.driver_message || log?.decision || p.message || ""}</div>
    {p.model ? <div className="mt-1 text-[11px] text-neutral-400">Model {p.model}{p.input_tokens_estimate ? " · ~" + p.input_tokens_estimate + " tokens" : ""}</div> : null}
    {p.name ? <div className="mt-1 font-mono text-[11px] text-neutral-400">Tool {p.name}</div> : null}
    {p.trigger ? <div className="mt-1 text-[11px] text-neutral-400">Compaction: {p.trigger} · {p.before_tokens ?? "?"} → {p.after_tokens ?? "?"}</div> : null}
  </div>;
}
