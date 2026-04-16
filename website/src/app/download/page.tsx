import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { getLocalReleases } from "../../lib/releases/local";

export const dynamic = "force-static";

export default async function DownloadPage() {
  const releases = await getLocalReleases();

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title="Download"
          subtitle="Production-style release artifacts with verification-first install steps. Always verify checksums before running."
          actions={
            <>
              <Link
                href="/docs/outside/verification"
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                Verification guide
              </Link>
              <Link
                href="/docs/install"
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                Install
              </Link>
            </>
          }
        />

        <DownloadSection releases={releases} />

        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            title="Installation (web guide)"
            description="A step-by-step website page for the common paths (Linux, Termux)."
            href="/installation"
          />
          <InfoCard
            title="Repository docs"
            description="The authoritative install, verification, and security docs from the repo."
            href="/docs"
          />
        </div>
      </div>
    </div>
  );
}
