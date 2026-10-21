import { useEffect, useRef } from "react";

export type AgentEvent = {
  seq: number;
  id: string;
  invocation_id: string;
  session_id?: string;
  branch_id?: string;
  profile?: string;
  kind: string;
  at_ms: number;
  duration_ms?: number | null;
  payload?: unknown;
  truncated?: boolean;
};

export type AgentLog = {
  id: string;
  session_id: string;
  branch_id: string;
  profile: string;
  status: string;
  trigger_ids: string[];
  model: string;
  endpoint_id: string;
  conversation_id: string;
  prompt_revision_id?: string;
  prompt_sha256?: string;
  started_at_ms?: number | null;
  ended_at_ms?: number | null;
  latency_ms?: number | null;
  compaction_count: number;
  notification_disposition: string;
  usage: {
    provider_requests: number;
    input_tokens?: number | null;
    output_tokens?: number | null;
    total_tokens?: number | null;
    cost_usd?: number | null;
    usage_estimated?: boolean;
  };
  decision?: string;
  driver_message?: string;
  error_code?: string;
  error_message?: string;
  event_count: number;
  last_kind?: string;
  last_at_ms?: number | null;
  output_json?: unknown;
};

/** Live activity feed. EventSource sends cookies and Last-Event-ID on reconnect. */
export function useAgentActivity(session: string, branch: string, onEvent: (e: AgentEvent) => void) {
  const handler = useRef(onEvent);
  handler.current = onEvent;
  useEffect(() => {
    const q = new URLSearchParams();
    if (session) q.set("session", session);
    if (branch) q.set("branch", branch);
    const es = new EventSource("/api/v1/engineer/activity/stream?" + q.toString());
    es.onmessage = (msg) => {
      try {
        handler.current(JSON.parse(msg.data) as AgentEvent);
      } catch {
        // Ignore a malformed event; the persisted log remains the source of truth.
      }
    };
    return () => es.close();
  }, [session, branch]);
}

export function eventStage(kind: string): string {
  switch (kind) {
    case "queued": return "Queued";
    case "context_built": return "Building context";
    case "compaction_started": return "Compacting context";
    case "compaction_completed": return "Context compacted";
    case "provider_request": return "Waiting for provider";
    case "provider_response": return "Provider responded";
    case "tool_call": return "Calling tool";
    case "tool_result": return "Tool returned";
    case "accepted": return "Accepted";
    case "rejected": return "Rejected";
    case "notification": return "Notifying";
    case "cancelled": return "Cancelled";
    case "interrupted": return "Interrupted";
    case "error": return "Error";
    default: return kind;
  }
}

export function runningKind(kind: string): boolean {
  return ["queued", "context_built", "compaction_started", "provider_request", "tool_call", "tool_result"].includes(kind);
}
