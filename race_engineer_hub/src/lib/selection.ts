import { useEffect, useState } from "react";

// One selected historical session drives History, Pit wall and Analysis. Live
// data keeps its own source; a selection never replaces it.
export type Selection = {
  sessionId: string | null;
  branchId: string | null;
  label: string;
  season: number | null;
  recorded: boolean;
};

const empty: Selection = { sessionId: null, branchId: null, label: "", season: null, recorded: false };
// shortcut: selection follows this tab, not the URL; move it into the query string if shareable links are needed.
const storedKey = "f1tele.selection";

function stored(): Selection {
  try {
    const raw = window.sessionStorage.getItem(storedKey);
    if (!raw) return empty;
    const v = JSON.parse(raw) as Partial<Selection>;
    return {
      sessionId: typeof v.sessionId === "string" ? v.sessionId : null,
      branchId: typeof v.branchId === "string" ? v.branchId : null,
      label: typeof v.label === "string" ? v.label : "",
      season: typeof v.season === "number" ? v.season : null,
      recorded: v.recorded === true,
    };
  } catch {
    return empty;
  }
}

let current = typeof window === "undefined" ? empty : stored();
const listeners = new Set<() => void>();

export function select(next: Selection) {
  current = next;
  try { window.sessionStorage.setItem(storedKey, JSON.stringify(next)); } catch { /* storage disabled */ }
  listeners.forEach((l) => l());
}
export function clearSelection() {
  current = empty;
  try { window.sessionStorage.removeItem(storedKey); } catch { /* storage disabled */ }
  listeners.forEach((l) => l());
}
export function useSelection(): Selection {
  const [, force] = useState(0);
  useEffect(() => {
    const l = () => force((n) => n + 1);
    listeners.add(l);
    return () => {
      listeners.delete(l);
    };
  }, []);
  return current;
}

/** At most two cars: a third pick replaces the earliest selection. */
export function toggleCarSelection(selected: number[], car: number): number[] {
  if (selected.includes(car)) return selected.length > 1 ? selected.filter((c) => c !== car) : selected;
  return [...selected.slice(-1), car];
}
