import path from "node:path";
import { readdir, readFile, stat } from "node:fs/promises";
import { cache } from "react";

export type ReleaseArtifactKind = "tar.gz" | "zip" | "bin";

export type ReleaseArtifact = {
  fileName: string;
  href: string;
  sizeBytes: number;
  kind: ReleaseArtifactKind;
  sha256?: string;
  sha256Href?: string;
  role?: "inside" | "outside";
  target?: string;
};

export type LocalRelease = {
  tag: string;
  buildRef?: string;
  createdAtUnix?: number;
  artifacts: ReleaseArtifact[];
  verification?: {
    checksumsHref?: string;
    signatureHref?: string;
    keyHref?: string;
    keyFingerprint?: string;
    keyFingerprintHref?: string;
  };
};

function parseSemver(version: string) {
  const m = /^v(\d+)\.(\d+)\.(\d+)$/.exec(version);
  if (!m) return null;
  return { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
}

function compareReleaseAsc(a: string, b: string) {
  const pa = parseSemver(a);
  const pb = parseSemver(b);
  if (!pa || !pb) return a.localeCompare(b);
  if (pa.major !== pb.major) return pa.major - pb.major;
  if (pa.minor !== pb.minor) return pa.minor - pb.minor;
  return pa.patch - pb.patch;
}

function artifactKind(fileName: string): ReleaseArtifactKind {
  if (fileName.endsWith(".tar.gz")) return "tar.gz";
  if (fileName.endsWith(".zip")) return "zip";
  return "bin";
}

function parseRoleAndTarget(tag: string, fileName: string): { role?: "inside" | "outside"; target?: string } {
  const prefixes = ["sunlionet-", "shadownet-"];
  const prefix = prefixes.find((p) => fileName.startsWith(p));
  if (!prefix) return {};
  const rest = fileName.slice(prefix.length);
  const roleMatch = /^(inside|outside)-/.exec(rest);
  if (!roleMatch) return {};
  const role = roleMatch[1] as "inside" | "outside";
  const afterRole = rest.slice(`${role}-`.length);
  if (!afterRole.startsWith(`${tag}-`)) return { role };
  const afterTag = afterRole.slice(`${tag}-`.length);

  const stripped = afterTag.endsWith(".tar.gz")
    ? afterTag.slice(0, -".tar.gz".length)
    : afterTag.endsWith(".zip")
      ? afterTag.slice(0, -".zip".length)
      : afterTag;

  return { role, target: stripped };
}

function parseSha256FileContents(contents: string) {
  const line = contents.split(/\r?\n/).find((l) => l.trim().length);
  if (!line) return null;
  const m = /^([a-fA-F0-9]{64})\s+(\S+)$/.exec(line.trim());
  if (!m) return null;
  return { hash: m[1].toLowerCase(), fileName: m[2] };
}

export const getLocalReleases = cache(async (): Promise<LocalRelease[]> => {
  try {
    const downloadsDir = path.join(process.cwd(), "public", "downloads");
    const entries = await readdir(downloadsDir, { withFileTypes: true });
    const versionDirs = entries
      .filter((e) => e.isDirectory() && e.name.startsWith("v"))
      .map((e) => e.name)
      .sort(compareReleaseAsc);

    const releases: LocalRelease[] = [];
    for (const tag of versionDirs) {
      const dir = path.join(downloadsDir, tag);
      const versionFile = path.join(dir, "VERSION.txt");

      let buildRef: string | undefined = undefined;
      let createdAtUnix: number | undefined = undefined;
      try {
        const raw = (await readFile(versionFile, "utf8")).trim();
        buildRef = raw || undefined;
        createdAtUnix = Math.floor((await stat(versionFile)).mtimeMs / 1000);
      } catch {
        buildRef = undefined;
        createdAtUnix = undefined;
      }

      const files = await readdir(dir, { withFileTypes: true });
      const shaByFile = new Map<string, string>();
      for (const f of files) {
        if (!f.isFile()) continue;
        if (!f.name.endsWith(".sha256")) continue;
        try {
          const parsed = parseSha256FileContents(await readFile(path.join(dir, f.name), "utf8"));
          if (parsed) shaByFile.set(parsed.fileName, parsed.hash);
        } catch {
          continue;
        }
      }

      const artifacts: ReleaseArtifact[] = [];
      for (const f of files) {
        if (!f.isFile()) continue;
        if (f.name === "VERSION.txt") continue;
        if (f.name === "checksums.txt" || f.name === "checksums.sig" || f.name === "checksums.pub" || f.name === "checksums.pub.sha256") continue;
        if (f.name.endsWith(".sha256")) continue;

        const p = path.join(dir, f.name);
        const s = await stat(p);
        const { role, target } = parseRoleAndTarget(tag, f.name);
        artifacts.push({
          fileName: f.name,
          href: `/downloads/${tag}/${f.name}`,
          sizeBytes: s.size,
          kind: artifactKind(f.name),
          sha256: shaByFile.get(f.name),
          sha256Href: shaByFile.has(f.name) ? `/downloads/${tag}/${f.name}.sha256` : undefined,
          role,
          target,
        });
      }

      let keyFingerprint: string | undefined = undefined;
      try {
        const raw = (await readFile(path.join(dir, "checksums.pub.sha256"), "utf8")).trim();
        const m = /^([a-fA-F0-9]{64})\s+(\S+)$/.exec(raw.split(/\r?\n/)[0]?.trim() ?? "");
        keyFingerprint = m?.[1]?.toLowerCase();
      } catch {
        keyFingerprint = undefined;
      }

      releases.push({
        tag,
        buildRef,
        createdAtUnix,
        artifacts,
        verification: {
          checksumsHref: files.some((f) => f.isFile() && f.name === "checksums.txt") ? `/downloads/${tag}/checksums.txt` : undefined,
          signatureHref: files.some((f) => f.isFile() && f.name === "checksums.sig") ? `/downloads/${tag}/checksums.sig` : undefined,
          keyHref: files.some((f) => f.isFile() && f.name === "checksums.pub") ? `/downloads/${tag}/checksums.pub` : undefined,
          keyFingerprint,
          keyFingerprintHref: files.some((f) => f.isFile() && f.name === "checksums.pub.sha256") ? `/downloads/${tag}/checksums.pub.sha256` : undefined,
        },
      });
    }

    return releases.sort((a, b) => compareReleaseAsc(a.tag, b.tag)).reverse();
  } catch {
    return [];
  }
});
