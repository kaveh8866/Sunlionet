import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output:
    process.env.SUNLIONET_STATIC_EXPORT === "1" ? "export" : undefined,
  trailingSlash: true,
  skipTrailingSlashRedirect: true,
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
