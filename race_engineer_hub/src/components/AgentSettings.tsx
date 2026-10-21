/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from "react";
import Editor, { DiffEditor, loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
import editorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import jsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";
import { api } from "@/lib/live";
import { useAuth } from "@/lib/auth";
import { useSelection } from "@/lib/selection";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

loader.config({ monaco });
(globalThis as any).MonacoEnvironment = {
  getWorker(_: string, label: string) {
    return label === "json" ? new jsonWorker() : new editorWorker();
  },
};

const panel = "border border-[#232830] bg-[#11141a]";
const label = "text-[11px] uppercase tracking-wider text-neutral-400";
const inputClass = "w-full border border-[#2a3038] bg-[#151920] px-2 py-1.5 text-sm text-neutral-100 outline-none focus-visible:border-[#5fd68f] disabled:opacity-50";

const eventTypes = ["pit_window_changed", "rival_pitted", "weather_changed", "safety_car", "virtual_safety_car", "red_flag", "tyre_risk", "fuel_risk", "energy_risk", "damage_changed", "race_control"];
const settingsTabs = ["agent", "prompt", "compaction", "connections", "security"] as const;
type SettingsTab = (typeof settingsTabs)[number];

export function AgentSettings() {
  const { logout } = useAuth();
  const [view, setView] = useState<any>(null);
  const [draft, setDraft] = useState<any>(null);
  const [tab, setTab] = useState<SettingsTab>("agent");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);

  async function loadConfig() {
    try {
      const v = await api<any>("/config");
      setView(v);
      setDraft(structuredClone(v.desired_config ?? v.active_config));
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }
  useEffect(() => { void loadConfig(); }, []);

  async function saveConfig() {
    if (!view || !draft) return;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const v = await api<any>("/config", {
        method: "PATCH",
        headers: { "If-Match": String(view.revision) },
        body: JSON.stringify({ config: draft, secrets: [], delete_secret_ids: [] }),
      });
      setView(v);
      setDraft(structuredClone(v.desired_config ?? v.active_config));
      setNotice("Saved. Live values apply immediately; restart-only values remain marked.");
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  }

  const source = (key: string) => view?.sources?.[key];
  const envLocked = (key: string) => source(key) === "env";
  const agent = draft?.agent;
  const endpoint = agent?.endpoints?.find((e: any) => e.id === agent.endpoint_id);

  function update(path: string, value: any) {
    setDraft((prev: any) => {
      const next = structuredClone(prev);
      const parts = path.split(".");
      let cur = next;
      for (let i = 0; i < parts.length - 1; i++) cur = cur[parts[i]];
      cur[parts[parts.length - 1]] = value;
      return next;
    });
  }

  const rule = agent?.rules?.[0];
  const lapChild = rule?.predicate?.children?.find((c: any) => c.kind === "lap");
  const eventChild = rule?.predicate?.children?.find((c: any) => c.kind === "event");

  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] p-4 text-neutral-100">
      <div className="mb-3 flex flex-wrap items-baseline gap-3">
        <h2 className="text-lg">Settings</h2>
        <span className="text-[12px] text-neutral-400">Revision {view?.revision ?? "—"} · agent and harness behavior</span>
        <button onClick={() => void saveConfig()} disabled={!draft || saving} className="ml-auto rounded-sm border border-[#2f5a3d] bg-[#1d3b2c] px-3 py-1.5 text-sm text-[#8ef0b4] disabled:opacity-40">{saving ? "Saving…" : "Save agent settings"}</button>
      </div>
      <Tabs value={tab} onValueChange={(v) => setTab(v as SettingsTab)}>
      <TabsList className="mb-3 flex h-auto flex-wrap justify-start gap-1 rounded-none bg-transparent p-0" aria-label="Settings sections">
        {settingsTabs.map((t) => (
          <TabsTrigger key={t} value={t} onClick={() => setTab(t)} className="rounded-sm px-3 py-1.5 text-sm text-neutral-300 hover:bg-[#151920] data-[state=active]:bg-[#1d3b2c] data-[state=active]:text-[#4fd48a]">{t[0].toUpperCase() + t.slice(1)}</TabsTrigger>
        ))}
      </TabsList>
      {error ? <p role="alert" className="mb-3 text-[#f25a5c]">{error}</p> : null}
      {notice ? <p role="status" className="mb-3 text-[#5fd68f]">{notice}</p> : null}
      {!draft || !view ? <p className="text-sm text-neutral-400">Loading configuration…</p> : null}

      <TabsContent value="agent" className="mt-0">{draft && view ? (
        <div className="flex flex-wrap gap-3">
          <Section title="Race engineer">
            <Check label="Enabled" checked={agent.enabled} disabled={envLocked("agent.enabled")} onChange={(v) => update("agent.enabled", v)} />
            <Row name="Endpoint" value={agent.endpoint_id ?? "not configured"} source={source("agent.endpoint_id")} />
            <Row name="Model" value={endpoint?.model ?? "not configured"} />
            <Row name="Protocol" value={endpoint?.protocol ?? "not configured"} />
            <Row name="API key" value={endpoint?.api_key_env ? (view.secrets_configured?.[endpoint.api_key_env] ? "Configured" : "Missing " + endpoint.api_key_env) : "not configured"} />
          </Section>
          <Section title="Schedule">
            <Number label="Lap cadence" value={lapChild?.every_completed_laps ?? 1} min={1} onChange={(v) => {
              const next = structuredClone(draft);
              const child = next.agent.rules[0].predicate.children.find((c: any) => c.kind === "lap");
              if (child) child.every_completed_laps = v;
              setDraft(next);
            }} />
            <Number label="Cooldown (s)" value={rule?.cooldown_seconds ?? 10} min={0} onChange={(v) => update("agent.rules.0.cooldown_seconds", v)} />
            <Number label="Minimum dispatch gap (s)" value={agent.minimum_dispatch_gap_seconds} min={0} onChange={(v) => update("agent.minimum_dispatch_gap_seconds", v)} />
            <div className="mt-2"><div className={label}>Events</div><div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[12px]">{eventTypes.map((ev) => <label key={ev} className="flex items-center gap-1"><input type="checkbox" checked={eventChild?.event_types?.includes(ev) ?? false} onChange={(e) => {
              const next = structuredClone(draft);
              const child = next.agent.rules[0].predicate.children.find((c: any) => c.kind === "event");
              if (!child) return;
              const set = new Set<string>(child.event_types ?? []);
              if (e.target.checked) set.add(ev); else set.delete(ev);
              child.event_types = [...set];
              setDraft(next);
            }} />{ev.replaceAll("_", " ")}</label>)}</div></div>
            <div className="mt-3"><div className={label}>Continuous mode</div><Check label="Enabled" checked={agent.continuous?.enabled ?? false} onChange={(v) => update("agent.continuous.enabled", v)} />
              <Number label="Cadence (s)" value={agent.continuous?.cadence_seconds ?? 10} min={1} onChange={(v) => update("agent.continuous.cadence_seconds", v)} />
              <Number label="Maximum minutes" value={agent.continuous?.max_minutes ?? 15} min={1} onChange={(v) => update("agent.continuous.max_minutes", v)} />
            </div>
          </Section>
          <Section title="Budgets and limits">
            <Number label="Calls per hour" value={agent.max_calls_per_hour} min={1} onChange={(v) => update("agent.max_calls_per_hour", v)} />
            <Number label="Tokens per hour" value={agent.max_tokens_per_hour} min={1} onChange={(v) => update("agent.max_tokens_per_hour", v)} />
            <Number label="Live deadline (ms)" value={agent.live_deadline_ms} min={1} onChange={(v) => update("agent.live_deadline_ms", v)} />
            <Number label="Analysis deadline (ms)" value={agent.analysis_deadline_ms} min={1} onChange={(v) => update("agent.analysis_deadline_ms", v)} />
            <Number label="Context tokens" value={agent.context_tokens} min={2048} onChange={(v) => update("agent.context_tokens", v)} />
            <Number label="Output tokens" value={agent.max_output_tokens} min={1} onChange={(v) => update("agent.max_output_tokens", v)} />
            <Number label="Maximum steps" value={agent.max_steps} min={1} onChange={(v) => update("agent.max_steps", v)} />
            <Number label="Maximum tool calls" value={agent.max_tool_calls} min={0} onChange={(v) => update("agent.max_tool_calls", v)} />
          </Section>
          <Section title="Sampling">
            <OptionalNumber label="Temperature" value={agent.generation?.temperature} onChange={(v) => update("agent.generation.temperature", v)} />
            <OptionalNumber label="Top-p" value={agent.generation?.top_p} onChange={(v) => update("agent.generation.top_p", v)} />
            <label className="mt-2 flex items-center gap-2 text-[12px]"><input type="checkbox" checked={agent.generation?.parallel_tool_calls ?? false} onChange={(e) => update("agent.generation.parallel_tool_calls", e.target.checked)} />Parallel tool calls <span className="text-neutral-500">(Responses protocol)</span></label>
          </Section>
          <Section title="Advanced rules JSON">
            <div className="h-[260px] overflow-hidden border border-[#232830]">
              <Editor height="260px" language="json" theme="vs-dark" value={JSON.stringify(agent.rules ?? [], null, 2)} onChange={(v) => { try { update("agent.rules", JSON.parse(v ?? "[]")); setError(""); } catch { setError("Rules JSON is invalid"); } }} options={{ minimap: { enabled: false }, fontSize: 12, scrollBeyondLastLine: false }} />
            </div>
          </Section>
        </div>
      ) : null}</TabsContent>

      <TabsContent value="prompt" className="mt-0"><PromptSettings /></TabsContent>
      <TabsContent value="compaction" className="mt-0">{draft ? <CompactionSettings view={view} draft={draft} update={update} /> : null}</TabsContent>
      <TabsContent value="connections" className="mt-0">{draft && view ? <Connections view={view} source={source} /> : null}</TabsContent>
      <TabsContent value="security" className="mt-0">{draft && view ? <Security view={view} logout={logout} /> : null}</TabsContent>
      </Tabs>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return <section className={panel + " min-w-0 flex-1 basis-[26rem] p-4"}><h3 className="mb-3 border-b border-[#232830] pb-2 text-[11px] uppercase tracking-wider text-neutral-400">{title}</h3><div className="space-y-2">{children}</div></section>;
}

function Row({ name, value, source, restart }: { name: string; value: string; source?: string; restart?: string }) {
  return <div className="flex items-baseline gap-2 border-b border-[#1c2128] py-1 text-sm last:border-0"><span className="w-40 shrink-0 text-neutral-400">{name}</span><span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{value}</span>{restart ? <span className={"rounded-sm border px-1 text-[10px] " + (restart === "Restart to apply" ? "border-[#5a4522] text-[#f0a03c]" : "border-[#2a3038] text-neutral-400")}>{restart}</span> : null}{source ? <span className="rounded-sm border border-[#2a3038] px-1 text-[10px] text-neutral-400">{source}</span> : null}</div>;
}

function Number({ label: text, value, min, onChange }: { label: string; value: number; min: number; onChange: (v: number) => void }) {
  return <label className="flex items-center gap-2 text-[12px]"><span className="w-48 text-neutral-400">{text}</span><input type="number" min={min} value={value} onChange={(e) => onChange(Number(e.target.value))} className={inputClass} /></label>;
}

function OptionalNumber({ label: text, value, onChange }: { label: string; value?: number | null; onChange: (v: number | null) => void }) {
  return <div className="flex items-center gap-2 text-[12px]"><span className="w-48 text-neutral-400">{text}</span><input type="number" step="0.05" value={value ?? ""} placeholder="provider default" onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))} className={inputClass} /><label className="flex items-center gap-1 text-neutral-500"><input type="checkbox" checked={value == null} onChange={(e) => { if (e.target.checked) onChange(null); }} />default</label></div>;
}

function Check({ label: text, checked, disabled, onChange }: { label: string; checked: boolean; disabled?: boolean; onChange: (v: boolean) => void }) {
  return <label className="flex items-center gap-2 text-[12px]"><input type="checkbox" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />{text}</label>;
}

function CompactionSettings({ view, draft, update }: { view: any; draft: any; update: (path: string, value: any) => void }) {
  const selection = useSelection();
  const [usage, setUsage] = useState<any>(null);
  const [focus, setFocus] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const c = draft.agent.compaction;
  async function load() {
    if (!selection.sessionId) { setUsage(null); return; }
    try { setUsage(await api<any>("/engineer/context?session=" + encodeURIComponent(selection.sessionId) + "&branch=" + encodeURIComponent(selection.branchId ?? ""))); } catch { setUsage(null); }
  }
  useEffect(() => { void load(); }, [selection.sessionId, selection.branchId]);
  async function compact() {
    setBusy(true); setError("");
    try {
      if (!selection.sessionId) { setError("Select a recorded session or start a live session first."); return; }
      await api<any>("/engineer/compact", { method: "POST", body: JSON.stringify({ session_id: selection.sessionId, branch_id: selection.branchId ?? "", focus }) });
      setFocus("");
      await load();
    } catch (e) { setError(String(e)); } finally { setBusy(false); }
  }
  return <div className="flex flex-wrap gap-3">
    <Section title="Compaction policy">
      <Check label="Enabled" checked={c.enabled} onChange={(v) => update("agent.compaction.enabled", v)} />
      <Number label="Trigger percent" value={c.trigger_percent} min={10} onChange={(v) => update("agent.compaction.trigger_percent", v)} />
      <Number label="Recent tail percent" value={c.recent_tail_percent} min={5} onChange={(v) => update("agent.compaction.recent_tail_percent", v)} />
      <p className="text-[11px] leading-relaxed text-neutral-500">Compaction preserves the plan, evidence IDs, assumptions, predictions, outcomes and accepted advice. Raw telemetry and repeated tool payloads are dropped first.</p>
    </Section>
    <Section title="Current context">
      {usage ? <><Row name="Conversation" value={usage.conversation_id ?? "—"} /><Row name="Messages" value={String(usage.messages ?? 0)} /><Row name="Token estimate" value={String(usage.token_estimate ?? 0)} /><Row name="Effective limit" value={String(usage.context_limit ?? "—")} />{usage.last_compaction ? <Row name="Last compaction" value={new Date(usage.last_compaction.created_at_ms).toLocaleString()} /> : null}</> : <p className="text-[12px] text-neutral-400">No active live conversation. Compaction is available once a session has messages.</p>}
      <label className="mt-3 block text-[11px] uppercase tracking-wider text-neutral-400">Focus (optional)<input value={focus} onChange={(e) => setFocus(e.target.value)} className={inputClass} placeholder="e.g. preserve the wet-tyre branch" /></label>
      <button onClick={() => void compact()} disabled={busy} className="mt-2 rounded-sm border border-[#2f5a3d] bg-[#1d3b2c] px-3 py-1.5 text-[12px] text-[#8ef0b4] disabled:opacity-40">{busy ? "Compacting…" : "Compact now"}</button>
      {error ? <p role="alert" className="mt-2 text-[12px] text-[#f25a5c]">{error}</p> : null}
    </Section>
  </div>;
}

function Connections({ view, source }: { view: any; source: (k: string) => string | undefined }) {
  const c = view.active_config;
  const restart = (key: string) => view.restart_required?.includes(key) ? "Restart to apply" : view.restart_only?.includes(key) ? "Restart only" : undefined;
  return <div className="flex flex-wrap gap-3">
    <Section title="Server"><Row name="Dashboard" value={c.http.host + ":" + c.http.port} source={source("http")} restart={restart("http")} /><Row name="Telemetry (UDP)" value={c.udp.host + ":" + c.udp.port} source={source("udp")} restart={restart("udp")} /><Row name="Game address" value={view.udp_advertise_host ? view.udp_advertise_host + ":" + c.udp.port : "unknown"} /><Row name="Data directory" value={c.data_dir ?? "default"} source={source("data_dir")} restart={restart("data_dir")} /></Section>
    <Section title="Recording"><Row name="Recording" value={c.recording.enabled ? "Enabled" : "Disabled"} source={source("recording.enabled")} restart={restart("recording")} /><Row name="Storage cap" value={gb(c.recording.cap_bytes)} source={source("recording.cap_bytes")} restart={restart("recording")} /><Row name="Free reserve" value={gb(c.recording.free_reserve_bytes)} source={source("recording.free_reserve_bytes")} restart={restart("recording")} /><Row name="Chunk size" value={(c.recording.chunk_bytes / 1048576).toFixed(0) + " MiB"} restart={restart("recording")} /><Row name="Rotation size" value={(c.recording.rotate_bytes / 1048576).toFixed(0) + " MiB"} restart={restart("recording")} /></Section>
    <Section title="Session"><Row name="Inactivity close" value={String(c.session?.inactivity_seconds ?? 600) + " s"} source={source("session.inactivity_seconds")} /></Section>
    <Section title="Search and voice"><Row name="Search provider" value={c.search.provider} source={source("search.provider")} /><Row name="Search URL" value={c.search.base_url || "provider default"} /><Row name="Voice" value={c.voice.enabled ? "Enabled" : "Disabled"} source={source("voice.enabled")} /><Row name="Voice URL" value={c.voice.transcription_endpoint || "not configured"} /></Section>
  </div>;
}

function Security({ view, logout }: { view: any; logout: () => Promise<void> }) {
  return <div className="flex flex-wrap gap-3">
    <Section title="Web security"><Row name="Origin validation" value={view.active_config.security?.origin_check ? "Enabled" : "Disabled"} source={view.sources?.["security.origin_check"]} /><Row name="Authentication" value="Password required" /><p className="text-[11px] leading-relaxed text-neutral-500">Origin validation, password reset and endpoint credentials live in the environment or native Windows Settings. The web page reports them without exposing secrets.</p><button onClick={() => void logout()} className="mt-2 w-fit rounded-sm border border-[#3a2a2a] bg-[#2a1a1a] px-3 py-1.5 text-[12px] text-[#f3c2c2]">Log out</button></Section>
  </div>;
}

function gb(n: number) { return (n / 1e9).toFixed(n % 1e9 === 0 ? 0 : 1) + " GB"; }

type PromptView = { profile: string; authored: string; body: string; current_revision_id: string | null; revisions: any[] };

function PromptSettings() {
  const [profiles, setProfiles] = useState<string[]>([]);
  const [profile, setProfile] = useState("scheduled_strategy");
  const [data, setData] = useState<PromptView | null>(null);
  const [body, setBody] = useState("");
  const [pending, setPending] = useState<{ mode: "save" | "select" | "reset"; id?: string } | null>(null);
  const [diff, setDiff] = useState<{ original: string; modified: string; mode: "save" | "select" | "reset"; id?: string } | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  async function load(p = profile) {
    try {
      const v = await api<PromptView>("/agent/prompts/" + p);
      setData(v); setBody(v.body); setError("");
    } catch (e) { setError(String(e)); }
  }
  useEffect(() => { api<{ items: PromptView[] }>("/agent/prompts").then((r) => setProfiles(r.items.map((i) => i.profile))).catch(() => null); }, []);
  useEffect(() => { void load(profile); }, [profile]);

  function review(mode: "save" | "select" | "reset", id?: string) {
    if (!data) return;
    setPending({ mode, id });
    setDiff({ original: data.body, modified: mode === "reset" ? data.authored : body, mode, id });
  }
  async function apply() {
    if (!diff) return;
    try {
      if (diff.mode === "save") await api("/agent/prompts/" + profile + "/revisions", { method: "POST", body: JSON.stringify({ body: diff.modified }) });
      if (diff.mode === "select") await api("/agent/prompts/" + profile + "/revisions/" + diff.id + "/select", { method: "POST" });
      if (diff.mode === "reset") await api("/agent/prompts/" + profile + "/reset", { method: "POST" });
      setDiff(null); setPending(null); setNotice(diff.mode === "reset" ? "Authored prompt restored." : diff.mode === "select" ? "Revision selected." : "Prompt revision saved."); await load();
    } catch (e) { setError(String(e)); }
  }
  async function remove(id: string) {
    try { await api("/agent/prompts/" + profile + "/revisions/" + id, { method: "DELETE" }); setConfirmDelete(null); await load(); setNotice("Revision deleted."); } catch (e) { setError(String(e)); }
  }
  return <div className="flex flex-wrap gap-3">
    <section className={panel + " min-w-0 flex-[2_1_42rem] p-4"}>
      <div className="flex flex-wrap items-center gap-2"><label className="text-[12px] text-neutral-400">Profile<select value={profile} onChange={(e) => setProfile(e.target.value)} className="ml-1 border border-[#2a3038] bg-[#151920] px-2 py-1">{profiles.map((p) => <option key={p}>{p}</option>)}</select></label>
        <span className="ml-auto text-[11px] text-neutral-400">{data?.current_revision_id ? "Custom revision" : "Authored default"}</span>
        <button onClick={() => review("reset")} className="rounded-sm bg-[#151920] px-2 py-1 text-[12px] text-neutral-300">Reset to authored</button>
        <button onClick={() => pending?.mode === "select" ? review("select", pending.id) : review("save")} disabled={!data || body === data.body} className="rounded-sm bg-[#1d3b2c] px-2 py-1 text-[12px] text-[#8ef0b4] disabled:opacity-40">{pending?.mode === "select" ? "Apply revision" : "Save revision"}</button>
      </div>
      {error ? <p role="alert" className="mt-2 text-[12px] text-[#f25a5c]">{error}</p> : null}
      {notice ? <p role="status" className="mt-2 text-[12px] text-[#5fd68f]">{notice}</p> : null}
      <div className="mt-3 h-[520px] overflow-hidden border border-[#232830]">
        <Editor height="520px" language="markdown" theme="vs-dark" value={body} onChange={(v) => { setBody(v ?? ""); setPending((p) => p?.mode === "select" ? null : p); }} options={{ minimap: { enabled: false }, fontSize: 12, wordWrap: "on", scrollBeyondLastLine: false }} />
      </div>
      <p className="mt-2 text-[11px] text-neutral-500">The output-schema instruction and tool set are appended automatically; do not add them here.</p>
    </section>
    <section className={panel + " min-w-0 flex-[1_1_18rem] p-4"}>
      <div className={label}>Revisions</div>
      <ul className="mt-2 divide-y divide-[#232830]">
        {data?.revisions?.length ? data.revisions.map((r) => <li key={r.id} className="py-2 text-[12px]">
          <button className="block w-full text-left" onClick={() => { setBody(r.body); setPending({ mode: "select", id: r.id }); setNotice("Revision loaded. Review the diff before applying."); }}>
            <div className="flex gap-2"><span className="font-mono text-[11px] text-neutral-400">{r.id}</span>{r.current ? <span className="text-[#5fd68f]">current</span> : null}</div>
            <div className="text-neutral-300">{new Date(r.created_at_ms).toLocaleString()}</div>
          </button>
          <div className="mt-1 flex items-center gap-2">
            <button onClick={() => { setBody(r.body); setPending({ mode: "select", id: r.id }); }} className="text-[11px] text-neutral-400 underline">Load</button>
            {confirmDelete === r.id ? <span className="flex items-center gap-1"><span className="text-[11px] text-[#f3c2c2]">Delete?</span><button onClick={() => void remove(r.id)} className="text-[11px] text-[#f25a5c] underline">Yes</button><button onClick={() => setConfirmDelete(null)} className="text-[11px] text-neutral-400 underline">No</button></span> : <button onClick={() => setConfirmDelete(r.id)} disabled={r.current} className="text-[11px] text-neutral-400 underline disabled:opacity-30">Delete</button>}
          </div>
        </li>) : <li className="py-3 text-[12px] text-neutral-400">No custom revisions yet.</li>}
      </ul>
    </section>
    {diff ? <Dialog open onOpenChange={(open) => { if (!open) setDiff(null); }}>
      <DialogContent className="flex h-[calc(100vh-2rem)] w-[calc(100vw-2rem)] max-w-6xl flex-col gap-0 border-[#2a3038] bg-[#0d1015] p-0 text-neutral-100">
        <div className="flex items-center gap-2 border-b border-[#232830] p-3 pr-12"><DialogTitle className="text-sm font-normal">Review {diff.mode === "reset" ? "reset to authored prompt" : diff.mode === "select" ? "revision selection" : "prompt revision"}</DialogTitle><DialogDescription className="text-[11px] text-neutral-400">left: current · right: proposed</DialogDescription><button onClick={() => setDiff(null)} className="ml-auto text-[12px] text-neutral-400 underline">Cancel</button><button onClick={() => void apply()} className="rounded-sm bg-[#1d3b2c] px-3 py-1 text-[12px] text-[#8ef0b4]">Confirm</button></div>
        <div className="min-h-0 flex-1"><DiffEditor height="70vh" language="markdown" theme="vs-dark" original={diff.original} modified={diff.modified} options={{ readOnly: true, minimap: { enabled: false }, fontSize: 12, wordWrap: "on", renderSideBySide: true, automaticLayout: true }} /></div>
      </DialogContent>
    </Dialog> : null}
  </div>;
}
