export const dynamic = "force-static";
import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { getDocsIndex, readDocMarkdownBySlug } from "../../lib/docs/fs";
import { renderMarkdown } from "../../lib/docs/markdown";

export default async function DocsIndexPage({
  params,
}: {
  params?: Promise<{ lang?: string }>;
}) {
  const resolved = (await params) ?? {};
  const entries = await getDocsIndex();
  const index = new Map(entries.map((e) => [e.slug.join("/"), e]));
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;
  const isFa = resolvedBasePrefix === "/fa";

  const overviewSlug = isFa ? ["fa", "index"] : ["index"];
  const overview = await readDocMarkdownBySlug(overviewSlug);
  const overviewRenderSlug = isFa ? overviewSlug.slice(1) : overviewSlug;
  const rendered = overview
    ? renderMarkdown(overview.raw, { baseSlug: overviewRenderSlug, basePrefix: resolvedBasePrefix })
    : null;
  const titleFor = (key: string, fallback: string) => index.get(isFa ? `fa/${key}` : key)?.title ?? fallback;

  return (
    <div className="grid gap-12">
      <PageHeader
        title={isFa ? "مستندات" : "Documentation"}
        subtitle={
          isFa
            ? "مستندات مبتنی بر مخزن برای SunLionet. بدون تحلیل‌گر، بدون میزبانی seed، و با اولویت استفاده محلی."
            : "Repository-backed docs for SunLionet. Local-first, no analytics, no live seed hosting."
        }
        actions={
          <>
            <Link
              href={hrefFor("/download")}
              prefetch={false}
              className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
            >
              {isFa ? "دانلود" : "Download"}
            </Link>
            <Link
              href={hrefFor("/architecture")}
              prefetch={false}
              className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
            >
              {isFa ? "صفحه معماری وب‌سایت" : "Website architecture page"}
            </Link>
          </>
        }
      />

      {rendered ? (
        <div className="rounded-xl border border-border bg-card/40 p-6">
          <article className="docs-prose" lang={isFa ? "fa" : undefined} dir={isFa ? "rtl" : undefined}>
            {rendered.nodes}
          </article>
        </div>
      ) : null}

      <section className="grid gap-6">
        <SectionHeader
          title={isFa ? "راهنمای استفاده" : "User Guides"}
          subtitle={
            isFa
              ? "آموزش‌های گام‌به‌گام برای کاربران نهایی."
              : "Step-by-step guides for end-users."
          }
        />
        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            href={hrefFor("/docs/install")}
            title={isFa ? "نصب و راه‌اندازی" : "Installation"}
            description={isFa ? "چگونه سان‌لاین‌نت را روی دستگاه خود نصب کنید." : "How to install SunLionet on your device."}
          />
          <InfoCard
            href={hrefFor("/docs/user/safety")}
            title={isFa ? "نکات ایمنی" : "Safety & Privacy"}
            description={isFa ? "اصول ایمنی برای استفاده در محیط‌های حساس." : "Security principles for sensitive environments."}
          />
        </div>
      </section>

      <section className="grid gap-6">
        <SectionHeader
          title={isFa ? "مستندات فنی" : "Technical Reference"}
          subtitle={
            isFa
              ? "جزئیات معماری و امنیت برای توسعه‌دهندگان."
              : "Architecture and security details for developers."
          }
        />
        <div className="grid md:grid-cols-2 gap-4">
          <InfoCard
            href={hrefFor("/docs/architecture")}
            title={isFa ? "معماری سیستم" : "Architecture"}
            description={isFa ? "بررسی اجزای Inside و Outside و نحوه تعامل آن‌ها." : "Deep dive into Inside/Outside components."}
          />
          <InfoCard
            href={hrefFor("/docs/outside/verification")}
            title={isFa ? "تأیید اصالت" : "Verification"}
            description={isFa ? "جزئیات فنی بررسی امضا و سلامت فایل‌ها." : "Technical details of signature and integrity checks."}
          />
        </div>
      </section>

      {isFa ? null : (
        <section className="grid gap-6" lang="fa" dir="rtl">
          <SectionHeader title="فارسی" subtitle="نسخه فارسی برای مسیرهای اصلی نصب، ایمنی، و تأیید." />
          <div className="grid md:grid-cols-2 gap-4">
            <InfoCard href="/fa/docs" title="مستندات فارسی" description="فهرست مستندات فارسی و لینک به صفحات مهم." />
            <InfoCard href="/fa/docs/user/safety" title="ایمنی" description="اصول ایمنی عملیاتی در محیط‌های پرریسک." />
          </div>
        </section>
      )}
    </div>
  );
}
