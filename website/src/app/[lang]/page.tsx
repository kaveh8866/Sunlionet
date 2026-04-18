import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";
import { Callout } from "../../components/ui/Callout";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { siteCopy } from "../../content/siteCopy";
import { getLocalReleases } from "../../lib/releases/local";

export default async function Home({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const releases = await getLocalReleases();

  const isFa = lang === "fa";
  const base = `/${lang}`;
  const copy = siteCopy[lang];

  return (
    <div dir={isFa ? "rtl" : "ltr"} className={isFa ? "font-fa" : undefined}>
      <div className="mx-auto w-full max-w-6xl px-4 py-12">
        <div className="grid gap-12">
          <PageHeader
            eyebrow="SunLionet"
            title={copy.home.headline}
            subtitle={copy.home.subheadline}
            actions={
              <>
                <Link
                  href={`${base}/installation`}
                  prefetch={false}
                  className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
                >
                  {copy.home.cta.getStarted}
                </Link>
                <Link
                  href={`${base}/manifest`}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                >
                  {copy.home.cta.readManifest}
                </Link>
                <Link
                  href={`${base}/download`}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                >
                  {copy.home.cta.download}
                </Link>
              </>
            }
          />

          <Callout title={isFa ? "هشدار ایمنی" : "Safety notice"} tone="warning">
            {isFa ? (
              <>
                فرض کنید دستگاه‌ها قابل توقیف هستند و ترافیک قابل پایش است. لاگ‌ها را حداقلی نگه دارید، فایل‌ها را قبل از اجرا بررسی کنید و
                راهنمای ایمنی عملیاتی را در{" "}
                <Link href={`${base}/docs/user/safety`} prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
                  /docs/user/safety
                </Link>{" "}
                بخوانید.
              </>
            ) : (
              <>
                Assume devices can be seized and traffic can be monitored. Prefer minimal logs, verify downloaded artifacts, and follow the
                operational safety guidance in{" "}
                <Link href={`${base}/docs/user/safety`} prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
                  /docs/user/safety
                </Link>
                .
              </>
            )}
          </Callout>

          <section className="grid gap-6">
            <SectionHeader
              title={isFa ? "معماری دو بخشی" : "Two-part architecture"}
              subtitle={isFa ? "Inside حلقهٔ محلی detector/policy را اجرا می‌کند؛ Outside بسته‌ها را تولید و از کانال‌های قابل اعتماد توزیع می‌کند." : "Inside runs the local detector/policy loop. Outside produces bundles and shares them through trusted channels."}
            />
            <div className="grid md:grid-cols-2 gap-4">
              <InfoCard
                eyebrow={isFa ? "روی دستگاه" : "On-device"}
                title={isFa ? "SunLionet Inside" : "SunLionet Inside"}
                description={
                  isFa
                    ? "Detector → موتور سیاست‌گذاری قطعی → reload در sing-box. رفتار قابل پیش‌بینی و محلی را ترجیح می‌دهد."
                    : "Detector → deterministic policy engine → sing-box reload. Operates locally and prioritizes predictable behavior."
                }
              />
              <InfoCard
                eyebrow={isFa ? "نود پشتیبان" : "Supporter node"}
                title={isFa ? "SunLionet Outside" : "SunLionet Outside"}
                description={
                  isFa
                    ? "Discovery → validation → بسته‌های امضاشده+رمزگذاری‌شده. برای توزیع کنترل‌شده (مثلاً Signal) طراحی شده."
                    : "Discovery → validation → signed+encrypted bundles. Produces artifacts intended for controlled delivery (e.g., Signal)."
                }
              />
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader
              title={isFa ? "چه کار می‌کند" : "What it does"}
              subtitle={isFa ? "یک مجموعه قابلیت کوچک و قابل مشاهده/قابل تست." : "A small set of capabilities, designed to be observable and testable."}
            />
            <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4">
              <InfoCard
                title={isFa ? "سیگنال‌های DPI / مسدودسازی" : "DPI / blocking signals"}
                description={
                  isFa ? "سیگنال‌های محلی را جمع‌آوری می‌کند (DNS poisoning، reset، injection، UDP drop) و رویداد تولید می‌کند." : "Collects local signals (DNS poisoning, resets, injection, UDP drops) and emits events."
                }
              />
              <InfoCard
                title={isFa ? "چرخش پروتکل/پروفایل" : "Protocol/profile rotation"}
                description={
                  isFa ? "پروفایل‌ها را به‌صورت قطعی رتبه‌بندی می‌کند و با تغییر شبکه سوییچ می‌کند." : "Ranks candidate profiles deterministically and switches when the network changes."
                }
              />
              <InfoCard
                title={isFa ? "اعتبارسنجی بسته‌ها" : "Bundle verification"}
                description={
                  isFa ? "Outside بسته تولید می‌کند؛ Inside امضا را بررسی کرده و فقط کانفیگ‌های قابل اعتماد را اعمال می‌کند." : "Outside generates bundles; Inside verifies signatures and applies only trusted configs."
                }
              />
              <InfoCard
                title={isFa ? "مسیرهای انتقال آفلاین" : "Offline transfer paths"}
                description={isFa ? "برای کارکرد در اختلال‌های شدید با انتقال خارج از بستر اینترنت طراحی شده." : "Designed to keep working during partial outages via out-of-band transfer."}
              />
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader title={isFa ? "از اینجا شروع کنید" : "Start here"} subtitle={isFa ? "برای شروع سریع صفحات وب کافی است؛ برای جزئیات از مستندات مخزن استفاده کنید." : "Use the website pages for quick orientation; use the repository-backed docs for depth."} />
            <div className="grid md:grid-cols-3 gap-4">
              <InfoCard href={`${base}/installation`} title={isFa ? "نصب" : "Installation"} description={isFa ? "مسیرهای نصب و نمونه‌های بررسی SHA256." : "Common install paths and SHA256 verification examples."} />
              <InfoCard href={`${base}/docs`} title={isFa ? "مستندات" : "Documentation"} description={isFa ? "مستندات مبتنی بر مخزن با لینک‌های پایدار." : "Repository-backed docs with a table of contents and stable URLs."} />
              <InfoCard href={`${base}/architecture`} title={isFa ? "معماری" : "Architecture"} description={isFa ? "جریان Inside/Outside، اجزا، و مدل عملیاتی." : "Inside vs Outside data flow, components, and operational model."} />
            </div>
          </section>

          <div id="downloads">
            <DownloadSection releases={releases} basePrefix={base} />
          </div>

          <section className="grid gap-6">
            <SectionHeader
              title={isFa ? "همکاری و مشارکت" : "Contribute & cooperate"}
              subtitle={isFa ? "تغییرات را کوچک، قابل بازبینی و حریم‌خصوصی‌محور نگه دارید. نیازی به اطلاعات شخصی نیست." : "Keep changes small, reviewable, and privacy-preserving. No personal data required."}
            />
            <div className="grid md:grid-cols-2 gap-4">
              <InfoCard title={isFa ? "Outside را اجرا کنید و بسته‌ها را منتقل کنید" : "Run Outside and deliver bundles"} description={isFa ? "پشتیبان‌ها می‌توانند SunLionet Outside را اجرا کنند و خروجی را از کانال‌های قابل اعتماد منتقل کنند." : "Supporters can run SunLionet Outside and transfer artifacts through trusted channels."} />
              <InfoCard title={isFa ? "مشاهدات میدانی را گزارش کنید" : "Report field observations"} description={isFa ? "یادداشت‌های ناشناس دربارهٔ بلاک/ریست‌های جدید بدون اتصال به شناسه‌های شخصی." : "Share anonymized notes about new blocks/resets without linking personal identifiers."} />
              <InfoCard title={isFa ? "مشارکت کد (AGPL-3.0)" : "Contribute code (AGPL-3.0)"} description={isFa ? "مشارکت ناشناس در GitHub خوش‌آمد است. PRهای کوچک و دارای تست ترجیح داده می‌شوند." : "Anonymous GitHub contributions are welcome. Prefer incremental PRs with tests."} />
              <InfoCard href={`${base}/support`} title={isFa ? "حمایت" : "Support"} description={isFa ? "راه‌های کم‌ریسک و ناشناس برای کمک." : "Anonymous donation options and other low-risk ways to help."} />
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader title={isFa ? "مجوز و مسئولیت" : "License & responsibility"} subtitle={isFa ? "قبل از اجرا، قوانین محلی و مدل ریسک شخصی خود را در نظر بگیرید." : "Understand your local laws and your personal risk model before running."} />
            <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
              <div className="grid gap-5">
                <Callout title={isFa ? "مجوز" : "License"} tone="info">
                  {isFa ? "SunLionet تحت مجوز AGPL-3.0 منتشر می‌شود." : "SunLionet is licensed under AGPL-3.0."}
                </Callout>
                <Callout title={isFa ? "سلب مسئولیت" : "Disclaimer"}>
                  {isFa
                    ? "مسئولیت نصب و استفاده از این نرم‌افزار با اجراکننده است. با آگاهی از ریسک و مطابق قوانین محلی استفاده کنید."
                    : "The responsibility for installing and operating this software lies with the person who runs it. Use at your own risk and in accordance with your local laws."}
                </Callout>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
