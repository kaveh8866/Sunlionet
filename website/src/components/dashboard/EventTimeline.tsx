"use client";

import { useMemo } from "react";
import type { RuntimeEvent } from "../../lib/runtime-client";
import { formatUnixAgo } from "./format";

function typeClass(type: string) {
  const t = type.trim().toUpperCase();
  if (t.includes("FAIL") || t.includes("CRASH") || t.includes("ERROR")) return "text-red-300";
  if (t.includes("SUCCESS") || t.includes("OK") || t.includes("CONNECTED")) return "text-emerald-300";
  if (t.includes("DECISION") || t.includes("SWITCH")) return "text-amber-300";
  if (t.includes("CONFIG") || t.includes("SINGBOX")) return "text-sky-300";
  return "text-foreground";
}

function safeStringify(v: unknown) {
  try {
    return JSON.stringify(v);
  } catch {
    return "";
  }
}

export function EventTimeline({ events, limit = 100 }: { events: RuntimeEvent[]; limit?: number }) {
  const recent = useMemo(() => {
    const list = [...events].sort((a, b) => b.timestamp - a.timestamp);
    return list.slice(0, limit);
  }, [events, limit]);

  return (
    <div className="rounded-xl border border-border bg-card/60 p-4">
      <div className="text-xs text-muted-foreground uppercase tracking-wider">Event Timeline</div>
      <div className="mt-3 grid gap-2">
        {recent.length ? (
          recent.map((ev) => {
            const meta = ev.metadata && Object.keys(ev.metadata).length ? safeStringify(ev.metadata) : "";
            return (
              <div
                key={`${ev.timestamp}-${ev.type}-${ev.message}`}
                className="grid grid-cols-[110px_170px_1fr] gap-3 text-xs font-mono px-3 py-2 rounded border border-border bg-background/40"
              >
                <div className="text-muted-foreground">{formatUnixAgo(ev.timestamp)}</div>
                <div className={typeClass(ev.type)}>{ev.type}</div>
                <div className="text-muted-foreground">
                  <div className="text-foreground">{ev.message}</div>
                  {meta ? <div className="mt-1 text-muted-foreground/80 break-words">{meta}</div> : null}
                </div>
              </div>
            );
          })
        ) : (
          <div className="text-sm text-muted-foreground">No events yet.</div>
        )}
      </div>
    </div>
  );
}

