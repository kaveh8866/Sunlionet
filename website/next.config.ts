import type { NextConfig } from "next";
import { execSync } from "node:child_process";

function readGitText(command: string): string | null {
  try {
    const out = execSync(command, { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim();
    return out ? out : null;
  } catch {
    return null;
  }
}

const buildSha =
  process.env.NEXT_PUBLIC_GIT_SHA ??
  process.env.VERCEL_GIT_COMMIT_SHA ??
  process.env.GITHUB_SHA ??
  readGitText("git rev-parse HEAD");
const buildAuthor =
  process.env.NEXT_PUBLIC_GIT_AUTHOR ??
  process.env.VERCEL_GIT_COMMIT_AUTHOR_LOGIN ??
  process.env.GITHUB_ACTOR ??
  readGitText("git log -1 --pretty=format:%an");
const buildTimestamp =
  process.env.NEXT_PUBLIC_GIT_COMMIT_TIMESTAMP ??
  process.env.VERCEL_GIT_COMMIT_TIMESTAMP ??
  readGitText("git log -1 --pretty=format:%cI");

const env: Record<string, string> = {};
if (buildSha) env.NEXT_PUBLIC_GIT_SHA = buildSha;
if (buildAuthor) env.NEXT_PUBLIC_GIT_AUTHOR = buildAuthor;
if (buildTimestamp) env.NEXT_PUBLIC_GIT_COMMIT_TIMESTAMP = buildTimestamp;

const nextConfig: NextConfig = {
  output:
    process.env.SUNLIONET_STATIC_EXPORT === "1" ? "export" : undefined,
  trailingSlash: true,
  skipTrailingSlashRedirect: true,
  env,
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "geolocation=(), microphone=(), camera=()" },
          { key: "Content-Security-Policy", value: "frame-ancestors 'none'; base-uri 'self'; object-src 'none'" },
        ],
      },
    ];
  },
  experimental: {
    externalDir: true,
  },
  images: {
    unoptimized: true,
  },
  /* config options here */
};

export default nextConfig;
