import type { CircuitInfo } from "@/types/circuitInfo";
import circuitsJson from "./circuits.json";

const circuits = circuitsJson as Record<string, CircuitInfo>;

/** Look up static circuit info by F1 25 trackId. Returns null if not found. */
export function getCircuitInfo(trackId: number): CircuitInfo | null {
  return circuits[String(trackId)] ?? null;
}
