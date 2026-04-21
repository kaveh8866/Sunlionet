import { LocalizedLink } from "./LocalizedLink";
import { getLocalReleases } from "../lib/releases/local";

const repoUrl = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");
const licenseUrl = `${repoUrl}/blob/main/LICENSE`;
const buildSha = process.env.NEXT_PUBLIC_GIT_SHA ?? process.env.VERCEL_GIT_COMMIT_SHA ?? process.env.GITHUB_SHA;
const buildShaShort = buildSha ? buildSha.slice(0, 7) : null;
const buildShaUrl = buildSha ? `${repoUrl}/commit/${buildSha}` : null;

export async function SiteFooter() {
  const releases = await getLocalReleases();
  const release = releases[0] ?? null;

  return (
    <footer className="border-t border-border bg-card/30 py-12 mt-16">
      <div className="mx-auto w-full max-w-6xl px-4 grid gap-10">
        <div className="grid gap-8 md:grid-cols-4">
          <div className="md:col-span-2">
            <div className="text-foreground font-semibold tracking-tight">SunLionet</div>
            <p className="mt-3 text-sm text-muted-foreground leading-relaxed max-w-xl">
              Offline-first, privacy-preserving DPI resistance with a dual Inside/Outside architecture. No accounts. No
              analytics. No live seed hosting on this website.
            </p>
            {release ? (
              <div className="mt-4 text-xs font-mono text-muted-foreground">
                Release {release.tag}
                {release.buildRef ? ` · ${release.buildRef}` : null}
              </div>
            ) : null}
            {buildShaUrl && buildShaShort ? (
              <div className="mt-2 text-xs font-mono text-muted-foreground">
                <a href={buildShaUrl} className="hover:text-foreground transition-colors" target="_blank" rel="noreferrer">
                  Source @ {buildShaShort}
                </a>
              </div>
            ) : null}
          </div>

          <div className="grid gap-2 text-sm">
            <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">Docs</div>
            <LocalizedLink href="/docs" prefetch={false} className="text-muted-foreground hover:text-foreground transition-colors">
              Documentation
            </LocalizedLink>
            <LocalizedLink
              href="/docs/install"
              prefetch={false}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Installation
            </LocalizedLink>
            <LocalizedLink
              href="/docs/architecture"
              prefetch={false}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Architecture
            </LocalizedLink>
            <LocalizedLink
              href="/docs/threat-model"
              prefetch={false}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Threat model
            </LocalizedLink>
          </div>

          <div className="grid gap-2 text-sm">
            <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">Project</div>
            <LocalizedLink
              href="/download"
              prefetch={false}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Downloads
            </LocalizedLink>
            <LocalizedLink
              href="/support"
              prefetch={false}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              Support
            </LocalizedLink>
            <a href={repoUrl} className="text-muted-foreground hover:text-foreground transition-colors">
              GitHub
            </a>
            <a href={licenseUrl} className="text-muted-foreground hover:text-foreground transition-colors">
              License (AGPL-3.0)
            </a>
          </div>
        </div>

        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 border-t border-border pt-6">
          <div className="text-xs text-muted-foreground leading-relaxed max-w-3xl">
            Safety note: treat local devices as potentially seized. Prefer minimal logs, verify downloads, and use trusted
            channels for seed delivery.
          </div>
          <div className="text-xs text-muted-foreground">© {new Date().getFullYear()} SunLionet contributors</div>
        </div>
      </div>
    </footer>
  );
}
