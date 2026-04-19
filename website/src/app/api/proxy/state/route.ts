import { NextResponse } from "next/server";

const runtimeBase =
  process.env.SUNLIONET_RUNTIME_API_BASE ?? process.env.SHADOWNET_RUNTIME_API_BASE ?? "http://127.0.0.1:8080";

export const dynamic = "force-dynamic";

export async function GET() {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 800);
  try {
    const res = await fetch(`${runtimeBase}/api/state`, {
      cache: "no-store",
      signal: controller.signal,
    });
    const body = await res.text();
    return new NextResponse(body, {
      status: res.status,
      headers: {
        "content-type": res.headers.get("content-type") ?? "application/json",
        "cache-control": "no-store",
      },
    });
  } catch {
    return NextResponse.json({ error: "runtime_unavailable" }, { status: 503, headers: { "cache-control": "no-store" } });
  } finally {
    clearTimeout(timeout);
  }
}
