import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";

export const dynamic = "force-static";

export default function ArchitecturePage() {
  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title="Architecture"
          subtitle="ShadowNet separates the data plane (sing-box) from the local control plane (detector → policy engine → optional bounded advisor). Two binaries cooperate: Inside (restricted networks) and Outside (stable networks) for curated delivery via trusted channels."
          actions={
            <>
              <Link
                href="/docs/architecture"
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                Read full doc
              </Link>
              <Link
                href="/download"
                prefetch={false}
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                Download
              </Link>
            </>
          }
        />

        <section className="grid gap-6">
          <SectionHeader title="Planes" subtitle="A strict separation keeps the tunnel simple and the decision logic auditable." />
          <div className="grid md:grid-cols-2 gap-4">
            <InfoCard
              title="Data plane"
              eyebrow="Network"
              description={
                <ul className="grid gap-2">
                  <li>sing-box runs the tunnel/outbound.</li>
                  <li>Atomic reload applies new config without losing process state.</li>
                  <li>No telemetry or external control channel required.</li>
                </ul>
              }
            />
            <InfoCard
              title="Control plane"
              eyebrow="Local"
              description={
                <ul className="grid gap-2">
                  <li>Detector emits privacy-safe events (no domains, bounded buffer).</li>
                  <li>Policy Engine resolves routine cases deterministically.</li>
                  <li>Advisor is invoked only for ambiguous multi-signal cases.</li>
                </ul>
              }
            />
          </div>
        </section>

        <section className="grid gap-6">
          <SectionHeader title="Coordination" subtitle="Distribution stays peer-to-peer to reduce blocking and central points of failure." />
          <InfoCard
            title="Delivery model"
            tone="panel"
            description={
              <ul className="grid gap-2">
                <li>Signal bundles: Outside → Inside signed + encrypted profile delivery.</li>
                <li>Offline mesh: Bluetooth / Wi‑Fi Direct for local transfer during partial outages.</li>
                <li>No central seed API: the website never hosts live proxy seeds.</li>
              </ul>
            }
          />
        </section>

        <section className="grid gap-6">
          <SectionHeader title="Flow" subtitle="Core loop: observe → decide → apply → measure." />
          <div className="grid md:grid-cols-3 gap-4">
            <InfoCard
              title="Detector"
              eyebrow="Signals"
              description="Active probes + passive stats → event stream with jitter and budgets."
            />
            <InfoCard
              title="Policy engine"
              eyebrow="Deterministic"
              description="Scores and ranks profiles, applies switch/cooldown rules and diversity."
            />
            <InfoCard title="Supervisor" eyebrow="Apply" description="Generates config → atomic reload → updates health metrics." />
          </div>
        </section>
      </div>
    </div>
  );
}
