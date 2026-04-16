import Link from "next/link";
import { DownloadSection } from "../components/DownloadSection";
import { Callout } from "../components/ui/Callout";
import { InfoCard } from "../components/ui/InfoCard";
import { PageHeader } from "../components/ui/PageHeader";
import { SectionHeader } from "../components/ui/SectionHeader";
import { getLocalReleases } from "../lib/releases/local";

export default async function Home() {
  const releases = await getLocalReleases();

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-12">
        <PageHeader
          eyebrow="Local-first censorship resilience tooling"
          title="ShadowNet Agent"
          subtitle={
            <>
              ShadowNet is split into two cooperating binaries: ShadowNet-Inside (runs on the censored device and manages the data plane) and
              ShadowNet-Outside (runs by supporters to generate signed and encrypted configuration bundles). No analytics, no seed hosting, and a
              bias toward offline operation.
            </>
          }
          actions={
            <>
              <Link
                href="#downloads"
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                Download
              </Link>
              <Link
                href="/docs"
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                Docs
              </Link>
              <Link
                href="/architecture"
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                Architecture
              </Link>
            </>
          }
        />

        <Callout title="Safety notice" tone="warning">
          Assume devices can be seized and traffic can be monitored. Prefer minimal logs, verify downloaded artifacts, and follow the operational
          safety guidance in{" "}
          <Link href="/docs/user/safety" prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
            /docs/user/safety
          </Link>
          .
        </Callout>

        <section className="grid gap-6">
          <SectionHeader
            title="Two-part architecture"
            subtitle="Inside runs the local detector/policy loop. Outside produces bundles and shares them through trusted channels."
          />
          <div className="grid md:grid-cols-2 gap-4">
            <InfoCard
              eyebrow="On-device"
              title="ShadowNet-Inside"
              description="Detector → deterministic policy engine → sing-box reload. Operates locally and prioritizes predictable behavior."
            />
            <InfoCard
              eyebrow="Supporter node"
              title="ShadowNet-Outside"
              description="Discovery → validation → signed+encrypted bundles. Produces artifacts intended for controlled delivery (e.g., Signal)."
            />
          </div>
        </section>

        <section className="grid gap-6">
          <SectionHeader title="What it does" subtitle="A small set of capabilities, designed to be observable and testable." />
          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4">
            <InfoCard title="DPI / blocking signals" description="Collects local signals (DNS poisoning, resets, injection, UDP drops) and emits events." />
            <InfoCard title="Protocol/profile rotation" description="Ranks candidate profiles deterministically and switches when the network changes." />
            <InfoCard title="Bundle verification" description="Outside generates bundles; Inside verifies signatures and applies only trusted configs." />
            <InfoCard title="Offline transfer paths" description="Designed to keep working during partial outages via out-of-band transfer." />
          </div>
        </section>

        <section className="grid gap-6">
          <SectionHeader title="Start here" subtitle="Use the website pages for quick orientation; use the repository-backed docs for depth." />
          <div className="grid md:grid-cols-3 gap-4">
            <InfoCard href="/installation" title="Installation" description="Common install paths and SHA256 verification examples." />
            <InfoCard href="/docs" title="Documentation" description="Repository-backed docs with a table of contents and stable URLs." />
            <InfoCard href="/architecture" title="Architecture" description="Inside vs Outside data flow, components, and operational model." />
          </div>
        </section>

        <div>
          <DownloadSection releases={releases} />
        </div>

        <section className="grid gap-6">
          <SectionHeader title="Contribute & cooperate" subtitle="Keep changes small, reviewable, and privacy-preserving. No personal data required." />
          <div className="grid md:grid-cols-2 gap-4">
            <InfoCard title="Run Outside and deliver bundles" description="Supporters can run ShadowNet-Outside and transfer artifacts through trusted channels." />
            <InfoCard title="Report field observations" description="Share anonymized notes about new blocks/resets without linking personal identifiers." />
            <InfoCard title="Contribute code (AGPL-3.0)" description="Anonymous GitHub contributions are welcome. Prefer incremental PRs with tests." />
            <InfoCard href="/support" title="Support" description="Anonymous donation options and other low-risk ways to help." />
          </div>
        </section>

        <section className="grid gap-6">
          <SectionHeader title="License & responsibility" subtitle="Understand your local laws and your personal risk model before running." />
          <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
            <div className="grid gap-5">
              <Callout title="License" tone="info">
                ShadowNet Agent is licensed under AGPL-3.0.
              </Callout>
              <Callout title="Disclaimer">
                The responsibility for installing and operating this software lies with the person who runs it. Use at your own risk and in
                accordance with your local laws.
              </Callout>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
