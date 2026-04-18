import path from "node:path";
import { readdir, readFile } from "node:fs/promises";
import { LocalizedLink } from "./LocalizedLink";

const repoUrl = "https://github.com/kaveh8866/sunlionet-core";
const licenseUrl = `${repoUrl}/blob/main/LICENSE`;

type ReleaseInfo = {
  version: string;
  buildRef?: string;
};

function parseSemver(version: string) {
  const m = /^v(\d+)\.(\d+)\.(\d+)$/.exec(version);
  if (!m) return null;
  return { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
}

function compareRelease(a: string, b: string) {
  const pa = parseSemver(a);
  const pb = parseSemver(b);
  if (!pa || !pb) return a.localeCompare(b);
  if (pa.major !== pb.major) return pa.major - pb.major;
  if (pa.minor !== pb.minor) return pa.minor - pb.minor;
  return pa.patch - pb.patch;
}

async function getLatestRelease(): Promise<ReleaseInfo | null> {
  try {
    const downloadsDir = path.join(process.cwd(), "public", "downloads");
    const entries = await readdir(downloadsDir, { withFileTypes: true });
    const versions = entries
      .filter((e) => e.isDirectory() && e.name.startsWith("v"))
      .map((e) => e.name)
      .sort(compareRelease);
    const version = versions.at(-1);
    if (!version) return null;

    const versionFile = path.join(downloadsDir, version, "VERSION.txt");
    const buildRef = (await readFile(versionFile, "utf8")).trim() || undefined;
    return { version, buildRef };
  } catch {
    return null;
  }
}

export async function SiteFooter() {
  const release = await getLatestRelease();

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
                Release {release.version}
                {release.buildRef ? ` · ${release.buildRef}` : null}
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
