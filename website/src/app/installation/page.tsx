import Link from "next/link";
import { CodeBlockShell } from "../../components/ui/CodeBlockShell";
import { PageHeader } from "../../components/ui/PageHeader";
import { getLocalReleases } from "../../lib/releases/local";
import { resolveUILang, uiCopy } from "../../lib/uiCopy";

export const dynamic = "force-static";

const githubRepo = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");
const githubDocsInstall = `${githubRepo}/blob/main/docs/install.md`;

function Step({ n, title, children }: { n: string; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
      <div className="flex items-start gap-4">
        <div className="w-10 h-10 rounded-lg bg-primary/15 border border-border text-foreground flex items-center justify-center font-bold">
          {n}
        </div>
        <div>
          <div className="text-foreground font-bold text-lg">{title}</div>
          <div className="mt-2 text-muted-foreground leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  );
}

type InstallationParams = { lang?: string };

export default async function InstallationPage({ params }: { params?: Promise<InstallationParams> }) {
  const resolved = await params;
  const lang = resolveUILang(resolved?.lang);
  const isFa = lang === "fa";
  const copy = uiCopy[lang].installationPage;
  const releases = await getLocalReleases();
  const tag = releases[0]?.tag ?? "v0.1.0";
  const resolvedBasePrefix = resolved?.lang === "fa" ? "/fa" : resolved?.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  const linuxAmd64 =
    releases[0]?.artifacts.find((a) => a.role === "inside" && a.target === "linux-amd64" && a.kind === "tar.gz")?.fileName ??
    `sunlionet-inside-${tag}-linux-amd64.tar.gz`;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title={copy.title}
          subtitle={copy.subtitle}
          actions={
            <>
              <Link
                href={hrefFor("/installation/wizard")}
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                {copy.wizardCta ?? "Install wizard"}
              </Link>
              <Link
                href={hrefFor("/download")}
                prefetch={false}
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                {uiCopy[lang].nav.download}
              </Link>
              <a
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                href={githubDocsInstall}
                target="_blank"
                rel="noreferrer"
              >
                {copy.repoInstallDoc}
              </a>
            </>
          }
        />

        <div className="grid gap-6">
          <Step n="1" title={copy.steps.download.title}>
            {copy.steps.download.desc}
            <div className="mt-4">
              <Link
                href={hrefFor("/download")}
                className="text-primary font-bold hover:underline"
              >
                {isFa ? "برو به صفحه دانلود" : "Go to Download Page"} →
              </Link>
            </div>
          </Step>

          <Step n="2" title={copy.steps.android.title}>
            {copy.steps.android.desc}
            <div className="mt-4 text-sm text-muted-foreground italic">
              {isFa ? "پیشنهادی برای کاربران اندروید" : "Recommended for Android users"}
            </div>
          </Step>

          <Step n="3" title={copy.steps.linux.title}>
            {copy.steps.linux.desc}
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`tar -xzf ${linuxAmd64}
sudo ./install-linux.sh inside
sudo systemctl enable --now sunlionet-inside.service`}
              />
            </div>
          </Step>

          <Step n="4" title={copy.steps.seeds.title}>
            {copy.steps.seeds.desc}
            <div className="mt-4">
              <Link
                href={hrefFor("/docs/user/safety")}
                className="text-primary text-sm font-bold hover:underline"
              >
                {isFa ? "مطالعه نکات ایمنی" : "Read Safety Tips"} →
              </Link>
            </div>
          </Step>
        </div>

        <div className="rounded-2xl border border-border bg-card/40 p-8 shadow-[0_0_0_1px_var(--border)] mt-4">
          <h2 className="text-2xl font-bold text-foreground mb-4">{copy.whyNoStore}</h2>
          <p className="text-muted-foreground leading-relaxed">
            {copy.whyNoStoreDesc}
          </p>
        </div>
      </div>
    </div>
  );
}
