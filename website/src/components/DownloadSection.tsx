"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { getUILangFromPathname, uiCopy } from "../lib/uiCopy";
import { type DetectedOS, useOsDetection } from "./useOsDetection";
import { Callout } from "./ui/Callout";
import { SectionHeader } from "./ui/SectionHeader";

type Role = "inside" | "outside";

type ReleaseArtifact = {
  fileName: string;
  href: string;
  sizeBytes: number;
  kind: "tar.gz" | "zip" | "bin";
  sha256?: string;
  sha256Href?: string;
  role?: "inside" | "outside";
  target?: string;
};

type LocalRelease = {
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

const githubRepo = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");
const repoName = githubRepo.split("/").filter(Boolean).at(-1) ?? "Sunlionet";
const githubReleases = `${githubRepo}/releases`;
const githubTagTarballBase = `${githubRepo}/archive/refs/tags`;
const iosAppStoreUrl = process.env.NEXT_PUBLIC_IOS_APPSTORE_URL?.trim() || "";
const iosTestFlightUrl = process.env.NEXT_PUBLIC_IOS_TESTFLIGHT_URL?.trim() || "";

function formatBytes(sizeBytes: number) {
  const units = ["B", "KB", "MB", "GB"] as const;
  let v = sizeBytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  const p = i === 0 ? 0 : 1;
  return `${v.toFixed(p)} ${units[i]}`;
}

function formatDate(unix?: number) {
  if (!unix) return null;
  try {
    return new Date(unix * 1000).toISOString().slice(0, 10);
  } catch {
    return null;
  }
}

function normalizeUA(s: string) {
  return s.toLowerCase();
}

function psSingleQuoteLiteral(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
}

type DetectedArch = "amd64" | "arm64" | "unknown";

function detectArchFromNavigator(): DetectedArch {
  if (typeof navigator === "undefined") return "unknown";
  const ua = normalizeUA(navigator.userAgent || "");
  const platform = normalizeUA((navigator as unknown as { platform?: string }).platform || "");
  const full = `${ua} ${platform}`;

  if (full.includes("aarch64") || full.includes("arm64") || full.includes("armv8")) return "arm64";
  if (full.includes("x86_64") || full.includes("x64") || full.includes("win64") || full.includes("amd64")) return "amd64";
  return "unknown";
}

function useOrigin() {
  const [origin] = useState<string>(() => (typeof window === "undefined" ? "" : window.location.origin));
  return origin;
}

function CommandBlock({
  label,
  code,
  language,
  note,
  copyLabel,
  copiedLabel,
}: {
  label: string;
  code: string;
  language?: string;
  note?: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  const [copied, setCopied] = useState(false);
  const onCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      setCopied(false);
    }
  }, [code]);

  return (
    <div className="rounded-xl border border-border bg-card/60 shadow-[0_0_0_1px_var(--border)] overflow-hidden">
      <div className="flex items-center justify-between gap-3 border-b border-border bg-card/40 px-3 py-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground uppercase tracking-wide">{label}</div>
        </div>
        <button
          type="button"
          onClick={onCopy}
          className="px-3 py-1.5 rounded-md border border-border bg-card text-xs font-semibold text-foreground hover:opacity-90 transition-opacity"
        >
          {copied ? copiedLabel : copyLabel}
        </button>
      </div>
      <pre className="overflow-auto p-4">
        <code className="text-[0.875rem] leading-relaxed font-mono text-foreground whitespace-pre">{code}</code>
      </pre>
      {language || note ? (
        <div className="border-t border-border bg-card/40 px-3 py-2 text-xs text-muted-foreground">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <span className="font-mono uppercase tracking-wide">{language ? language : "shell"}</span>
            {note ? <span className="min-w-0">{note}</span> : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function CodeBlock({ children }: { children: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-[0_0_0_1px_var(--border)]">
      <pre className="text-xs font-mono text-muted whitespace-pre-wrap break-words">{children}</pre>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={[
        "px-4 py-2 text-sm font-semibold rounded-md border transition-colors",
        active
          ? "bg-primary border-border text-primary-foreground shadow-[0_0_16px_var(--ring)]"
          : "bg-card border-border text-muted hover:text-foreground hover:opacity-90",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

type PlatformKey =
  | "windows-amd64"
  | "macos-arm64"
  | "macos-amd64"
  | "linux-amd64"
  | "linux-arm64"
  | "raspberrypi-arm64"
  | "android"
  | "ios"
  | "source"
  | "unknown";

function platformLabel(k: PlatformKey, lang: "en" | "fa") {
  const isFa = lang === "fa";
  switch (k) {
    case "windows-amd64":
      return isFa ? "ویندوز (Windows) x86_64" : "Windows x86_64";
    case "macos-arm64":
      return isFa ? "مک (macOS) Apple Silicon (arm64)" : "macOS Apple Silicon (arm64)";
    case "macos-amd64":
      return isFa ? "مک (macOS) Intel (x86_64)" : "macOS Intel (x86_64)";
    case "linux-amd64":
      return isFa ? "لینوکس (Linux) x86_64" : "Linux x86_64";
    case "linux-arm64":
      return isFa ? "لینوکس (Linux) arm64" : "Linux arm64";
    case "raspberrypi-arm64":
      return isFa ? "رزبری‌پای (Raspberry Pi) ARM64" : "Raspberry Pi (ARM64)";
    case "android":
      return isFa ? "اندروید (Android)" : "Android";
    case "ios":
      return isFa ? "آی‌اواس (iOS)" : "iOS";
    case "source":
      return isFa ? "کد منبع (Source code)" : "Source code";
    default:
      return isFa ? "نامشخص" : "Unknown";
  }
}

function pickPlatform(os: DetectedOS, arch: DetectedArch): PlatformKey {
  if (os === "android") return "android";
  if (os === "ios") return "ios";
  if (os === "raspberrypi") return "raspberrypi-arm64";
  if (os === "windows") {
    if (arch === "amd64") return "windows-amd64";
    return "unknown";
  }
  if (os === "macos") {
    if (arch === "arm64") return "macos-arm64";
    if (arch === "amd64") return "macos-amd64";
    return "unknown";
  }
  if (os === "linux") {
    if (arch === "arm64") return "linux-arm64";
    if (arch === "amd64") return "linux-amd64";
    return "unknown";
  }
  return "unknown";
}

function findArtifact(release: LocalRelease | null, role: Role, target: string) {
  if (!release) return null;
  return (
    release.artifacts.find((a) => a.role === role && a.target === target) ||
    release.artifacts.find((a) => a.role === role && a.fileName.includes(target))
  );
}

function supportedTargets(release: LocalRelease | null) {
  if (!release) return [];
  return Array.from(new Set(release.artifacts.map((a) => a.target).filter(Boolean))) as string[];
}

function releaseArtifactCount(release: LocalRelease | null) {
  return release ? release.artifacts.length : 0;
}

export function DownloadSection({ releases, basePrefix }: { releases: LocalRelease[]; basePrefix?: string }) {
  const pathname = usePathname();
  const lang = getUILangFromPathname(pathname);
  const copy = uiCopy[lang];
  const isFa = lang === "fa";
  const origin = useOrigin();
  const { detection, supportsAutoRecommendation } = useOsDetection();
  const [hydrated, setHydrated] = useState(false);
  const [arch, setArch] = useState<DetectedArch>("unknown");
  const resolvedBasePrefix = basePrefix?.trim() ? basePrefix : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  const [role, setRole] = useState<Role>("inside");
  const [manualPlatform, setManualPlatform] = useState<PlatformKey>("unknown");
  const [selectedTag, setSelectedTag] = useState<string>(releases[0]?.tag ?? "");

  useEffect(() => {
    const id = window.setTimeout(() => {
      setArch(detectArchFromNavigator());
      setHydrated(true);
    }, 0);
    return () => window.clearTimeout(id);
  }, []);

  const selectedRelease = useMemo(() => releases.find((r) => r.tag === selectedTag) ?? releases[0] ?? null, [releases, selectedTag]);
  const createdAt = formatDate(selectedRelease?.createdAtUnix) ?? "n/a";

  const effectiveAuto = hydrated && supportsAutoRecommendation;
  const autoPlatform = useMemo(
    () => (effectiveAuto ? pickPlatform(detection.os, arch) : "unknown"),
    [effectiveAuto, detection.os, arch],
  );

  const effectivePlatform: PlatformKey = manualPlatform !== "unknown" ? manualPlatform : autoPlatform;

  const recommendedArtifact = useMemo(() => {
    if (!selectedRelease) return null;
    if (effectivePlatform === "windows-amd64") return findArtifact(selectedRelease, role, "windows-amd64");
    if (effectivePlatform === "macos-arm64") return findArtifact(selectedRelease, role, "darwin-arm64");
    if (effectivePlatform === "macos-amd64") return findArtifact(selectedRelease, role, "darwin-amd64");
    if (effectivePlatform === "linux-amd64") return findArtifact(selectedRelease, role, "linux-amd64");
    if (effectivePlatform === "linux-arm64") return findArtifact(selectedRelease, role, "linux-arm64");
    if (effectivePlatform === "raspberrypi-arm64") return findArtifact(selectedRelease, role, "linux-arm64");
    if (effectivePlatform === "android") {
      if (role !== "inside") return null;
      const apk = selectedRelease.artifacts.find((a) => a.fileName.toLowerCase().endsWith(".apk"));
      if (apk) return apk;
      return (
        selectedRelease.artifacts.find((a) => {
          const name = a.fileName.toLowerCase();
          return name === "app-release.apk" || name.endsWith("-app-release.apk");
        }) ??
        findArtifact(selectedRelease, "inside", "android-arm64")
      );
    }
    return null;
  }, [selectedRelease, effectivePlatform, role]);

  const recommendedSupport: { level: "stable" | "experimental" | "planned" | "unsupported"; note: string } = useMemo(() => {
    if (effectivePlatform === "android") {
      if (role !== "inside") {
        return {
          level: "unsupported",
          note: isFa
            ? "نسخه Outside روی اندروید پشتیبانی نمی‌شود. Outside را روی یک دستگاه جدا اجرا کنید."
            : "Outside is not supported on Android. Run Outside on a separate machine.",
        };
      }
      const hasApk = Boolean(recommendedArtifact?.fileName?.toLowerCase()?.endsWith(".apk"));
      return {
        level: "experimental",
        note: hasApk
          ? isFa
            ? "APK اندروید برای نصب دستی موجود است. بعد از دانلود، اندروید هنگام نصب تأیید می‌خواهد و این سایت نصب بی‌صدا انجام نمی‌دهد. قبل از نصب، checksum/امضا را بررسی کنید."
            : "Android APK is available for sideload install. After download, Android will ask you to confirm install and this site cannot install silently. Verify checksums/signature before installing."
          : isFa
            ? "در این بیلد، APK اندروید منتشر نشده است. از مسیر Termux/CLI استفاده کنید یا زمانی که موجود شد، APK را از GitHub Releases دریافت کنید."
            : "Android APK is not published in this build. Use the Termux CLI binary fallback, or download the APK from GitHub Releases when available.",
      };
    }
    if (effectivePlatform === "ios") {
      return {
        level: "planned",
        note: isFa
          ? "در حال حاضر نسخه iOS از این سایت منتشر نمی‌شود. در صورت انتشار، از لینک رسمی App Store/TestFlight استفاده کنید."
          : "iOS builds are not currently published from this site. Use the official App Store/TestFlight link when available.",
      };
    }
    if (effectivePlatform === "windows-amd64") {
      return {
        level: "experimental",
        note: isFa
          ? "بیلد ویندوز موجود است، اما مسیر اصلی فعلاً لینوکس است. قبل از اجرا، checksum را بررسی کنید."
          : "Windows bundles are provided, but the primary supported path is still Linux. Verify checksums before running.",
      };
    }
    if (effectivePlatform === "macos-arm64") {
      return {
        level: "experimental",
        note: isFa
          ? "بیلد macOS برای Apple Silicon (arm64) موجود است. قبل از اجرا، checksum را بررسی کنید."
          : "macOS bundles are available for Apple Silicon (arm64). Verify checksums before running.",
      };
    }
    if (effectivePlatform === "macos-amd64") {
      return {
        level: "unsupported",
        note: isFa
          ? "برای macOS Intel در این بیلد فایل آماده منتشر نشده است. از نسخه Apple Silicon یا ساخت از کد منبع استفاده کنید."
          : "No macOS Intel bundle is published in this build. Use Apple Silicon bundle or build from source.",
      };
    }
    if (effectivePlatform === "raspberrypi-arm64") {
      return {
        level: "experimental",
        note: isFa
          ? "از بیلد لینوکس arm64 استفاده می‌کند. تا تکمیل تست میدانی طولانی‌مدت، آزمایشی محسوب می‌شود."
          : "Uses the Linux arm64 bundle. Treat as experimental until long-running field testing is complete.",
      };
    }
    if (effectivePlatform === "linux-amd64" || effectivePlatform === "linux-arm64") {
      return {
        level: "stable",
        note: isFa
          ? "بیلدهای لینوکس مسیر اصلی فعلی هستند (tarball + اسکریپت نصب + systemd)."
          : "Linux bundles are the current MVP path (tarball + install script + systemd unit).",
      };
    }
    return {
      level: "unsupported",
      note: isFa
        ? "تشخیص خودکار قطعی نیست. از جدول پلتفرم‌ها استفاده کنید و قبل از اجرا تأیید اصالت انجام دهید."
        : "Auto-detection is inconclusive. Use the platform grid and verify before running.",
    };
  }, [effectivePlatform, role, recommendedArtifact?.fileName, isFa]);

  const releasePageUrl = selectedRelease ? `${githubReleases}/tag/${selectedRelease.tag}` : githubReleases;
  const sourceTarballUrl = selectedRelease ? `${githubTagTarballBase}/${selectedRelease.tag}.tar.gz` : `${githubRepo}/archive/refs/heads/main.tar.gz`;

  const isAbsoluteUrl = (href: string) => /^https?:\/\//i.test(href);

  const artifactUrl = useMemo(() => {
    if (!origin || !recommendedArtifact) return null;
    if (isAbsoluteUrl(recommendedArtifact.href)) return recommendedArtifact.href;
    return `${origin}${recommendedArtifact.href}`;
  }, [origin, recommendedArtifact]);

  const shaUrl = useMemo(() => {
    if (!origin || !recommendedArtifact?.sha256Href) return null;
    if (isAbsoluteUrl(recommendedArtifact.sha256Href)) return recommendedArtifact.sha256Href;
    return `${origin}${recommendedArtifact.sha256Href}`;
  }, [origin, recommendedArtifact]);

  const verificationFiles = selectedRelease?.verification;

  const installSteps = useMemo(() => {
    if (!recommendedArtifact || !artifactUrl) return null;
    const file = recommendedArtifact.fileName;
    const isTar = recommendedArtifact.kind === "tar.gz";
    const sigReady = Boolean(verificationFiles?.checksumsHref && verificationFiles?.signatureHref && verificationFiles?.keyHref);

    if (effectivePlatform === "android") {
      const isApk = file.toLowerCase().endsWith(".apk");
      const cliWarning =
        lang === "fa"
          ? "هشدار: این فایل یک برنامه خط فرمان است و APK امضا شده نیست. قبل از اجرا حتماً تأیید اصالت را انجام دهید."
          : "WARNING: This is a CLI binary, not a signed APK. Verify checksum and only run if you trust the source.";
      return {
        language: "bash",
        download: isApk
          ? `wget -O ${file} ${artifactUrl}\nwget -O ${file}.sha256 ${shaUrl ?? "<sha256-url>"}\n${
              verificationFiles?.checksumsHref ? `wget -O checksums.txt ${origin}${verificationFiles.checksumsHref}\n` : ""
            }${verificationFiles?.signatureHref ? `wget -O checksums.sig ${origin}${verificationFiles.signatureHref}\n` : ""}${
              verificationFiles?.keyHref ? `wget -O checksums.pub ${origin}${verificationFiles.keyHref}\n` : ""
            }`
          : `curl -fL -O ${artifactUrl}\ncurl -fL -O ${shaUrl ?? "<sha256-url>"}\n`,
        verify: isApk
          ? `sha256sum -c ${file}.sha256${sigReady ? `\ncosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt` : ""}`
          : `sha256sum -c ${file}.sha256`,
        install: isApk
          ? `adb install -r ${file}`
          : `echo '${cliWarning}'\nchmod +x "${file}"\n./"${file}"`,
      };
    }

    if (effectivePlatform === "linux-amd64" || effectivePlatform === "linux-arm64" || effectivePlatform === "raspberrypi-arm64") {
      const checksumsCmd = verificationFiles?.checksumsHref ? `curl -fL -O ${origin}${verificationFiles.checksumsHref}` : "curl -fL -O <checksums-url>";
      const signatureCmd = verificationFiles?.signatureHref ? `curl -fL -O ${origin}${verificationFiles.signatureHref}` : "curl -fL -O <signature-url>";
      const keyCmd = verificationFiles?.keyHref ? `curl -fL -O ${origin}${verificationFiles.keyHref}` : "curl -fL -O <cosign-pubkey-url>";
      return {
        language: "bash",
        download: `curl -fL -O ${artifactUrl}\ncurl -fL -O ${shaUrl ?? "<sha256-url>"}\n${checksumsCmd}\n${signatureCmd}\n${keyCmd}\n`,
        verify: `sha256sum -c ${file}.sha256\ncosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt`,
        install: isTar ? `tar -xzf ${file}\nsudo ./install-linux.sh ${role}` : `chmod +x ${file}\n./${file}`,
      };
    }

    if (effectivePlatform === "windows-amd64") {
      const psArtifactUrl = psSingleQuoteLiteral(artifactUrl);
      const psShaUrl = psSingleQuoteLiteral(shaUrl ?? "<sha256-url>");
      const psFile = psSingleQuoteLiteral(file);
      const psShaFile = psSingleQuoteLiteral(`${file}.sha256`);
      const psDest = psSingleQuoteLiteral(`.\\sunlionet-${role}`);
      return {
        language: "powershell",
        download:
          `$artifactUrl = ${psArtifactUrl}\n` +
          `$shaUrl = ${psShaUrl}\n` +
          `$file = ${psFile}\n` +
          `$shaFile = ${psShaFile}\n` +
          `Invoke-WebRequest -Uri $artifactUrl -OutFile $file\n` +
          `Invoke-WebRequest -Uri $shaUrl -OutFile $shaFile\n`,
        verify:
          `$expected = (Get-Content $shaFile).Split(" ")[0].ToLower()\n` +
          `$actual = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLower()\n` +
          `if ($actual -ne $expected) { throw "SHA256 mismatch: expected $expected, got $actual" }\n`,
        install: `$dest = ${psDest}\nExpand-Archive -Path $file -DestinationPath $dest -Force`,
      };
    }

    if (effectivePlatform === "macos-arm64" || effectivePlatform === "macos-amd64") {
      return {
        language: "bash",
        download: `curl -fL -O ${artifactUrl}\ncurl -fL -O ${shaUrl ?? "<sha256-url>"}\n`,
        verify: `shasum -a 256 -c ${file}.sha256`,
        install: isTar ? `tar -xzf ${file}\n./sunlionet-${role} || ./SUNLIONET-${role}` : `./${file}`,
      };
    }

    return null;
  }, [recommendedArtifact, artifactUrl, shaUrl, effectivePlatform, role, origin, verificationFiles, lang]);

  const hasLocalReleases = releases.length > 0;

  const recommendedHeading = useMemo(() => {
    if (lang === "fa") {
      if (!hydrated) return "دانلود پیشنهادی";
      if (!supportsAutoRecommendation) return "دانلود پیشنهادی (انتخاب دستی)";
      if (autoPlatform === "unknown") return "دانلود پیشنهادی (تشخیص قطعی نیست)";
      return "دانلود پیشنهادی";
    }
    if (!hydrated) return "Recommended download";
    if (!supportsAutoRecommendation) return "Recommended download (manual selection)";
    if (autoPlatform === "unknown") return "Recommended download (detection is uncertain)";
    return "Recommended download";
  }, [supportsAutoRecommendation, autoPlatform, lang, hydrated]);

  const platformChoices: { key: PlatformKey; description: string; support: string; method: string; target?: string }[] = useMemo(() => {
    const isFa = lang === "fa";
    const support = {
      experimental: isFa ? "آزمایشی" : "Experimental",
      planned: isFa ? "برنامه‌ریزی‌شده" : "Planned",
      notPublished: isFa ? "منتشر نشده" : "Not published",
      mvp: isFa ? "مسیر اصلی فعلی" : "MVP path",
      mvpArm: isFa ? "مسیر اصلی فعلی (arm64)" : "MVP path (arm64)",
      available: isFa ? "همیشه در دسترس" : "Always available",
    };

    return [
      {
        key: "windows-amd64",
        description: isFa
          ? "بسته ویندوز (zip). فعلاً بیشتر برای تست/توسعه؛ مسیر اصلی همچنان لینوکس است."
          : "Windows x86_64 bundle (zip). Intended for development/testing while Linux remains the primary supported path.",
        support: support.experimental,
        method: isFa ? "ZIP + بررسی SHA256 + استخراج" : "ZIP + verify SHA256 + unzip",
        target: "windows-amd64",
      },
      {
        key: "macos-arm64",
        description: isFa ? "بسته macOS برای Apple Silicon (arm64)." : "macOS Apple Silicon (arm64) bundle (tarball).",
        support: support.experimental,
        method: isFa ? "Tarball + بررسی SHA256" : "Tarball + verify SHA256",
        target: "darwin-arm64",
      },
      {
        key: "macos-amd64",
        description: isFa
          ? "برای macOS Intel فایل آماده منتشر نشده است (از Apple Silicon یا ساخت از کد منبع استفاده کنید)."
          : "macOS Intel bundles are not currently published (use source build or Apple Silicon bundle).",
        support: support.notPublished,
        method: isFa ? "ساخت از کد منبع" : "Source build",
        target: "darwin-amd64",
      },
      {
        key: "linux-amd64",
        description: isFa ? "بسته لینوکس x86_64 با اسکریپت نصب و unit برای systemd." : "Static bundle for Linux x86_64. Includes install script + systemd unit.",
        support: support.mvp,
        method: isFa ? "Tarball + بررسی SHA256 + اسکریپت نصب" : "Tarball + verify SHA256 + install script",
        target: "linux-amd64",
      },
      {
        key: "linux-arm64",
        description: isFa ? "بسته لینوکس arm64 (سرورها/SBCها) با اسکریپت نصب و systemd." : "Static bundle for Linux arm64 (servers, SBCs). Includes install script + systemd unit.",
        support: support.mvpArm,
        method: isFa ? "Tarball + بررسی SHA256 + اسکریپت نصب" : "Tarball + verify SHA256 + install script",
        target: "linux-arm64",
      },
      {
        key: "raspberrypi-arm64",
        description: isFa ? "از بسته لینوکس arm64 برای Inside استفاده می‌کند (حالت gateway)." : "Uses the Linux arm64 Inside bundle (gateway use).",
        support: support.experimental,
        method: isFa ? "Tarball + بررسی SHA256 + اسکریپت نصب" : "Tarball + verify SHA256 + install script",
        target: "linux-arm64",
      },
      {
        key: "android",
        description: isFa ? "نصب APK امضا شده (در صورت انتشار). مسیر Termux فقط برای کاربران فنی." : "Signed APK sideload flow (when published). Optional Termux CLI fallback.",
        support: support.experimental,
        method: isFa ? "APK + بررسی SHA256/امضا + نصب دستی" : "APK + verify SHA256/signature + sideload",
      },
      {
        key: "ios",
        description: isFa ? "در حال حاضر از این سایت منتشر نمی‌شود (App Store/TestFlight در آینده)." : "iOS builds are not currently published from this site (App Store/TestFlight planned).",
        support: support.planned,
        method: "App Store/TestFlight",
      },
      {
        key: "source",
        description: isFa ? "ساخت از کد منبع (بهترین گزینه وقتی پلتفرم شما پوشش داده نشده است)." : "Build from source (best fallback when your platform is not covered).",
        support: support.available,
        method: isFa ? "git clone یا tarball + go build" : "git clone or tag tarball + go build",
      },
    ];
  }, [lang]);

  const effectivePlatformLabel = effectivePlatform === "unknown" ? (lang === "fa" ? "انتخاب پلتفرم" : "Choose a platform") : platformLabel(effectivePlatform, lang);
  const detectionSummary = effectiveAuto
    ? `${detection.label}${arch !== "unknown" ? ` • ${arch}` : ""}`
    : `${lang === "fa" ? "نامشخص" : "Unknown"}${arch !== "unknown" ? ` • ${arch}` : ""}`;

  const hasChecksum = Boolean(recommendedArtifact?.sha256 && recommendedArtifact?.sha256Href);
  const hasSignature = Boolean(verificationFiles?.checksumsHref && verificationFiles?.signatureHref && verificationFiles?.keyHref);

  const missingArtifactMessage = useMemo(() => {
    if (!selectedRelease) return "No release metadata is available in this build.";
    const platform =
      effectivePlatform === "unknown"
        ? lang === "fa"
          ? "پلتفرم نامشخص"
          : "Unknown platform"
        : platformLabel(effectivePlatform, lang);
    const targets = supportedTargets(selectedRelease);
    const available = targets.length ? `Available targets: ${targets.join(", ")}.` : "No targets are available in this release.";
    if (effectivePlatform === "ios") {
      return "No iOS installer is published in this build. Use the official App Store/TestFlight link when available, or build from source where applicable.";
    }
    if (effectivePlatform === "macos-amd64") {
      return `No matching artifact for ${platform} (${role}). This build does not publish a macOS Intel bundle. ${available} Use source build as fallback.`;
    }
    return `No matching artifact for ${platform} (${role}). ${available} Use source build or choose a different platform from the grid.`;
  }, [selectedRelease, effectivePlatform, role, lang]);

  return (
    <section id="downloads" className="w-full">
      <div className="grid gap-10">
        {!hasLocalReleases ? (
          <Callout title={lang === "fa" ? "نسخه محلی پیدا نشد" : "No local release artifacts found"} tone="warning">
            {lang === "fa" ? (
              <>
                در این بیلد فایل‌های مسیر <span className="font-mono">website/public/downloads</span> موجود نیست. هنوز می‌توانید از GitHub
                Releases دانلود کنید و checksum را بررسی کنید.
              </>
            ) : (
              <>
                This site build does not include any files under <span className="font-mono">website/public/downloads</span>. You can still
                download from GitHub Releases and verify with published checksums when available.
              </>
            )}
          </Callout>
        ) : null}

        <Callout title={lang === "fa" ? "وضعیت پروژه" : "Project status"} tone="warning">
          {lang === "fa"
            ? "SunLionet فعلاً نسخه Beta (در حد MVP) است. مسیر اصلی پشتیبانی‌شده بسته‌های Linux است. انتشار اندروید ممکن است شامل APK امضا شده باشد. قبل از نصب/اجرا، checksum و در صورت وجود فایل‌های امضاشده را حتماً بررسی کنید."
            : "SunLionet is currently Beta (MVP-level). Linux bundles (`.tar.gz` + `.deb`) are the primary supported path. Android releases may include a signed APK. Before installing/running, always verify checksums and (when published) the signed checksum bundle."}
        </Callout>

        <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <SectionHeader
            title={recommendedHeading}
            subtitle={
              <>
                {isFa
                  ? "فایل مناسب دستگاه خود را انتخاب کنید، تأیید اصالت را بررسی کنید، سپس نصب/اجرا را مرحله‌به‌مرحله انجام دهید. تشخیص خودکار محافظه‌کار است و هیچ گزینه‌ای را پنهان نمی‌کند."
                  : "Pick the correct artifact for your machine, verify it, then install in steps. Auto-detection is conservative and never hides other options."}
              </>
            }
            actions={
              <>
                <select
                  value={selectedTag}
                  onChange={(e) => setSelectedTag(e.target.value)}
                  data-testid="download-release-select"
                  className="bg-card border border-border rounded-md px-3 py-2 text-sm text-foreground"
                >
                  {releases.map((r) => (
                    <option key={r.tag} value={r.tag}>
                      {r.tag}
                    </option>
                  ))}
                </select>
                <TabButton active={role === "inside"} onClick={() => setRole("inside")}>
                  Inside
                </TabButton>
                <TabButton active={role === "outside"} onClick={() => setRole("outside")}>
                  Outside
                </TabButton>
              </>
            }
          />

          <div className="mt-6 grid gap-4 md:grid-cols-3">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-xs text-muted-foreground uppercase tracking-wider">{isFa ? "تشخیص" : "Detected"}</div>
              <div className="mt-2 text-foreground font-semibold">{detectionSummary}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {supportsAutoRecommendation ? (
                  hydrated ? (
                    <>
                      {isFa ? "اطمینان" : "Confidence"}: <span className="font-mono">{Math.round(detection.confidence * 100)}%</span>
                    </>
                  ) : (
                    <>
                      {isFa ? "اطمینان" : "Confidence"}: <span className="font-mono">—</span>
                    </>
                  )
                ) : (
                  <>{isFa ? "در این محیط، تشخیص قطعی نیست." : "Detection is inconclusive in this environment."}</>
                )}
              </div>
            </div>

            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-xs text-muted-foreground uppercase tracking-wider">{isFa ? "نسخه" : "Release"}</div>
              <div className="mt-2 text-foreground font-semibold font-mono">{selectedRelease ? selectedRelease.tag : "n/a"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa ? "تاریخ" : "Date"}: <span className="font-mono">{createdAt}</span>
              </div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa ? "فایل‌ها" : "Artifacts"}: <span className="font-mono">{releaseArtifactCount(selectedRelease)}</span>
              </div>
              <div className="mt-3 text-sm">
                <a className="text-primary hover:opacity-90 transition-opacity" href={releasePageUrl} target="_blank" rel="noreferrer">
                  {isFa ? "مشاهده در گیت‌هاب" : "View on GitHub"}
                </a>
              </div>
            </div>

            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-xs text-muted-foreground uppercase tracking-wider">{isFa ? "پلتفرم" : "Platform"}</div>
              <div className="mt-2 text-foreground font-semibold">{effectivePlatformLabel}</div>
              <div className="mt-3">
                <select
                  value={manualPlatform}
                  onChange={(e) => setManualPlatform(e.target.value as PlatformKey)}
                  data-testid="download-platform-select"
                  data-hydrated={hydrated ? "1" : "0"}
                  className="w-full bg-card border border-border rounded-md px-3 py-2 text-sm text-foreground"
                >
                  <option value="unknown">
                    {supportsAutoRecommendation ? (isFa ? "خودکار (پیشنهادی)" : "Auto (recommended)") : isFa ? "انتخاب…" : "Select…"}
                  </option>
                  {platformChoices.map((p) => (
                    <option key={p.key} value={p.key}>
                      {platformLabel(p.key, lang)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="mt-2 text-sm text-muted-foreground">{recommendedSupport.note}</div>
            </div>
          </div>

          <div className="mt-6 grid gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-sm font-semibold text-foreground">{isFa ? "فایل پیشنهادی" : "Recommended artifact"}</div>
              {recommendedArtifact ? (
                <>
                  <div className="mt-2 text-xs text-muted-foreground uppercase tracking-wide">{isFa ? "فایل" : "File"}</div>
                  <div className="mt-1 font-mono text-sm text-foreground break-all">{recommendedArtifact.fileName}</div>
                  <div className="mt-2 text-sm text-muted-foreground">
                    {isFa ? "نوع" : "Type"}: <span className="font-mono">{recommendedArtifact.kind}</span> • {isFa ? "حجم" : "Size"}:{" "}
                    <span className="font-mono">{formatBytes(recommendedArtifact.sizeBytes)}</span>
                  </div>
                  {effectivePlatform === "android" && !recommendedArtifact.fileName.toLowerCase().endsWith(".apk") ? (
                    <div className="mt-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-100">
                      {isFa
                        ? "برای این نسخه، APK اندروید منتشر نشده است. این دانلود مربوط به باینری خط فرمان Termux (android-arm64) است."
                        : "Android APK is not published for this release. This download is the Termux CLI binary (android-arm64)."}
                    </div>
                  ) : null}
                  <div className="mt-3 flex items-center gap-2 flex-wrap">
                    <a
                      href={recommendedArtifact.href}
                      data-testid="recommended-download"
                      className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
                      download
                    >
                      {isFa ? "دانلود" : "Download"}
                    </a>
                    {recommendedArtifact.sha256Href ? (
                      <a
                        href={recommendedArtifact.sha256Href}
                        className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                        download
                      >
                        SHA256
                      </a>
                    ) : null}
                    {verificationFiles?.signatureHref ? (
                      <a
                        href={verificationFiles.signatureHref}
                        className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                        download
                      >
                        {isFa ? "امضا" : "Signature"}
                      </a>
                    ) : null}
                    {verificationFiles?.keyHref ? (
                      <a
                        href={verificationFiles.keyHref}
                        className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                        download
                      >
                        {isFa ? "کلید Cosign" : "Cosign key"}
                      </a>
                    ) : null}
                    {!isAbsoluteUrl(recommendedArtifact.href) ? (
                      <a
                        href={`${githubRepo}/blob/main/website/public${recommendedArtifact.href}`}
                        className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                        target="_blank"
                        rel="noreferrer"
                      >
                        {isFa ? "مشاهده در مخزن" : "Browse source"}
                      </a>
                    ) : null}
                  </div>
                  {effectivePlatform === "android" && recommendedArtifact.fileName.toLowerCase().endsWith(".apk") ? (
                    <div
                      className="mt-4 rounded-lg border border-border bg-card/40 p-4 text-sm text-muted-foreground"
                      data-testid="android-install-handoff"
                    >
                      {isFa ? (
                        <>
                          بعد از دانلود، روی فایل APK بزنید تا نصب‌کننده اندروید باز شود. اندروید هنگام نصب از شما تأیید می‌خواهد (این سایت نصب بی‌صدا انجام
                          نمی‌دهد). پس از نصب، برنامه را باز کنید و مجوز VPN را تأیید کنید.
                        </>
                      ) : (
                        <>
                          After download, tap the APK to open Android’s installer. Android will ask you to confirm install (this site cannot install
                          silently). After install, open the app and approve the VPN permission prompt.
                        </>
                      )}
                    </div>
                  ) : null}
                  <div className="mt-4 text-sm text-muted-foreground">
                    {isFa ? "تأیید اصالت" : "Verification"}:{" "}
                    {hasChecksum ? (
                      <span className="text-emerald-300">{isFa ? "checksum موجود است" : "checksum available"}</span>
                    ) : (
                      <span className="text-amber-300">{isFa ? "checksum موجود نیست" : "checksum missing"}</span>
                    )}{" "}
                    • {isFa ? "امضا" : "Signature"}:{" "}
                    {hasSignature ? (
                      <span className="text-emerald-300">{isFa ? "منتشر شده (cosign)" : "published (cosign)"}</span>
                    ) : (
                      <span className="text-amber-300">{isFa ? "موجود نیست" : "missing"}</span>
                    )}
                  </div>
                  {recommendedArtifact.sha256 ? (
                    <div className="mt-2 text-xs text-muted-foreground">
                      SHA256: <span className="font-mono break-all text-foreground">{recommendedArtifact.sha256}</span>
                    </div>
                  ) : null}
                  {verificationFiles?.keyFingerprint ? (
                    <div className="mt-2 text-xs text-muted-foreground">
                      {isFa ? "اثر انگشت کلید Cosign" : "Cosign key fingerprint"}:{" "}
                      <span className="font-mono break-all text-foreground">{verificationFiles.keyFingerprint}</span>
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="mt-3 text-sm text-muted-foreground">
                  {missingArtifactMessage}
                  {effectivePlatform === "ios" && (iosAppStoreUrl || iosTestFlightUrl) ? (
                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      {iosAppStoreUrl ? (
                        <a
                          className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
                          href={iosAppStoreUrl}
                          target="_blank"
                          rel="noreferrer"
                        >
                          App Store
                        </a>
                      ) : null}
                      {iosTestFlightUrl ? (
                        <a
                          className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                          href={iosTestFlightUrl}
                          target="_blank"
                          rel="noreferrer"
                        >
                          TestFlight
                        </a>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              )}
            </div>

            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-sm font-semibold text-foreground">{isFa ? "نصب سریع (مرحله‌به‌مرحله)" : "Quick install (stepwise)"}</div>
              {installSteps ? (
                <div className="mt-3 grid gap-3">
                  <CommandBlock
                    label={lang === "fa" ? "1) دانلود" : "1) Download"}
                    code={installSteps.download}
                    language={installSteps.language}
                    note={lang === "fa" ? "هم فایل اصلی و هم checksum را بگیرید" : "Fetch both the artifact and its checksum file"}
                    copyLabel={copy.buttons.copy}
                    copiedLabel={copy.buttons.copied}
                  />
                  <CommandBlock
                    label={lang === "fa" ? "2) بررسی" : "2) Verify"}
                    code={installSteps.verify}
                    language={installSteps.language}
                    note={lang === "fa" ? "در صورت تغییر یا خرابی فایل شکست می‌خورد" : "Fails if the download was modified or corrupted"}
                    copyLabel={copy.buttons.copy}
                    copiedLabel={copy.buttons.copied}
                  />
                  <CommandBlock
                    label={lang === "fa" ? "3) نصب / اجرا" : "3) Install / run"}
                    code={installSteps.install}
                    language={installSteps.language}
                    note={
                      lang === "fa"
                        ? "برای بسته‌های Linux سرویس نصب می‌شود"
                        : installSteps.language === "powershell"
                          ? "Unzips the bundle on Windows"
                          : "Installs a service on Linux bundles"
                    }
                    copyLabel={copy.buttons.copy}
                    copiedLabel={copy.buttons.copied}
                  />
                </div>
              ) : (
                <div className="mt-3 text-sm text-muted-foreground">
                  {isFa
                    ? "برای ساخت فرمان‌ها یک پلتفرم پشتیبانی‌شده را انتخاب کنید، یا از مسیر ساخت از کد منبع در پایین استفاده کنید."
                    : "Select a supported platform to generate commands, or use the source build path below."}
                </div>
              )}
            </div>
          </div>
        </div>

        <div>
          <SectionHeader
            title={isFa ? "جدول پلتفرم‌ها" : "Platform grid"}
            subtitle={
              isFa
                ? "همه فایل‌های اصلی و روش پیشنهادی. هیچ چیز پنهان نیست؛ همیشه می‌توانید پیشنهاد را دستی تغییر دهید."
                : "All primary artifacts and recommended methods. Nothing is hidden; you can always override the recommendation."
            }
          />
          <div className="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {platformChoices.map((p) => {
              const a =
                p.key === "source"
                  ? null
                  : p.key === "android"
                    ? (
                        role === "inside"
                          ? (selectedRelease?.artifacts.find((artifact) => artifact.fileName.toLowerCase().endsWith(".apk")) ??
                              selectedRelease?.artifacts.find(
                                (artifact) => {
                                  const name = artifact.fileName.toLowerCase();
                                  return name === "app-release.apk" || name.endsWith("-app-release.apk");
                                },
                              ) ??
                              findArtifact(selectedRelease, "inside", "android-arm64"))
                          : null
                      )
                    : p.key === "raspberrypi-arm64"
                      ? findArtifact(selectedRelease, "inside", "linux-arm64")
                      : p.target
                        ? findArtifact(selectedRelease, role, p.target)
                        : null;

              const title = platformLabel(p.key, lang);
              const support = p.support;
              const method = p.method;

              return (
                <div key={p.key} className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="text-foreground font-bold">{title}</div>
                      <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{p.description}</div>
                    </div>
                    <button
                      type="button"
                      onClick={() => setManualPlatform(p.key)}
                      className="px-3 py-2 rounded-md border border-border bg-card text-xs font-semibold text-foreground hover:opacity-90 transition-opacity"
                    >
                      {isFa ? "انتخاب" : "Select"}
                    </button>
                  </div>

                  <div className="mt-4 grid gap-2 text-sm">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-muted-foreground">{isFa ? "پشتیبانی" : "Support"}</span>
                      <span className="font-mono text-foreground">{support}</span>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-muted-foreground">{isFa ? "روش" : "Method"}</span>
                      <span className="font-mono text-foreground">{method}</span>
                    </div>
                  </div>

                  {p.key === "source" ? (
                    <div className="mt-5 grid gap-3">
                      <a
                        href={sourceTarballUrl}
                        className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)] text-center"
                      >
                        {isFa ? "دانلود سورس (tarball)" : "Download source tarball"}
                      </a>
                      <a
                        href={githubRepo}
                        className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border text-center"
                        target="_blank"
                        rel="noreferrer"
                      >
                        {isFa ? "مخزن" : "Repository"}
                      </a>
                    </div>
                  ) : (
                    <div className="mt-5 grid gap-3">
                      {a ? (
                        <>
                          <a
                            href={a.href}
                            className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)] text-center"
                            download
                          >
                            {isFa ? "دانلود" : "Download"}
                          </a>
                          <div className="text-xs text-muted-foreground break-all">
                            <span className="font-mono">{a.fileName}</span> • <span className="font-mono">{formatBytes(a.sizeBytes)}</span>
                          </div>
                          {a.sha256Href ? (
                            <a
                              href={a.sha256Href}
                              className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border text-center"
                              download
                            >
                              {isFa ? "تأیید اصالت (SHA256)" : "Verification (SHA256)"}
                            </a>
                          ) : (
                            <div className="text-sm text-amber-300">
                              {isFa ? "برای این فایل، checksum منتشر نشده است." : "No checksum file present for this artifact."}
                            </div>
                          )}
                        </>
                      ) : (
                        <div className="text-sm text-muted-foreground">
                          {isFa
                            ? "برای این نقش (Inside/Outside) در این نسخه فایلی موجود نیست. از ساخت از سورس استفاده کنید یا نقش/پلتفرم را تغییر دهید."
                            : "No artifact available for this role in this release. Use source build or switch role/platform."}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>

        <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <SectionHeader
            title={isFa ? "تأیید اصالت" : "Verification"}
            subtitle={
              <>
                {isFa
                  ? "تأیید اصالت از شما در برابر خرابی یا دستکاری هنگام انتقال محافظت می‌کند، اما به‌تنهایی نرم‌افزار را «ایمن» نمی‌کند."
                  : "Verification protects you against corruption and tampering in transit. It does not magically make software “safe”."}
              </>
            }
          />
          <div className="mt-6 grid gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-sm font-semibold text-foreground">{isFa ? "SHA256 چه چیزی را به شما می‌دهد؟" : "What the SHA256 check gives you"}</div>
              <ul className="mt-3 text-sm text-muted-foreground space-y-2">
                <li>
                  {isFa
                    ? "- یکپارچگی فایل: دقیقاً همان بایتی را دارید که منتشرکننده قصد انتشار داشته است."
                    : "- File integrity: you got exactly the bytes the publisher intended to publish."}
                </li>
                <li>
                  {isFa
                    ? "- کشف دستکاری: اگر mirror/CDN فایل را تغییر دهد، بررسی شکست می‌خورد."
                    : "- Tamper detection: if a mirror/CDN modifies the file, the check fails."}
                </li>
                <li>
                  {isFa
                    ? "- یک مقدار هش ثابت برای مقایسه بین چند منبع/دوستان."
                    : "- A consistent hash value you can compare across mirrors and friends."}
                </li>
                <li>
                  {isFa
                    ? "- تأیید امضا (`cosign verify-blob`) نشان می‌دهد فایل checksum توسط کلید انتشار مورد اعتماد امضا شده است."
                    : "- Signature validation (`cosign verify-blob`) confirms checksums were signed by a trusted release key."}
                </li>
              </ul>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-sm font-semibold text-foreground">{isFa ? "چه چیزی را تضمین نمی‌کند؟" : "What it does not guarantee"}</div>
              <ul className="mt-3 text-sm text-muted-foreground space-y-2">
                <li>
                  {isFa ? "- هیچ تضمینی برای ناشناس‌بودن یا امنیت در مدل تهدید شما نیست." : "- No guarantee of anonymity or safety in your threat model."}
                </li>
                <li>
                  {isFa
                    ? "- هیچ تضمینی برای بدون‌باگ بودن یا تناسب با ریسک قانونی محلی وجود ندارد."
                    : "- No guarantee the binary is bug-free or appropriate for your local legal risk."}
                </li>
                <li>
                  {isFa
                    ? "- اگر مهاجم هم به فایل و هم به منبع checksum دسترسی داشته باشد، checksum به‌تنهایی کافی نیست."
                    : "- If an attacker controls both the binary and checksum source, checksums alone are not enough."}
                </li>
              </ul>
            </div>
          </div>

          <details className="mt-6 rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
            <summary className="cursor-pointer text-foreground font-semibold">{isFa ? "فرمان‌های مخصوص هر پلتفرم" : "Platform-specific commands"}</summary>
            <div className="mt-4 grid gap-4">
              <div>
                <div className="text-xs text-muted-foreground uppercase tracking-wide">Linux</div>
                <CodeBlock>{`sha256sum -c <file>.sha256
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt`}</CodeBlock>
              </div>
              <div>
                <div className="text-xs text-muted-foreground uppercase tracking-wide">macOS</div>
                <CodeBlock>{`shasum -a 256 -c <file>.sha256`}</CodeBlock>
              </div>
              <div>
                <div className="text-xs text-muted-foreground uppercase tracking-wide">Windows (PowerShell)</div>
                <CodeBlock>{`certutil -hashfile <file> SHA256`}</CodeBlock>
              </div>
              <div className="text-sm text-muted-foreground">
                {isFa ? "برای اصالت bundle (امضاهای ناشر که توسط agent بررسی می‌شود) به" : "For bundle authenticity (publisher signatures enforced by the agent), see"}{" "}
                <Link href={hrefFor("/docs/outside/verification")} prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
                  /docs/outside/verification
                </Link>
                .
              </div>
            </div>
          </details>
        </div>

        <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <SectionHeader
            title={isFa ? "مسیرهای نصب" : "Installation paths"}
            subtitle={isFa ? "مسیر مناسب سطح مهارت و ریسک‌پذیری خود را انتخاب کنید." : "Choose the flow that matches your skill level and risk tolerance."}
          />
          <div className="mt-6 grid gap-4 md:grid-cols-3">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "مسیر سریع" : "Quick path"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa
                  ? "از فایل پیشنهادی و مراحل بالا استفاده کنید. مناسب کاربرانی که همچنان checksum را بررسی می‌کنند."
                  : "Use the recommended artifact and the stepwise commands above. Intended for experienced users who still verify checksums."}
              </div>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "مسیر دقیق و تأیید شده" : "Careful verified path"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa
                  ? "SHA256 را بررسی کنید، در صورت وجود امضا، `checksums.txt` را با cosign تأیید کنید و اثرانگشت کلید را از یک کانال دومِ مورد اعتماد هم چک کنید."
                  : "Verify SHA256, verify `checksums.txt` signature with cosign, and cross-check key fingerprint from a second trusted channel."}
              </div>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "مسیر ساخت از سورس" : "Source build path"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa
                  ? "اگر پلتفرم شما پوشش داده نشده است، از سورس بسازید. برای build tagها و جزئیات ابزار Go، از مستندات مخزن استفاده کنید."
                  : "Build from source if your platform is not covered. Use the repository-backed docs for build tags and Go toolchain details."}
              </div>
              <div className="mt-4">
                <CommandBlock
                  label={lang === "fa" ? "ساخت (نمونه)" : "Build (example)"}
                  language="bash"
                  code={`git clone ${githubRepo}\ncd ${repoName}\nmkdir -p bin\ngo build -tags inside -ldflags="-s -w -X main.version=v0.1.0" -o bin/sunlionet-inside ./cmd/inside/\ngo build -tags outside -ldflags="-s -w -X main.version=v0.1.0" -o bin/sunlionet-outside ./cmd/outside/\n`}
                  note={lang === "fa" ? "نیازمند Go مطابق نسخه پین‌شده در go.mod" : "Requires Go toolchain as pinned in go.mod"}
                  copyLabel={copy.buttons.copy}
                  copiedLabel={copy.buttons.copied}
                />
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <SectionHeader
            title={isFa ? "محدودیت‌ها / آمادگی" : "Limitations / readiness"}
            subtitle={isFa ? "محدودیت‌های واقعی تا کاربر تصمیم آگاهانه بگیرد." : "Truthful constraints so users can make informed decisions."}
          />
          <div className="mt-6 grid gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "چیزهایی که امروز کار می‌کند" : "What works today"}</div>
              <ul className="mt-3 text-sm text-muted-foreground space-y-2">
                <li>- Linux bundles with install script + systemd service (Inside/Outside).</li>
                <li>- Linux `.deb` packaging for amd64 hosts.</li>
                <li>- Bundle signature verification and strict parsing in the agent (trust is local and explicit).</li>
                <li>- Android signed release APK and Android/Termux Inside CLI path.</li>
              </ul>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "چیزهایی که هنوز آزمایشی است" : "What is still experimental"}</div>
              <ul className="mt-3 text-sm text-muted-foreground space-y-2">
                <li>- Optional RPM packaging (builds only when rpm tooling is available).</li>
                <li>- Full multi-maintainer release-signing policy and rotation tooling.</li>
                <li>- Broader cross-device Android validation across OEMs/ROM variants.</li>
              </ul>
            </div>
          </div>
        </div>

        <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <SectionHeader
            title={isFa ? "منابع جایگزین" : "Fallback sources"}
            subtitle={
              isFa
                ? "اگر فایل یا متادیتا موجود نبود، این‌ها قدم‌های بعدیِ امن هستند."
                : "If a platform artifact or metadata is missing, these are the safe next steps."
            }
          />
          <div className="mt-6 grid gap-4 md:grid-cols-3">
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "سورس (tarball)" : "Source tarball"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa ? "وقتی باینری مناسب نیست، از سورس نسخه‌دار استفاده کنید." : "Use a tagged source tarball when binaries are not suitable."}
              </div>
              <div className="mt-4">
                <a
                  href={sourceTarballUrl}
                  className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)] text-center block"
                >
                  {selectedRelease ? `Download ${selectedRelease.tag}.tar.gz` : "Download source"}
                </a>
              </div>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "مخزن" : "Repository"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa ? "کد و مستندات مرجع؛ قابل بررسی و قابل mirror." : "Canonical code and docs, mirrored and reviewable."}
              </div>
              <div className="mt-4">
                <a
                  href={githubRepo}
                  className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border text-center block"
                  target="_blank"
                  rel="noreferrer"
                >
                  {isFa ? "مخزن گیت‌هاب" : "GitHub repo"}
                </a>
              </div>
            </div>
            <div className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-semibold">{isFa ? "مستندات" : "Docs"}</div>
              <div className="mt-2 text-sm text-muted-foreground">
                {isFa ? "نصب، نکات ایمنی، و جزئیات تأیید اصالت." : "Installation, safety guidance, and verification details."}
              </div>
              <div className="mt-4 grid gap-2">
                <Link
                  href={hrefFor("/docs/install")}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border text-center"
                >
                  {isFa ? "راهنمای نصب" : "Install guide"}
                </Link>
                <Link
                  href={hrefFor("/docs/outside/verification")}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border text-center"
                >
                  {isFa ? "راهنمای تأیید اصالت" : "Verification guide"}
                </Link>
              </div>
            </div>
          </div>

          <details className="mt-6 rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
            <summary className="cursor-pointer text-foreground font-semibold">
              {isFa ? "هدف‌های موجود در این نسخه" : "Available targets in this release"}
            </summary>
            <div className="mt-3 text-sm text-muted-foreground">
              {selectedRelease ? (
                <div className="font-mono break-all">{supportedTargets(selectedRelease).join(", ") || "n/a"}</div>
              ) : (
                <>n/a</>
              )}
            </div>
          </details>
        </div>
      </div>
    </section>
  );
}
