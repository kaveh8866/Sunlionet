export const dynamic = "force-static";
import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { getDocsIndex, readDocMarkdownBySlug } from "../../lib/docs/fs";
import { renderMarkdown } from "../../lib/docs/markdown";

export default async function DocsIndexPage() {
  const entries = await getDocsIndex();
  const index = new Map(entries.map((e) => [e.slug.join("/"), e]));

  const overview = await readDocMarkdownBySlug(["index"]);
  const rendered = overview ? renderMarkdown(overview.raw, { baseSlug: ["index"] }) : null;

  return (
    <div className="grid gap-12">
      <PageHeader
        title="Documentation"
        subtitle="Repository-backed docs for ShadowNet Agent. Local-first, no analytics, no live seed hosting."
        actions={
          <>
            <Link
              href="/download"
              prefetch={false}
              className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
            >
              Download
            </Link>
            <Link
              href="/architecture"
              prefetch={false}
              className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
            >
              Website architecture page
            </Link>
          </>
        }
      />

      {rendered ? (
        <div className="rounded-xl border border-border bg-card/40 p-6">
          <article className="docs-prose">{rendered.nodes}</article>
        </div>
      ) : null}

      <section className="grid gap-6">
        <SectionHeader
          title="Start here"
          subtitle="The quickest path to installing safely and understanding how Inside/Outside work together."
        />
        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            href="/docs/install"
            title={index.get("install")?.title ?? "Installation"}
            description="Release artifacts, verification, and basic setup."
          />
          <InfoCard
            href="/docs/user/safety"
            title={index.get("user/safety")?.title ?? "Safety"}
            description="Operational safety principles for high-risk environments."
          />
          <InfoCard
            href="/docs/architecture"
            title={index.get("architecture")?.title ?? "Architecture"}
            description="Inside vs Outside, data plane vs control plane."
          />
          <InfoCard
            href="/docs/outside/verification"
            title={index.get("outside/verification")?.title ?? "Verification"}
            description="Verify artifacts and bundles before use."
          />
        </div>
      </section>

      <section className="grid gap-6" lang="fa" dir="rtl">
        <SectionHeader title="فارسی" subtitle="ترجمه فارسی برای مسیرهای اصلی نصب، ایمنی، و تأیید." />
        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            href="/docs/fa"
            title={index.get("fa/index")?.title ?? "مستندات (فارسی)"}
            description="فهرست مستندات فارسی و لینک به صفحات مهم."
          />
          <InfoCard
            href="/docs/fa/user/safety"
            title={index.get("fa/user/safety")?.title ?? "ایمنی"}
            description="محدودیت‌ها و اصول ایمنی در شرایط پرریسک."
          />
        </div>
      </section>
    </div>
  );
}
