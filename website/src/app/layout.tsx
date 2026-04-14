import type { Metadata } from "next";
import Link from "next/link";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { ThemeInitScript } from "../components/ThemeInitScript";
import { ThemeToggle } from "../components/ThemeToggle";

const fontSans = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

const fontMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "ShadowNet Agent",
  description:
    "Open-source censorship circumvention with offline DPI detection, deterministic rotation, and local AI assistance. Built for Iran internet freedom.",
  keywords: [
    "censorship circumvention",
    "offline DPI bypass",
    "Iran internet freedom",
    "sing-box",
    "reality",
    "hysteria2",
    "tuic",
    "shadowtls",
    "open source privacy tools",
  ],
  openGraph: {
    title: "ShadowNet Agent",
    description:
      "Local, intelligent, offline DPI resistance. Dual Inside/Outside architecture for resilient censorship circumvention.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${fontSans.variable} ${fontMono.variable}`} suppressHydrationWarning>
      <head>
        <ThemeInitScript />
      </head>
      <body className="min-h-screen flex flex-col antialiased bg-background text-foreground">
        <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-50">
          <div className="container mx-auto px-4 h-16 flex items-center justify-between">
            <Link href="/" prefetch={false} className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
              <span className="w-8 h-8 rounded bg-primary text-primary-foreground flex items-center justify-center shadow-[0_0_18px_var(--ring)]">S</span>
              ShadowNet
            </Link>
            <nav className="hidden md:flex gap-6 text-sm font-medium text-muted">
              <Link href="/architecture" prefetch={false} className="hover:text-foreground transition-colors">Architecture</Link>
              <Link href="/docs" prefetch={false} className="hover:text-foreground transition-colors">Docs</Link>
              <Link href="/installation" prefetch={false} className="hover:text-foreground transition-colors">Installation</Link>
              <Link href="/download" prefetch={false} className="hover:text-foreground transition-colors">Download</Link>
              <Link href="/support" prefetch={false} className="hover:text-foreground transition-colors">Support</Link>
              <Link href="/roadmap" prefetch={false} className="hover:text-foreground transition-colors">Roadmap</Link>
            </nav>
            <div className="flex items-center gap-3">
              <ThemeToggle />
              <Link
                href="/download"
                prefetch={false}
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                Get Agent
              </Link>
            </div>
          </div>
        </header>

        <main className="flex-1">
          {children}
        </main>

        <footer className="border-t border-border bg-card/30 py-12 mt-20">
          <div className="container mx-auto px-4 text-center text-muted text-sm">
            <p className="mb-4 font-semibold text-foreground">ShadowNet Agent</p>
            <p className="mb-4 max-w-xl mx-auto">
              Privacy-first, open-source DPI resistance. Designed for resilience in restricted environments.
              No telemetry, no analytics, no logs.
            </p>
            <div className="flex justify-center gap-6 mb-8">
              <a href="https://github.com/kaveh8866/shadownet-agent" className="hover:text-foreground transition-colors">GitHub (AGPL-3.0)</a>
              <Link href="/docs/security" prefetch={false} className="hover:text-foreground transition-colors">Security Model</Link>
              <Link href="/installation" prefetch={false} className="hover:text-foreground transition-colors">Installation</Link>
              <Link href="/support" prefetch={false} className="hover:text-foreground transition-colors">Support</Link>
              <Link href="/architecture" prefetch={false} className="hover:text-foreground transition-colors">Architecture</Link>
              <Link href="/video" prefetch={false} className="hover:text-foreground transition-colors">Video</Link>
            </div>
            <div className="flex items-center justify-center gap-3">
              <ThemeToggle />
              <p className="text-muted-foreground">Built by people who believe freedom of information is a human right.</p>
            </div>
          </div>
        </footer>
      </body>
    </html>
  );
}
