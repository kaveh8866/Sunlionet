import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output:
    process.env.SUNLIONET_STATIC_EXPORT === "1" || process.env.SHADOWNET_STATIC_EXPORT === "1"
      ? "export"
      : undefined,
  trailingSlash: true,
  skipTrailingSlashRedirect: true,
  experimental: {
    externalDir: true,
  },
  images: {
    unoptimized: true,
  },
  /* config options here */
};

export default nextConfig;
