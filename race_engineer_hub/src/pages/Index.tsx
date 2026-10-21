import { useEffect, useState } from "react";
import { Drive } from "@/components/Drive";
import { Analysis, History, PitWall, Setups, Settings, nav } from "@/pages/Workspaces";
import { RecordedPitWall } from "@/components/RecordedPit";
import { RecordedAnalysis } from "@/components/RecordedAnalysis";
import { Logs } from "@/components/Logs";
import { useSelection } from "@/lib/selection";
import { useAuth } from "@/lib/auth";

type Tab = "drive" | "pit" | "history" | "analysis" | "setups" | "logs" | "settings";
const tabs: { id: Tab; label: string }[] = [
  { id: "drive", label: "Drive" },
  { id: "pit", label: "Pit wall" },
  { id: "history", label: "History" },
  { id: "analysis", label: "Analysis" },
  { id: "setups", label: "Setups" },
  { id: "logs", label: "Logs" },
  { id: "settings", label: "Settings" },
];

export default function Index() {
  const selection = useSelection();
  const { logout } = useAuth();
  const [logSession, setLogSession] = useState(() => new URLSearchParams(window.location.search).get("session") ?? "");
  const [logBranch, setLogBranch] = useState(() => new URLSearchParams(window.location.search).get("branch") ?? "");
  const initial = (): Tab => {
    const q = new URLSearchParams(window.location.search).get("workspace");
    return (tabs.find((t) => t.id === q)?.id ?? "drive") as Tab;
  };
  const [tab, setTab] = useState<Tab>(initial);
  const openLogs = (sessionId = "", branchId = "") => {
    setLogSession(sessionId);
    setLogBranch(branchId);
    setTab("logs");
  };
  useEffect(() => {
    document.title = "F1 Telemetry";
  }, []);
  useEffect(() => {
    const url = new URL(window.location.href);
    if (tab === "drive") url.searchParams.delete("workspace");
    else url.searchParams.set("workspace", tab);
    if (tab === "logs" && logSession) url.searchParams.set("session", logSession);
    else url.searchParams.delete("session");
    if (tab === "logs" && logBranch) url.searchParams.set("branch", logBranch);
    else url.searchParams.delete("branch");
    window.history.replaceState(null, "", url);
  }, [tab, logSession, logBranch]);
  return (
    <div className="min-h-screen bg-[#0b0d11]">
      <nav aria-label="Workspaces" className="flex gap-1 border-b border-[#232830] bg-[#0d1015] px-3 py-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            aria-current={tab === t.id ? "page" : undefined}
            className={nav + (tab === t.id ? " bg-[#1d3b2c] text-[#4fd48a]" : " text-neutral-300 hover:bg-[#151920]")}
          >
            {t.label}
          </button>
        ))}
        <button onClick={() => void logout()} className="ml-auto px-2 text-[12px] text-neutral-400 hover:text-neutral-200">Log out</button>
      </nav>
      {tab === "drive" ? <Drive /> : null}
      {tab === "pit" ? (selection.sessionId ? <RecordedPitWall onOpenLogs={openLogs} /> : <PitWall onOpenLogs={openLogs} />) : null}
      {tab === "history" ? <History onOpen={() => setTab("pit")} onOpenLogs={openLogs} /> : null}
      {tab === "analysis" ? (selection.sessionId ? <RecordedAnalysis /> : <Analysis />) : null}
      {tab === "setups" ? <Setups /> : null}
      {tab === "logs" ? <Logs initialSession={logSession} initialBranch={logBranch} onSessionChange={(s, b) => { setLogSession(s); setLogBranch(b); }} /> : null}
      {tab === "settings" ? <Settings /> : null}
    </div>
  );
}
