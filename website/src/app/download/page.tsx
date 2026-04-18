import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { resolveUILang, uiCopy } from "../../lib/uiCopy";
import { getLocalReleases } from "../../lib/releases/local";

export const dynamic = "force-static";

export default async function DownloadPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolveUILang(resolved.lang);
  const copy = uiCopy[lang].downloadPage;
  const releases = await getLocalReleases();
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title={copy.title}
          subtitle={copy.subtitle}
          actions={
            <>
              <Link
                href={hrefFor("/docs/outside/verification")}
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                {copy.verificationGuide}
              </Link>
              <Link
                href={hrefFor("/docs/install")}
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                {copy.install}
              </Link>
            </>
          }
        />

        <DownloadSection releases={releases} basePrefix={resolvedBasePrefix} />

        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            title={copy.installationGuide}
            description={copy.installationGuideDesc}
            href={hrefFor("/installation")}
          />
          <InfoCard
            title={copy.repoDocs}
            description={copy.repoDocsDesc}
            href={hrefFor("/docs")}
          />
        </div>
      </div>
    </div>
  );
}
