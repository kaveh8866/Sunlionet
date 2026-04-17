import { NextResponse } from "next/server";

const runtimeBase = process.env.SHADOWNET_RUNTIME_API_BASE ?? "http://127.0.0.1:8080";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET() {
  try {
    const res = await fetch(`${runtimeBase}/api/events/stream`, {
      cache: "no-store",
      headers: { accept: "text/event-stream" },
    });
    if (!res.ok || !res.body) {
      const body = await res.text().catch(() => "");
      return new NextResponse(body || JSON.stringify({ error: "runtime_unavailable" }), {
        status: res.status || 503,
        headers: {
          "content-type": "application/json",
          "cache-control": "no-store",
        },
      });
    }
    return new NextResponse(res.body, {
      status: res.status,
      headers: {
        "content-type": "text/event-stream",
        "cache-control": "no-store",
        connection: "keep-alive",
      },
    });
  } catch {
    return NextResponse.json({ error: "runtime_unavailable" }, { status: 503, headers: { "cache-control": "no-store" } });
  }
}
