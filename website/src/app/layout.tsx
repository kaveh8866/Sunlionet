import type { Metadata } from "next";
import "./globals.css";
import { ThemeInitScript } from "../components/ThemeInitScript";
import { SiteFooter } from "../components/SiteFooter";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "SunLionet",
  description:
    "Offline-first, open-source privacy and resilient communication system with local detection, deterministic rotation, and local-only assistance. Built for high-risk restricted networks.",
  keywords: [
    "privacy",
    "resilient networking",
    "offline-first",
    "sing-box",
    "reality",
    "hysteria2",
    "tuic",
    "shadowtls",
    "open source",
  ],
  openGraph: {
    title: "SunLionet",
    description:
      "Offline-first privacy and resilient communication. Dual Inside/Outside architecture with local-only runtime visibility.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <ThemeInitScript />
      </head>
      <body className="min-h-screen flex flex-col antialiased bg-background text-foreground">
        <SiteHeader />

        <main className="flex-1">
          {children}
        </main>

        <SiteFooter />
      </body>
    </html>
  );
}
