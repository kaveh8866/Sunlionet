export const dynamic = "force-static";

const items = [
  {
    title: "Mesh transfer during blackouts",
    desc: "Bluetooth/Wi‑Fi Direct local exchange of signed, encrypted seed bundles when the internet is unavailable.",
  },
  {
    title: "Bounded advisor tuning (offline)",
    desc: "Synthetic-data fine-tuning for the local advisor to improve profile selection without any external API calls.",
  },
  {
    title: "Android VPN wrapper",
    desc: "Separate Android app (VpnService + foreground service) that embeds the Inside engine and exposes a safe UI.",
  },
  {
    title: "Release automation",
    desc: "CI builds for all targets, checksum generation, and reproducible build steps.",
  },
];

export default function RoadmapPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-4xl">
      <h1 className="text-4xl font-extrabold tracking-tight text-white mb-4">Roadmap</h1>
      <p className="text-gray-400 leading-relaxed max-w-3xl">
        ShadowNet is designed to be practical for solo development while remaining resilient under 2026 DPI conditions.
        This roadmap focuses on features that increase safety, uptime, and offline survivability.
      </p>

      <div className="mt-10 grid gap-4">
        {items.map((i) => (
          <div key={i.title} className="rounded-xl border border-gray-800 bg-gray-900/40 p-6">
            <div className="text-white font-bold">{i.title}</div>
            <div className="text-gray-400 text-sm mt-2 leading-relaxed">{i.desc}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

