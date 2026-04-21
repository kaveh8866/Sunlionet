import { NextResponse } from "next/server";
import { getLocalReleases, type LocalRelease, type ReleaseArtifact } from "../../../lib/releases/local";

export const dynamic = "force-dynamic";

type DownloadRole = "inside" | "outside";
type DownloadPlatform =
  | "windows-amd64"
  | "macos-arm64"
  | "macos-amd64"
  | "linux-amd64"
  | "linux-arm64"
  | "raspberrypi-arm64"
  | "android"
  | "ios"
  | "source";

const platforms: DownloadPlatform[] = [
  "windows-amd64",
  "macos-arm64",
  "macos-amd64",
  "linux-amd64",
  "linux-arm64",
  "raspberrypi-arm64",
  "android",
  "ios",
  "source",
];

function findByTarget(release: LocalRelease, role: DownloadRole, target: string) {
  return release.artifacts.find((a) => a.role === role && a.target === target) ?? null;
}

function findAndroidArtifact(release: LocalRelease, role: DownloadRole) {
  if (role !== "inside") return null;
  const apk = release.artifacts.find((a) => a.fileName.toLowerCase().endsWith(".apk"));
  if (apk) return apk;
  return findByTarget(release, "inside", "android-arm64");
}

function pickPlatformArtifact(release: LocalRelease, platform: DownloadPlatform, role: DownloadRole) {
  if (platform === "windows-amd64") return findByTarget(release, role, "windows-amd64");
  if (platform === "macos-arm64") return findByTarget(release, role, "darwin-arm64");
  if (platform === "macos-amd64") return findByTarget(release, role, "darwin-amd64");
  if (platform === "linux-amd64") return findByTarget(release, role, "linux-amd64");
  if (platform === "linux-arm64") return findByTarget(release, role, "linux-arm64");
  if (platform === "raspberrypi-arm64") return findByTarget(release, "inside", "linux-arm64");
  if (platform === "android") return findAndroidArtifact(release, role);
  return null;
}

function toArtifactJson(origin: string, a: ReleaseArtifact) {
  const href = a.href.startsWith("http://") || a.href.startsWith("https://") ? a.href : `${origin}${a.href}`;
  const sha256Href =
    a.sha256Href && (a.sha256Href.startsWith("http://") || a.sha256Href.startsWith("https://"))
      ? a.sha256Href
      : a.sha256Href
        ? `${origin}${a.sha256Href}`
        : null;

  return {
    fileName: a.fileName,
    url: href,
    sizeBytes: a.sizeBytes,
    kind: a.kind,
    role: a.role ?? null,
    target: a.target ?? null,
    sha256: a.sha256 ?? null,
    sha256Url: sha256Href,
  };
}

function abs(origin: string, href?: string) {
  if (!href) return null;
  if (href.startsWith("http://") || href.startsWith("https://")) return href;
  return `${origin}${href}`;
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const origin = url.origin;

  const platformRaw = (url.searchParams.get("platform") ?? "").trim();
  if (platformRaw && !platforms.includes(platformRaw as DownloadPlatform)) {
    return NextResponse.json({ error: "invalid_platform" }, { status: 400 });
  }
  const platform = platformRaw ? (platformRaw as DownloadPlatform) : "";
  const role = ((url.searchParams.get("role") ?? "inside").trim() as DownloadRole) || "inside";

  const releases = await getLocalReleases();
  const latest = releases[0] ?? null;
  const iosAppStoreUrl = process.env.NEXT_PUBLIC_IOS_APPSTORE_URL?.trim() || null;
  const iosTestFlightUrl = process.env.NEXT_PUBLIC_IOS_TESTFLIGHT_URL?.trim() || null;

  if (!latest) {
    return NextResponse.json(
      {
        generatedAtUnix: Math.floor(Date.now() / 1000),
        latest: null,
        error: "no_local_releases",
      },
      { status: 200, headers: { "cache-control": "no-store" } },
    );
  }

  const verification = {
    checksumsUrl: abs(origin, latest.verification?.checksumsHref),
    signatureUrl: abs(origin, latest.verification?.signatureHref),
    cosignKeyUrl: abs(origin, latest.verification?.keyHref),
    cosignKeyFingerprintSha256: latest.verification?.keyFingerprint ?? null,
    cosignKeyFingerprintSha256Url: abs(origin, latest.verification?.keyFingerprintHref),
  };

  if (platform) {
    if (platform === "ios") {
      return NextResponse.json(
        {
          generatedAtUnix: Math.floor(Date.now() / 1000),
          latest: {
            tag: latest.tag,
            buildRef: latest.buildRef ?? null,
            createdAtUnix: latest.createdAtUnix ?? null,
            verification,
          },
          platform: {
            key: "ios",
            role,
            appStoreUrl: iosAppStoreUrl,
            testFlightUrl: iosTestFlightUrl,
            inside: null,
            outside: null,
          },
          artifact: null,
          issues: ["ios_external_links_only"],
        },
        { status: 200, headers: { "cache-control": "no-store" } },
      );
    }

    if (platform === "source") {
      return NextResponse.json(
        {
          generatedAtUnix: Math.floor(Date.now() / 1000),
          latest: {
            tag: latest.tag,
            buildRef: latest.buildRef ?? null,
            createdAtUnix: latest.createdAtUnix ?? null,
            verification,
          },
          platform: { key: "source" },
        },
        { status: 200, headers: { "cache-control": "no-store" } },
      );
    }

    if (platform === "android" && role === "outside") {
      return NextResponse.json(
        {
          generatedAtUnix: Math.floor(Date.now() / 1000),
          latest: {
            tag: latest.tag,
            buildRef: latest.buildRef ?? null,
            createdAtUnix: latest.createdAtUnix ?? null,
            verification,
          },
          platform: { key: platform, role },
          artifact: null,
          issues: ["android_outside_not_supported"],
        },
        { status: 200, headers: { "cache-control": "no-store" } },
      );
    }

    const artifact = pickPlatformArtifact(latest, platform, role);
    if (!artifact) {
      const androidApkMissing =
        platform === "android" && !latest.artifacts.some((a) => a.fileName.toLowerCase().endsWith(".apk")) ? true : false;
      return NextResponse.json(
        {
          generatedAtUnix: Math.floor(Date.now() / 1000),
          latest: {
            tag: latest.tag,
            buildRef: latest.buildRef ?? null,
            createdAtUnix: latest.createdAtUnix ?? null,
            verification,
          },
          platform: { key: platform, role },
          artifact: null,
          issues: [
            androidApkMissing ? "android_apk_missing" : null,
            platform === "macos-amd64" ? "macos_intel_not_published" : null,
          ].filter(Boolean),
        },
        { status: 200, headers: { "cache-control": "no-store" } },
      );
    }

    return NextResponse.json(
      {
        generatedAtUnix: Math.floor(Date.now() / 1000),
        latest: {
          tag: latest.tag,
          buildRef: latest.buildRef ?? null,
          createdAtUnix: latest.createdAtUnix ?? null,
          verification,
        },
        platform: { key: platform, role },
        artifact: toArtifactJson(origin, artifact),
      },
      { status: 200, headers: { "cache-control": "no-store" } },
    );
  }

  const byPlatform = Object.fromEntries(
    platforms.map((p) => {
      if (p === "ios") {
        return [
          p,
          {
            key: p,
            appStoreUrl: iosAppStoreUrl,
            testFlightUrl: iosTestFlightUrl,
            inside: null,
            outside: null,
          },
        ] as const;
      }
      if (p === "source") {
        return [p, { key: p }] as const;
      }

      const inside = pickPlatformArtifact(latest, p, "inside");
      const outside = pickPlatformArtifact(latest, p, "outside");
      return [
        p,
        {
          key: p,
          inside: inside ? toArtifactJson(origin, inside) : null,
          outside: outside ? toArtifactJson(origin, outside) : null,
        },
      ] as const;
    }),
  );

  return NextResponse.json(
    {
      generatedAtUnix: Math.floor(Date.now() / 1000),
      latest: {
        tag: latest.tag,
        buildRef: latest.buildRef ?? null,
        createdAtUnix: latest.createdAtUnix ?? null,
        verification,
      },
      platforms: byPlatform,
      issues: [
        latest.artifacts.some((a) => a.fileName.toLowerCase().endsWith(".apk")) ? null : "android_apk_missing",
        latest.verification?.checksumsHref ? null : "signature_bundle_missing",
      ].filter(Boolean),
    },
    { status: 200, headers: { "cache-control": "no-store" } },
  );
}
