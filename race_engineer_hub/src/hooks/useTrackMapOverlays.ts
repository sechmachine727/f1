import { useCallback, useState } from "react";

export type OverlayKey = "corners" | "sectorBoundaries" | "sectorColors" | "marshalZones" | "marshalSectors" | "drs" | "startFinish";

export type OverlayState = Record<OverlayKey, boolean>;

const STORAGE_KEY = "trackMap:overlays";

const DEFAULTS: OverlayState = {
  corners: true,
  sectorBoundaries: false,
  sectorColors: false,
  marshalZones: false,
  marshalSectors: false,
  drs: true,
  startFinish: true,
};

function loadOverlays(): OverlayState {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) return { ...DEFAULTS, ...JSON.parse(stored) };
  } catch { /* ignore */ }
  return { ...DEFAULTS };
}

function saveOverlays(state: OverlayState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch { /* ignore */ }
}

export function useTrackMapOverlays() {
  const [overlays, setOverlays] = useState<OverlayState>(loadOverlays);

  const toggle = useCallback((key: OverlayKey) => {
    setOverlays((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      saveOverlays(next);
      return next;
    });
  }, []);

  return { overlays, toggle };
}
