"use client";

import type { DashboardSnapshot } from "../../lib/dashboard/types";
import { formatUnixAgo } from "./format";
import { severityColor } from "./status";

const windowToSec = (w: "15m" | "1h" | "24h") => {
  if (w === "15m") return 15 * 60;
  if (w === "1h") return 60 * 60;
  return 24 * 60 * 60;
};

export const ActivityFeed = ({
  snapshot,
  severity,
  timeWindow,
}: {
  snapshot: DashboardSnapshot;
  severity: "all" | DashboardSnapshot["activity"][number]["severity"];
  timeWindow: "15m" | "1h" | "24h";
}) => {
  const cutoff = snapshot.generatedAtUnix - windowToSec(timeWindow);
  const items = snapshot.activity
    .slice()
    .filter((a) => a.tsUnix >= cutoff)
    .filter((a) => (severity === "all" ? true : a.severity === severity))
    .sort((a, b) => b.tsUnix - a.tsUnix)
    .slice(0, 10);

  return (
    <div className="rounded-xl border border-border bg-card/60 p-4 shadow-[0_0_0_1px_var(--border)]">
      <div className="flex items-center justify-between gap-4">
        <div className="text-foreground font-bold">Activity</div>
        <div className="text-xs text-muted-foreground font-mono">{formatUnixAgo(snapshot.generatedAtUnix)}</div>
      </div>

      <div className="mt-4 grid gap-2">
        {items.length === 0 ? (
          <div className="rounded-lg border border-border bg-card p-3 text-sm text-muted-foreground">No activity.</div>
        ) : (
          items.map((a) => (
              <div key={a.id} className="rounded-lg border border-border bg-card p-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="inline-block w-2.5 h-2.5 rounded-full" style={{ backgroundColor: severityColor[a.severity] }} />
                    <div className="text-sm text-foreground truncate">{a.message}</div>
                  </div>
                  <div className="text-xs font-mono text-muted-foreground">{formatUnixAgo(a.tsUnix)}</div>
                </div>
                {a.regionLabel ? <div className="mt-1 text-xs text-muted-foreground">{a.regionLabel}</div> : null}
              </div>
            ))
        )}
      </div>
    </div>
  );
};
