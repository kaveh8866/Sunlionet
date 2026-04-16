"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useMemo, useState } from "react";
import { cx } from "../lib/cx";
import { ThemeToggle } from "./ThemeToggle";

type NavItem = {
  href: string;
  label: string;
};

const navItems: NavItem[] = [
  { href: "/", label: "Home" },
  { href: "/docs", label: "Docs" },
  { href: "/download", label: "Download" },
  { href: "/architecture", label: "Architecture" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/roadmap", label: "Roadmap" },
  { href: "/support", label: "Support" },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  const activeHref = useMemo(() => {
    if (!pathname) return "/";
    if (pathname === "/") return "/";
    const match = navItems
      .filter((i) => i.href !== "/")
      .find((i) => pathname === i.href || pathname.startsWith(`${i.href}/`));
    return match?.href ?? "/";
  }, [pathname]);

  return (
    <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-50">
      <div className="mx-auto w-full max-w-6xl px-4 h-16 flex items-center justify-between gap-4">
        <Link
          href="/"
          prefetch={false}
          className="text-sm sm:text-base font-semibold tracking-tight text-foreground flex items-center gap-2"
          onClick={() => setOpen(false)}
        >
          <span className="w-8 h-8 rounded bg-primary text-primary-foreground flex items-center justify-center shadow-[0_0_18px_var(--ring)] font-bold">
            S
          </span>
          ShadowNet
        </Link>

        <nav className="hidden lg:flex items-center gap-6 text-sm font-semibold text-muted-foreground">
          {navItems.map((i) => (
            <Link
              key={i.href}
              href={i.href}
              prefetch={false}
              className={cx(
                "transition-colors hover:text-foreground",
                activeHref === i.href ? "text-foreground" : null,
              )}
            >
              {i.label}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-2">
          <ThemeToggle />
          <Link
            href="/download"
            prefetch={false}
            className="hidden sm:inline-flex bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
          >
            Download
          </Link>
          <button
            type="button"
            className="inline-flex lg:hidden items-center justify-center rounded-md border border-border bg-card/60 p-2 text-foreground hover:bg-card transition-colors"
            aria-label="Open navigation menu"
            onClick={() => setOpen((v) => !v)}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M4 6h16M4 12h16M4 18h16"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              />
            </svg>
          </button>
        </div>
      </div>

      {open ? (
        <div className="lg:hidden border-t border-border bg-card/60">
          <div className="mx-auto w-full max-w-6xl px-4 py-4 grid gap-2">
            {navItems.map((i) => (
              <Link
                key={i.href}
                href={i.href}
                prefetch={false}
                onClick={() => setOpen(false)}
                className={cx(
                  "rounded-lg border border-border bg-card/60 px-4 py-3 text-sm font-semibold text-muted-foreground hover:text-foreground hover:bg-card transition-colors",
                  activeHref === i.href ? "text-foreground" : null,
                )}
              >
                {i.label}
              </Link>
            ))}
            <Link
              href="/download"
              prefetch={false}
              onClick={() => setOpen(false)}
              className="mt-1 rounded-lg bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground hover:opacity-90 transition-opacity"
            >
              Download
            </Link>
          </div>
        </div>
      ) : null}
    </header>
  );
}
