"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { getRuntimeEvents, getRuntimeState, type RuntimeEvent, type RuntimeSnapshot } from "../../lib/runtime-client";
import { formatUnixAgo } from "./format";
import { EventTimeline } from "./EventTimeline";

type UIStatus = "CONNECTING" | "CONNECTED" | "DISCONNECTED" | "ERROR";

function mapStatus(s: string): UIStatus {
  const v = s.trim().toLowerCase();
  if (v === "connected") return "CONNECTED";
  if (v === "connecting") return "CONNECTING";
  if (v === "error" || v === "failed") return "ERROR";
  return "DISCONNECTED";
}

export function RuntimeDashboard() {
  const [state, setState] = useState<RuntimeSnapshot | null>(null);
  const [events, setEvents] = useState<RuntimeEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const snapshotTimerRef = useRef<number | null>(null);

  const uiStatus = useMemo(() => mapStatus(state?.status ?? "disconnected"), [state?.status]);

  const mergeEvents = useCallback((prev: RuntimeEvent[], incoming: RuntimeEvent[]) => {
    const map = new Map<string, RuntimeEvent>();
    for (const ev of [...incoming, ...prev]) {
      const key = `${ev.timestamp}-${ev.type}-${ev.message}`;
      if (!map.has(key)) map.set(key, ev);
    }
    const merged = [...map.values()].sort((a, b) => b.timestamp - a.timestamp);
    return merged.slice(0, 120);
  }, []);

  const refreshSnapshot = useCallback(async () => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 1500);
    try {
      const nextState = await getRuntimeState(controller.signal);
      setState(nextState);
      setError(null);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "runtime fetch failed";
      setError(msg);
      setState(null);
    } finally {
      window.clearTimeout(timeout);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 1500);
    void Promise.all([getRuntimeState(controller.signal), getRuntimeEvents(controller.signal)])
      .then(([nextState, nextEvents]) => {
        setState(nextState);
        setEvents((prev) => mergeEvents(prev, nextEvents));
        setError(null);
      })
      .catch((e) => {
        const msg = e instanceof Error ? e.message : "runtime fetch failed";
        setError(msg);
        setState(null);
        setEvents([]);
      })
      .finally(() => {
        window.clearTimeout(timeout);
      });

    const es = new EventSource("/api/proxy/events");
    eventSourceRef.current = es;
    es.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data) as RuntimeEvent;
        setEvents((prev) => mergeEvents(prev, [parsed]));
        setError(null);
      } catch {
        setError("event stream parse failed");
      }
    };
    es.onerror = () => {
      setError("event stream disconnected");
    };

    if (snapshotTimerRef.current) {
      window.clearInterval(snapshotTimerRef.current);
      snapshotTimerRef.current = null;
    }
    snapshotTimerRef.current = window.setInterval(() => {
      void refreshSnapshot();
    }, 15000);

    return () => {
      es.close();
      eventSourceRef.current = null;
      if (snapshotTimerRef.current) {
        window.clearInterval(snapshotTimerRef.current);
        snapshotTimerRef.current = null;
      }
    };
  }, [mergeEvents, refreshSnapshot]);

  const badgeClass =
    uiStatus === "CONNECTED"
      ? "border-emerald-400/40 text-emerald-300 bg-emerald-950/30"
      : uiStatus === "CONNECTING"
        ? "border-amber-400/40 text-amber-300 bg-amber-950/30"
        : uiStatus === "ERROR"
          ? "border-red-400/40 text-red-300 bg-red-950/30"
          : "border-border text-muted-foreground bg-card/60";

  return (
    <div className="grid gap-6">
      <div className="flex items-start justify-between gap-6 flex-wrap">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground uppercase tracking-wider">ShadowNet Dashboard</div>
          <div className="mt-1 flex items-center gap-3 flex-wrap">
            <div className="text-foreground font-bold text-lg">ShadowNet Inside</div>
            <div data-testid="runtime-status-badge" className={`text-xs font-mono px-2 py-1 rounded border ${badgeClass}`}>
              {uiStatus}
            </div>
            <div className="text-xs font-mono text-muted-foreground">
              updated {state ? formatUnixAgo(state.lastUpdated) : "—"}
            </div>
          </div>
          <div className="mt-2 text-sm text-muted-foreground max-w-3xl">
            Live view of the local runtime on this machine (no internet required). Streaming events in real time.
          </div>
          {error ? (
            <div className="mt-2 text-xs font-mono text-red-400 border border-border rounded px-3 py-2 bg-card/60">
              runtime unavailable: {error}
            </div>
          ) : null}
        </div>
      </div>

      {!state ? (
        <div className="rounded-xl border border-border bg-card/60 p-4">
          <div className="text-foreground font-semibold">No active ShadowNet runtime detected</div>
          <div className="mt-2 text-sm text-muted-foreground">
            Start ShadowNet Inside locally with the runtime API enabled, then refresh this page.
          </div>
          <div className="mt-3 text-xs font-mono border border-border bg-background/40 rounded px-3 py-2 overflow-auto">
            shadownet-inside --runtime-api-addr 127.0.0.1:8080 --runtime-api-keepalive ...
          </div>
        </div>
      ) : (
        <div className="grid md:grid-cols-3 gap-6">
          <div className="rounded-xl border border-border bg-card/60 p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Active Profile</div>
            <div className="mt-2 font-mono text-sm text-foreground">{state.activeProfile || "—"}</div>
          </div>
          <div className="rounded-xl border border-border bg-card/60 p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Latency</div>
            <div className="mt-2 font-mono text-sm text-foreground">
              {uiStatus === "CONNECTED" ? `${state.latencyMs} ms` : "—"}
            </div>
          </div>
          <div className="rounded-xl border border-border bg-card/60 p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Mode</div>
            <div className="mt-2 font-mono text-sm text-foreground">{state.mode || "—"}</div>
          </div>

          <div className="md:col-span-3 rounded-xl border border-border bg-card/60 p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Last Event</div>
            <div className="mt-2 text-sm text-muted-foreground">
              {events.length ? `${formatUnixAgo(events[0].timestamp)} • ${events[0].message}` : "—"}
            </div>
          </div>
        </div>
      )}

      <details className="rounded-xl border border-border bg-card/60 p-4">
        <summary className="cursor-pointer text-foreground font-semibold">Show activity</summary>
        <div className="mt-4">
          <EventTimeline events={events} limit={100} />
        </div>
      </details>
    </div>
  );
}
