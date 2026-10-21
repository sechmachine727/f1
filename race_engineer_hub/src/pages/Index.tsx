import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";
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
  const location = useLocation();
  const navigate = useNavigate();
  const params = new URLSearchParams(location.search);
  const tab = (tabs.find((t) => t.id === params.get("workspace"))?.id ?? "drive") as Tab;
  const logSession = params.get("session") ?? "";
  const logBranch = params.get("branch") ?? "";
  const go = (next: Tab) => {
    if (next === tab) return;
    const q = new URLSearchParams();
    if (next !== "drive") q.set("workspace", next);
    navigate({ pathname: "/", search: q.toString() });
  };
  const openLogs = (sessionId = "", branchId = "") => {
    const q = new URLSearchParams({ workspace: "logs" });
    if (sessionId) q.set("session", sessionId);
    if (branchId) q.set("branch", branchId);
    navigate({ pathname: "/", search: q.toString() });
  };
  const setLogFilter = (sessionId: string, branchId: string) => {
    const q = new URLSearchParams(location.search);
    if (sessionId) q.set("session", sessionId); else q.delete("session");
    if (branchId) q.set("branch", branchId); else q.delete("branch");
    navigate({ pathname: "/", search: q.toString() }, { replace: true });
  };
  useEffect(() => {
    document.title = "F1 Telemetry";
  }, []);
  return (
    <div className="min-h-screen bg-[#0b0d11]">
      <nav aria-label="Workspaces" className="flex gap-1 border-b border-[#232830] bg-[#0d1015] px-3 py-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => go(t.id)}
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
      {tab === "history" ? <History onOpen={() => go("pit")} onOpenLogs={openLogs} /> : null}
      {tab === "analysis" ? (selection.sessionId ? <RecordedAnalysis /> : <Analysis />) : null}
      {tab === "setups" ? <Setups /> : null}
      {tab === "logs" ? <Logs initialSession={logSession} initialBranch={logBranch} onSessionChange={setLogFilter} /> : null}
      {tab === "settings" ? <Settings /> : null}
    </div>
  );
}
