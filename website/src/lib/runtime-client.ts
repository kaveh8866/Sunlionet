export type RuntimeSnapshot = {
  status: string;
  activeProfile: string;
  latencyMs: number;
  lastUpdated: number;
  failures: { timestamp: number; reason: string }[];
  mode: string;
};

export type RuntimeEvent = {
  timestamp: number;
  type: string;
  message: string;
  metadata?: Record<string, unknown>;
};

async function fetchJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { cache: "no-store", signal });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `${path} failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

export async function getRuntimeState(signal?: AbortSignal) {
  return fetchJSON<RuntimeSnapshot>("/api/proxy/state", signal);
}

export async function getRuntimeEvents(signal?: AbortSignal) {
  return fetchJSON<RuntimeEvent[]>("/api/proxy/events/list", signal);
}

export async function getRuntimeHealth(signal?: AbortSignal) {
  return fetchJSON<{ status: string }>("/api/proxy/health", signal);
}
