import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { Callout } from "../../components/ui/Callout";
import { resolveUILang, uiCopy } from "../../lib/uiCopy";
import { getLocalReleases } from "../../lib/releases/local";

export const dynamic = "force-static";

const githubRepo = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");
const githubReleasesUrl = `${githubRepo}/releases`;
const fdroidUrl = (process.env.NEXT_PUBLIC_FDROID_URL ?? "").trim();

export default async function DownloadPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolveUILang(resolved.lang);
  const copy = uiCopy[lang].downloadPage;
  const releases = await getLocalReleases();
  const isFa = lang === "fa";
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-12">
        <PageHeader
          title={copy.title}
          subtitle={copy.subtitle}
          actions={
            <>
              <Link
                href={hrefFor("/docs/outside/verification")}
                prefetch={false}
                data-testid="cta-verification"
                className="bg-primary hover:opacity-90 text-primary-foreground px-6 py-3 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                {copy.verificationGuide}
              </Link>
            </>
          }
        />

        <Callout title={isFa ? "یادداشت اعتماد" : "Trust Note"} tone="info">
          {copy.trustNote}
        </Callout>

        <section className="grid gap-8">
          <SectionHeader
            title={copy.officialSources}
            subtitle={isFa ? "کانال‌های تأیید شده برای دریافت سان‌لاین‌نت." : "Verified channels to obtain SunLionet."}
          />
          
          <div className="grid gap-6">
            <div className="rounded-2xl border border-primary/20 bg-primary/5 p-6 md:p-8">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div className="grid gap-2">
                  <div className="flex items-center gap-2">
                    <span className="bg-primary text-primary-foreground text-[10px] font-bold px-2 py-0.5 rounded uppercase tracking-wider">
                      {copy.recommended}
                    </span>
                    <h3 className="text-xl font-bold text-foreground">{copy.sources.github.title}</h3>
                  </div>
                  <p className="text-muted-foreground text-sm max-w-xl">
                    {copy.sources.github.desc}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {isFa
                      ? "با کلیک، صفحه نسخه‌های رسمی گیت‌هاب باز می‌شود. فایل را دانلود کنید، تأیید اصالت را بررسی کنید، سپس نصب/اجرا را انجام دهید."
                      : "Opens the official GitHub Releases page. Download your file, verify it, then install/run."}
                  </p>
                </div>
                <Link
                  href={githubReleasesUrl}
                  target="_blank"
                  data-testid="source-github-releases"
                  className="bg-primary hover:opacity-90 text-primary-foreground px-8 py-4 rounded-xl text-center font-bold transition-opacity"
                >
                  {isFa ? "مشاهده نسخه‌ها" : "View Releases"}
                </Link>
              </div>
            </div>

            <div className="grid md:grid-cols-2 gap-4">
              <div className="rounded-xl border border-border bg-card/40 p-6 flex flex-col justify-between gap-4">
                <div className="grid gap-2">
                  <div className="flex items-center gap-2">
                    <span className="bg-muted text-muted-foreground text-[10px] font-bold px-2 py-0.5 rounded uppercase tracking-wider">
                      {copy.alternative}
                    </span>
                    <h3 className="font-bold text-foreground">{copy.sources.fdroid.title}</h3>
                  </div>
                  <p className="text-sm text-muted-foreground">{copy.sources.fdroid.desc}</p>
                  <p className="text-xs text-muted-foreground">
                    {isFa
                      ? "با کلیک، صفحه رسمی F-Droid باز می‌شود. نصب و به‌روزرسانی‌ها با تأییدهای استاندارد اندروید انجام می‌شود."
                      : "Opens the official F-Droid listing. Install and updates still follow standard Android confirmation prompts."}
                  </p>
                </div>
                {fdroidUrl ? (
                  <Link
                    href={fdroidUrl}
                    target="_blank"
                    data-testid="source-fdroid"
                    className="w-full bg-card border border-border hover:bg-muted text-foreground px-4 py-2 rounded-lg text-sm font-semibold text-center transition-colors"
                  >
                    {isFa ? "باز کردن در F-Droid" : "Open in F-Droid"}
                  </Link>
                ) : (
                  <button
                    disabled
                    data-testid="source-fdroid-disabled"
                    className="w-full bg-muted text-muted-foreground px-4 py-2 rounded-lg text-sm font-semibold cursor-not-allowed"
                  >
                    {isFa ? "به‌زودی" : "Coming Soon"}
                  </button>
                )}
              </div>

              <div className="rounded-xl border border-border bg-card/40 p-6 flex flex-col justify-between gap-4">
                <div className="grid gap-2">
                  <div className="flex items-center gap-2">
                    <span className="bg-muted text-muted-foreground text-[10px] font-bold px-2 py-0.5 rounded uppercase tracking-wider">
                      {copy.alternative}
                    </span>
                    <h3 className="font-bold text-foreground">{copy.sources.directApk.title}</h3>
                  </div>
                  <p className="text-sm text-muted-foreground">{copy.sources.directApk.desc}</p>
                  <p className="text-xs text-muted-foreground">
                    {isFa
                      ? "با کلیک، به فایل‌های رسمی همین صفحه می‌روید. APK را دانلود کنید؛ سپس اندروید نصب را با تأیید شما انجام می‌دهد."
                      : "Scrolls to the official artifacts below. Download the APK; Android will still ask you to confirm install."}
                  </p>
                </div>
                <Link
                  href="#artifacts"
                  data-testid="source-direct-apk"
                  className="w-full bg-card border border-border hover:bg-muted text-foreground px-4 py-2 rounded-lg text-sm font-semibold text-center transition-colors"
                >
                  {isFa ? "دانلود مستقیم" : "Direct Download"}
                </Link>
              </div>
            </div>

            <div className="rounded-xl border border-border bg-card/20 p-6 border-dashed">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div className="grid gap-1">
                  <div className="flex items-center gap-2">
                    <span className="bg-muted/50 text-muted-foreground text-[10px] font-bold px-2 py-0.5 rounded uppercase tracking-wider">
                      {copy.advanced}
                    </span>
                    <h3 className="font-bold text-foreground">{copy.sources.termux.title}</h3>
                  </div>
                  <p className="text-sm text-muted-foreground">{copy.sources.termux.desc}</p>
                  <p className="text-xs text-muted-foreground">
                    {isFa ? "با کلیک، راهنمای خط فرمان باز می‌شود (فقط برای کاربران فنی)." : "Opens the CLI guide (advanced users only)."}
                  </p>
                </div>
                <Link
                  href={hrefFor("/docs/install")}
                  data-testid="source-termux"
                  className="text-primary text-sm font-bold hover:underline"
                >
                  {isFa ? "مشاهده راهنما" : "View Guide"} →
                </Link>
              </div>
            </div>
          </div>
        </section>

        <section id="artifacts" className="grid gap-8 pt-8 border-t border-border">
          <SectionHeader
            title={isFa ? "دریافت مستقیم فایل‌ها" : "Direct Artifact Access"}
            subtitle={isFa ? "دسترسی به تمامی نسخه‌های بیلد شده." : "Direct access to all build artifacts."}
          />
          <DownloadSection releases={releases} basePrefix={resolvedBasePrefix} />
        </section>

        <Callout title={isFa ? "هشدار امنیتی" : "Security Warning"} tone="warning">
          {copy.warningMirror}
        </Callout>

        <div className="grid md:grid-cols-2 gap-6">
          <InfoCard
            title={copy.installationGuide}
            description={copy.installationGuideDesc}
            href={hrefFor("/installation")}
          />
          <InfoCard
            title={copy.repoDocs}
            description={copy.repoDocsDesc}
            href={hrefFor("/docs/distribution-policy")}
          />
        </div>
      </div>
    </div>
  );
}
