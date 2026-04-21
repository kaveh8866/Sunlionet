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

export default async function InstallationPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolveUILang(resolved.lang);
  const copy = uiCopy[lang].installationPage;
  const releases = await getLocalReleases();
  const tag = releases[0]?.tag ?? "v0.1.0";
  const githubDownloads = `${githubRepo}/tree/main/website/public/downloads/${tag}`;
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
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
          <Step n="1" title="Download + verify">
            Go to{" "}
            <Link href={hrefFor("/download")} prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
              /download
            </Link>{" "}
            and download the artifact plus its <span className="font-mono">.sha256</span> file. Always verify before running.
            <div className="mt-3 text-sm text-muted-foreground">
              These examples assume you set <span className="font-mono">BASE_URL</span> to the website you are using (example:
              <span className="font-mono"> https://sunlionet.example</span>).
            </div>
            <div className="mt-3 text-sm text-muted-foreground">
              You can also browse the exact files on GitHub:{" "}
            <a className="text-primary hover:opacity-90 transition-opacity" href={githubDownloads} target="_blank" rel="noreferrer">
                downloads/{tag}
            </a>
            .
            </div>
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`BASE_URL="https://sunlionet.example"
curl -fL -O "$BASE_URL/downloads/${tag}/${linuxAmd64}"
curl -fL -O "$BASE_URL/downloads/${tag}/${linuxAmd64}.sha256"
sha256sum -c "${linuxAmd64}.sha256"`}
              />
            </div>
          </Step>

          <Step n="2" title="Install (Linux bundle)">
            Extract and install:
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`tar -xzf ${linuxAmd64}
sudo ./install-linux.sh inside
sudo systemctl enable --now sunlionet-inside.service || sudo systemctl enable --now SUNLIONET-inside.service`}
              />
            </div>
            <div className="mt-3 text-sm text-muted-foreground">
              Full install reference (GitHub source):{" "}
              <a className="text-primary hover:opacity-90 transition-opacity" href={githubDocsInstall} target="_blank" rel="noreferrer">
                docs/install.md
              </a>
            </div>
          </Step>

          <Step n="3" title="Install (Android app)">
            Install SunLionet as a signed APK from GitHub Releases, then import a trusted configuration bundle and connect.
            <div className="mt-4 text-sm text-muted-foreground">
              Follow:{" "}
              <Link href={hrefFor("/docs/install/android")} prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
                /docs/install/android
              </Link>
              .
            </div>
          </Step>

          <Step n="4" title="Seeds via Signal (trusted contact)">
            Ask a trusted supporter to run Outside and send you signed + encrypted bundles via Signal. Inside accepts bundles only from
            explicitly trusted publisher keys. No domains are stored, and only a bounded event buffer is retained locally.
          </Step>
        </div>
      </div>
    </div>
  );
}
