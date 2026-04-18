import type { Metadata } from "next";
import { Inter, JetBrains_Mono, Vazirmatn } from "next/font/google";
import "./globals.css";
import { ThemeInitScript } from "../components/ThemeInitScript";
import { SiteFooter } from "../components/SiteFooter";
import { SiteHeader } from "../components/SiteHeader";

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

const fontFa = Vazirmatn({
  subsets: ["arabic"],
  variable: "--font-fa",
  display: "swap",
});

export const metadata: Metadata = {
  title: "SunLionet",
  description:
    "Open-source DPI resistance with offline detection, deterministic rotation, and local-only assistance. Built for high-risk restricted networks.",
  keywords: [
    "censorship circumvention",
    "offline DPI bypass",
    "sing-box",
    "reality",
    "hysteria2",
    "tuic",
    "shadowtls",
    "open source privacy tools",
  ],
  openGraph: {
    title: "SunLionet",
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
    <html lang="en" className={`${fontSans.variable} ${fontMono.variable} ${fontFa.variable}`} suppressHydrationWarning>
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
