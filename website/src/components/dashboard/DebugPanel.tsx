"use client";

import { useMemo } from "react";
import type { RuntimeEvent, RuntimeSnapshot } from "../../lib/runtime-client";

function safeStringify(v: unknown) {
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return "";
  }
}

function latestByType(events: RuntimeEvent[], types: string[]) {
  const wanted = new Set(types.map((t) => t.trim().toUpperCase()));
  let best: RuntimeEvent | null = null;
  for (const ev of events) {
    if (!wanted.has(ev.type.trim().toUpperCase())) continue;
    if (!best || ev.timestamp > best.timestamp) best = ev;
  }
  return best;
}

export function DebugPanel({ snapshot, events }: { snapshot: RuntimeSnapshot | null; events: RuntimeEvent[] }) {
  const lastDecision = useMemo(() => latestByType(events, ["ORCHESTRATOR_DECISION", "POLICY_DECISION"]), [events]);
  const lastFailure = useMemo(() => latestByType(events, ["CONNECTION_FAIL", "SINGBOX_START_FAILED", "PROBE_FAILED"]), [events]);

  return (
    <div className="rounded-xl border border-border bg-card/60 p-4">
      <div className="text-xs text-muted-foreground uppercase tracking-wider">Debug Panel</div>
      <div className="mt-3 grid md:grid-cols-3 gap-4">
        <div className="rounded-lg border border-border bg-background/40 p-3">
          <div className="text-xs text-muted-foreground uppercase tracking-wider">Last Decision</div>
          <div className="mt-2 text-xs font-mono text-foreground">{lastDecision?.type ?? "—"}</div>
          <div className="mt-2 text-xs font-mono text-muted-foreground break-words">{lastDecision?.message ?? ""}</div>
          {lastDecision?.metadata ? (
            <pre className="mt-2 text-[11px] leading-4 font-mono text-muted-foreground whitespace-pre-wrap break-words">
              {safeStringify(lastDecision.metadata)}
            </pre>
          ) : null}
        </div>
        <div className="rounded-lg border border-border bg-background/40 p-3">
          <div className="text-xs text-muted-foreground uppercase tracking-wider">Last Error</div>
          <div className="mt-2 text-xs font-mono text-foreground">{lastFailure?.type ?? "—"}</div>
          <div className="mt-2 text-xs font-mono text-muted-foreground break-words">{lastFailure?.message ?? ""}</div>
          {lastFailure?.metadata ? (
            <pre className="mt-2 text-[11px] leading-4 font-mono text-muted-foreground whitespace-pre-wrap break-words">
              {safeStringify(lastFailure.metadata)}
            </pre>
          ) : null}
        </div>
        <div className="rounded-lg border border-border bg-background/40 p-3">
          <div className="text-xs text-muted-foreground uppercase tracking-wider">Snapshot</div>
          {snapshot ? (
            <pre className="mt-2 text-[11px] leading-4 font-mono text-muted-foreground whitespace-pre-wrap break-words">
              {safeStringify(snapshot)}
            </pre>
          ) : (
            <div className="mt-2 text-xs font-mono text-muted-foreground">—</div>
          )}
        </div>
      </div>
    </div>
  );
}

