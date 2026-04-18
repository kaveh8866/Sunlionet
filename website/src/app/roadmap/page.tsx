import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";

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

export default async function RoadmapPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title="Roadmap"
          subtitle="Near-term work that improves safety, uptime, verification, and offline survivability."
          actions={
            <Link
              href={hrefFor("/docs/governance/model")}
              prefetch={false}
              className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
            >
              Governance model
            </Link>
          }
        />

        <div className="grid gap-4">
          {items.map((i) => (
            <InfoCard key={i.title} title={i.title} description={i.desc} />
          ))}
        </div>
      </div>
    </div>
  );
}
