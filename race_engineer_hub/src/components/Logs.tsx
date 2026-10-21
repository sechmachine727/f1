/* eslint-disable @typescript-eslint/no-explicit-any */
import { Fragment, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/live";
import { type AgentEvent, type AgentLog, eventStage, useAgentActivity } from "@/lib/activity";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const panel = "border border-[#232830] bg-[#11141a]";
const tabClass = "rounded-sm px-2 py-1 text-[12px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#5fd68f]";

type Detail = { invocation: AgentLog; events: AgentEvent[]; compactions: any[]; messages: any[] };
const detailTabs = ["overview", "input", "output", "tools", "context", "raw"] as const;
type DetailTab = (typeof detailTabs)[number];

export function Logs({ initialSession = "", initialBranch = "", onSessionChange }: { initialSession?: string; initialBranch?: string; onSessionChange?: (session: string, branch: string) => void }) {
  const [session, setSession] = useState(initialSession);
  const [branch, setBranch] = useState(initialBranch);
  const [profile, setProfile] = useState("");
  const [status, setStatus] = useState("");
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [compactions, setCompactions] = useState(false);
  const [idFilter, setIdFilter] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [items, setItems] = useState<AgentLog[]>([]);
  const [next, setNext] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [detailTab, setDetailTab] = useState<DetailTab>("overview");
  const [follow, setFollow] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [live, setLive] = useState<Record<string, AgentEvent>>({});

  useEffect(() => {
    setSession(initialSession);
    setBranch(initialBranch);
  }, [initialSession, initialBranch]);

  function query(cursor = "") {
    const q = new URLSearchParams({ limit: "50" });
    if (session) q.set("session", session);
    if (branch) q.set("branch", branch);
    if (profile) q.set("profile", profile);
    if (status) q.set("status", status);
    if (errorsOnly) q.set("errors", "true");
    if (compactions) q.set("compactions", "true");
    if (idFilter.trim()) q.set("id", idFilter.trim());
    const fromMs = from ? new Date(from).getTime() : NaN;
    const toMs = to ? new Date(to).getTime() : NaN;
    if (Number.isFinite(fromMs)) q.set("from", String(fromMs));
    if (Number.isFinite(toMs)) q.set("to", String(toMs));
    if (cursor) q.set("cursor", cursor);
    return q.toString();
  }

  async function load(cursor = "") {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ items: AgentLog[]; next_cursor: string | null }>("/engineer/logs?" + query(cursor));
      setItems((old) => (cursor ? [...old, ...res.items] : res.items));
      setNext(res.next_cursor);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    onSessionChange?.(session, branch);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, branch, profile, status, errorsOnly, compactions, idFilter, from, to]);

  useEffect(() => {
    if (!follow) return;
    const t = setInterval(() => { void load(); }, 2500);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [follow, session, branch, profile, status, errorsOnly, compactions, idFilter, from, to]);

  useAgentActivity(session, branch, (e) => {
    setLive((prev) => ({ ...prev, [e.invocation_id]: e }));
    if (e.invocation_id === selected) void openDetail(e.invocation_id);
    if (e.kind === "compaction_completed" || e.kind === "accepted" || e.kind === "interrupted") void load();
  });

  async function openDetail(id: string) {
    setSelected(id);
    setDetailTab("overview");
    try {
      setDetail(await api<Detail>("/engineer/logs/" + encodeURIComponent(id)));
    } catch (e) {
      setError(String(e));
      setDetail(null);
    }
  }

  const activeLive = useMemo(() => Object.values(live).filter((e) => ["queued", "context_built", "compaction_started", "provider_request", "tool_call", "tool_result"].includes(e.kind)).slice(0, 8), [live]);
  const contextEvent = detail?.events.find((e) => e.kind === "context_built");
  const outputEvent = detail?.events.find((e) => e.kind === "accepted" || e.kind === "rejected");
  const output = outputEvent?.payload;
  const toolEvents = detail?.events.filter((e) => e.kind === "tool_call" || e.kind === "tool_result") ?? [];

  return (
    <div className="min-h-[calc(100vh-49px)] space-y-3 bg-[#0b0d11] p-4 text-neutral-100">
      <div className="flex flex-wrap items-end gap-3">
        <div><h2 className="text-lg font-semibold">Agent activity</h2><p className="text-[12px] text-neutral-400">Every invocation, tool call, compaction and decision the model saw.</p></div>
        <label className="ml-auto flex items-center gap-2 text-[12px] text-neutral-400"><input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} />Follow live</label>
      </div>
      <section className={panel + " flex flex-wrap items-end gap-2 p-3 text-[12px]"}>
        <label>Session<input value={session} onChange={(e) => setSession(e.target.value)} className="ml-1 w-40 border border-[#2a3038] bg-[#151920] px-2 py-1" /></label>
        <label>Branch<input value={branch} onChange={(e) => setBranch(e.target.value)} className="ml-1 w-32 border border-[#2a3038] bg-[#151920] px-2 py-1" /></label>
        <label>Profile<select value={profile} onChange={(e) => setProfile(e.target.value)} className="ml-1 border border-[#2a3038] bg-[#151920] px-2 py-1"><option value="">All</option>{["scheduled_strategy", "continuous_strategy", "live_question", "post_race", "setup_research", "compaction"].map((p) => <option key={p}>{p}</option>)}</select></label>
        <label>Status<select value={status} onChange={(e) => setStatus(e.target.value)} className="ml-1 border border-[#2a3038] bg-[#151920] px-2 py-1"><option value="">All</option>{["running", "accepted", "invalid", "failed", "cancelled", "superseded", "interrupted", "budget_blocked"].map((s) => <option key={s}>{s}</option>)}</select></label>
        <label>ID<input value={idFilter} onChange={(e) => setIdFilter(e.target.value)} className="ml-1 w-40 border border-[#2a3038] bg-[#151920] px-2 py-1" /></label>
        <label>From<input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className="ml-1 border border-[#2a3038] bg-[#151920] px-2 py-1" /></label>
        <label>To<input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className="ml-1 border border-[#2a3038] bg-[#151920] px-2 py-1" /></label>
        <label className="flex items-center gap-1"><input type="checkbox" checked={errorsOnly} onChange={(e) => setErrorsOnly(e.target.checked)} />Errors</label>
        <label className="flex items-center gap-1"><input type="checkbox" checked={compactions} onChange={(e) => setCompactions(e.target.checked)} />Compacted</label>
        <button onClick={() => void load()} className={tabClass + " ml-auto bg-[#1d3b2c] text-[#8ef0b4]"}>Refresh</button>
      </section>
      {activeLive.length ? <section className={panel + " p-3"}><div className="text-[11px] uppercase tracking-wider text-neutral-400">Live now</div><div className="mt-2 flex flex-wrap gap-2">{activeLive.map((e) => <button key={e.invocation_id + e.seq} onClick={() => void openDetail(e.invocation_id)} className="border border-[#2a4a33] bg-[#16201a] px-2 py-1 text-left text-[12px]"><span className="text-[#8ef0b4]">{eventStage(e.kind)}</span><span className="ml-2 text-neutral-400">{e.profile}</span></button>)}</div></section> : null}
      {error ? <p role="alert" className="text-[#f25a5c]">{error}</p> : null}
      <section className={panel + " overflow-auto"}>
        <table className="w-full min-w-[980px] text-[12px]">
          <thead className="sticky top-0 bg-[#151920] text-neutral-400"><tr>{["Time", "Session", "Profile", "Status", "Trigger", "Model", "Prompt", "In / Out", "Cost", "Latency", "Events", "Decision"].map((h) => <th key={h} className="p-2 text-left font-normal">{h}</th>)}</tr></thead>
          <tbody>
            {items.length === 0 ? <tr><td colSpan={12} className="p-6 text-center text-neutral-400">{loading ? "Loading activity…" : "No agent activity for this filter."}</td></tr> : null}
            {items.map((row) => (
              <tr key={row.id} role="button" tabIndex={0} aria-label={"Open invocation " + row.id} onClick={() => void openDetail(row.id)}
                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); void openDetail(row.id); } }}
                className={"cursor-pointer border-t border-[#232830] hover:bg-[#151920] focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[#5fd68f] " + (selected === row.id ? "bg-[#18211b]" : "")}>
                <td className="whitespace-nowrap p-2 tabular-nums">{time(row.started_at_ms)}</td>
                <td className="p-2 font-mono text-[11px]">{short(row.session_id)}</td>
                <td className="p-2">{row.profile.replaceAll("_", " ")}</td>
                <td className={"p-2 " + statusColor(row.status)}>{row.status}</td>
                <td className="p-2 text-neutral-400">{row.trigger_ids?.join(", ") || "—"}</td>
                <td className="p-2">{row.model || "—"}</td>
                <td className="p-2 text-neutral-400">{row.prompt_revision_id ? "custom" : "authored"}</td>
                <td className="p-2 tabular-nums">{row.usage.input_tokens ?? "—"} / {row.usage.output_tokens ?? "—"}</td>
                <td className="p-2 tabular-nums">{row.usage.cost_usd != null ? "$" + row.usage.cost_usd.toFixed(4) : "—"}</td>
                <td className="p-2 tabular-nums">{row.latency_ms != null ? row.latency_ms + " ms" : "—"}</td>
                <td className="p-2 tabular-nums">{row.event_count}{row.compaction_count ? " · " + row.compaction_count + " compact" : ""}</td>
                <td className="max-w-[220px] truncate p-2 text-neutral-300">{row.driver_message || row.decision || row.error_code || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {next ? <div className="p-3 text-center"><button onClick={() => void load(next)} className={tabClass + " bg-[#151920] text-neutral-300"}>Load older</button></div> : null}
      </section>

      {detail ? (
        <section className={panel + " p-4"}>
          <Tabs value={detailTab} onValueChange={(v) => setDetailTab(v as DetailTab)}>
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold">{detail.invocation.profile.replaceAll("_", " ")} · {detail.invocation.status}</h3>
            <span className="font-mono text-[11px] text-neutral-400">{detail.invocation.id}</span>
            <TabsList className="ml-auto flex h-auto flex-wrap justify-start gap-1 rounded-none bg-transparent p-0" aria-label="Invocation detail">
              {detailTabs.map((t) => <TabsTrigger key={t} value={t} onClick={() => setDetailTab(t)} className={tabClass + " text-neutral-300 data-[state=active]:bg-[#1d3b2c] data-[state=active]:text-[#8ef0b4]"}>{t[0].toUpperCase() + t.slice(1)}</TabsTrigger>)}
            </TabsList>
            <button onClick={() => download(detail)} className={tabClass + " bg-[#151920] text-neutral-300"}>Download</button>
          </div>
          <TabsContent value="overview" className="mt-3 text-[12px]"><Overview detail={detail} /></TabsContent>
          <TabsContent value="input" className="mt-3 text-[12px]"><InputView event={contextEvent} /></TabsContent>
          <TabsContent value="output" className="mt-3 text-[12px]">
            {output != null ? <pre className="max-h-[420px] overflow-auto whitespace-pre-wrap text-[11px] text-neutral-300">{JSON.stringify(output, null, 2)}</pre>
              : detail.invocation.output_json != null && detail.invocation.output_json !== "" ? <div><p className="mb-2 text-[11px] text-neutral-400">Legacy invocation: stored output, recorded before accepted and rejected events.</p><pre className="max-h-[420px] overflow-auto whitespace-pre-wrap text-[11px] text-neutral-300">{jsonText(detail.invocation.output_json)}</pre></div>
              : <pre className="max-h-[420px] overflow-auto whitespace-pre-wrap text-[11px] text-neutral-300">{JSON.stringify({ note: "No accepted or rejected output recorded." }, null, 2)}</pre>}
          </TabsContent>
          <TabsContent value="tools" className="mt-3 text-[12px]"><pre className="max-h-[420px] overflow-auto whitespace-pre-wrap text-[11px] text-neutral-300">{JSON.stringify(toolEvents, null, 2)}</pre></TabsContent>
          <TabsContent value="context" className="mt-3 text-[12px]"><ContextView detail={detail} /></TabsContent>
          <TabsContent value="raw" className="mt-3 text-[12px]"><pre className="max-h-[520px] overflow-auto whitespace-pre-wrap text-[11px] text-neutral-300">{JSON.stringify(detail, null, 2)}</pre></TabsContent>
          </Tabs>
        </section>
      ) : null}
    </div>
  );
}

function Overview({ detail }: { detail: Detail }) {
  const i = detail.invocation;
  const rows: [string, any][] = [
    ["Invocation", i.id], ["Conversation", i.conversation_id || "—"], ["Session", i.session_id], ["Branch", i.branch_id],
    ["Trigger", i.trigger_ids?.join(", ") || "—"], ["Endpoint", i.endpoint_id || "—"], ["Model", i.model || "—"],
    ["Prompt revision", i.prompt_revision_id || "authored default"], ["Prompt SHA-256", i.prompt_sha256 || "—"],
    ["Started", i.started_at_ms ? new Date(i.started_at_ms).toLocaleString() : "—"], ["Ended", i.ended_at_ms ? new Date(i.ended_at_ms).toLocaleString() : "—"],
    ["Latency", i.latency_ms != null ? i.latency_ms + " ms" : "—"], ["Requests", i.usage.provider_requests],
    ["Tokens", (i.usage.input_tokens ?? "—") + " in / " + (i.usage.output_tokens ?? "—") + " out"], ["Cost", i.usage.cost_usd != null ? "$" + i.usage.cost_usd.toFixed(4) : "not priced"],
    ["Compactions", i.compaction_count], ["Notification", i.notification_disposition], ["Decision", i.decision || "—"],
  ];
  return <dl className="grid grid-cols-[160px_minmax(0,1fr)] gap-x-3 gap-y-1">{rows.map(([k, v]) => <Fragment key={k}><dt className="text-neutral-400">{k}</dt><dd className="min-w-0 [overflow-wrap:anywhere]">{String(v)}</dd></Fragment>)}</dl>;
}

function InputView({ event }: { event?: AgentEvent }) {
  if (!event) return <p className="text-neutral-400">Input was not captured for this invocation. Older invocations show output and usage only.</p>;
  const p = (event.payload ?? {}) as any;
  return <div className="space-y-3">
    <div><div className="text-neutral-400">System prompt snapshot</div><pre className="mt-1 max-h-[260px] overflow-auto whitespace-pre-wrap border border-[#232830] bg-[#0b0d11] p-2 text-[11px]">{p.system_prompt ?? ""}</pre></div>
    <div><div className="text-neutral-400">Current input</div><pre className="mt-1 max-h-[320px] overflow-auto whitespace-pre-wrap border border-[#232830] bg-[#0b0d11] p-2 text-[11px]">{JSON.stringify(p.input ?? {}, null, 2)}</pre></div>
    <div><div className="text-neutral-400">Model-visible prior messages</div><pre className="mt-1 max-h-[320px] overflow-auto whitespace-pre-wrap border border-[#232830] bg-[#0b0d11] p-2 text-[11px]">{JSON.stringify(p.messages ?? [], null, 2)}</pre></div>
  </div>;
}

function ContextView({ detail }: { detail: Detail }) {
  const ctx = (detail.events.find((e) => e.kind === "context_built")?.payload ?? {}) as any;
  return <div className="space-y-3">
    <div className="flex flex-wrap gap-4"><span>Estimated tokens: <b>{ctx.token_estimate ?? "—"}</b></span><span>Effective limit: <b>{ctx.context_limit ?? "—"}</b></span><span>Messages: <b>{detail.messages.length}</b></span></div>
    <div><div className="text-neutral-400">Compactions</div><pre className="mt-1 max-h-[360px] overflow-auto whitespace-pre-wrap border border-[#232830] bg-[#0b0d11] p-2 text-[11px]">{JSON.stringify(detail.compactions, null, 2)}</pre></div>
    <div><div className="text-neutral-400">Compaction events</div><pre className="mt-1 max-h-[260px] overflow-auto whitespace-pre-wrap border border-[#232830] bg-[#0b0d11] p-2 text-[11px]">{JSON.stringify(detail.events.filter((e) => e.kind.startsWith("compaction")), null, 2)}</pre></div>
  </div>;
}

function time(ms?: number | null) {
  return ms ? new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "—";
}
function short(id: string) { return id ? id.slice(0, 8) : "—"; }
function statusColor(status: string) {
  if (status === "accepted") return "text-[#5fd68f]";
  if (status === "running") return "text-[#f0a03c]";
  if (status === "invalid" || status === "failed" || status === "interrupted") return "text-[#f25a5c]";
  return "text-neutral-400";
}
function download(detail: Detail) {
  const blob = new Blob([JSON.stringify(detail, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "agent-" + detail.invocation.id + ".json";
  a.click();
  URL.revokeObjectURL(url);
}
function jsonText(value: unknown) {
  if (typeof value !== "string") return JSON.stringify(value, null, 2);
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
}
