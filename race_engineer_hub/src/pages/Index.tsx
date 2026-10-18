import { useEffect, useState } from "react";
import { Drive } from "@/components/Drive";
import { Analysis, History, PitWall, Setups, Settings, nav } from "@/pages/Workspaces";

type Tab = "drive" | "pit" | "history" | "analysis" | "setups" | "settings";
const tabs: { id: Tab; label: string }[] = [
  { id: "drive", label: "Drive" },
  { id: "pit", label: "Pit wall" },
  { id: "history", label: "History" },
  { id: "analysis", label: "Analysis" },
  { id: "setups", label: "Setups" },
  { id: "settings", label: "Settings" },
];

export default function Index() {
  const initial = (): Tab => {
    const q = new URLSearchParams(window.location.search).get("workspace");
    return (tabs.find((t) => t.id === q)?.id ?? "drive") as Tab;
  };
  const [tab, setTab] = useState<Tab>(initial);
  useEffect(() => {
    document.title = "F1 Telemetry";
  }, []);
  useEffect(() => {
    const url = new URL(window.location.href);
    if (tab === "drive") url.searchParams.delete("workspace");
    else url.searchParams.set("workspace", tab);
    window.history.replaceState(null, "", url);
  }, [tab]);
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
      </nav>
      {tab === "drive" ? <Drive /> : null}
      {tab === "pit" ? <PitWall /> : null}
      {tab === "history" ? <History onOpen={() => setTab("pit")} /> : null}
      {tab === "analysis" ? <Analysis /> : null}
      {tab === "setups" ? <Setups /> : null}
      {tab === "settings" ? <Settings /> : null}
    </div>
  );
}
