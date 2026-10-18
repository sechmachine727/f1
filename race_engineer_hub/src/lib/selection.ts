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
let current = empty;
const listeners = new Set<() => void>();

export function select(next: Selection) {
  current = next;
  listeners.forEach((l) => l());
}
export function clearSelection() {
  current = empty;
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
