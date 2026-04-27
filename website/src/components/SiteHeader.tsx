"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useMemo, useState } from "react";
import { cx } from "../lib/cx";
import { getUILangFromPathname, uiCopy } from "../lib/uiCopy";
import { ThemeToggle } from "./ThemeToggle";

type NavItem = {
  href: string;
  key: keyof (typeof uiCopy)["en"]["nav"];
};

const navItems: NavItem[] = [
  { href: "/", key: "home" },
  { href: "/download", key: "download" },
  { href: "/docs", key: "docs" },
  { href: "/support", key: "support" },
  { href: "/manifest", key: "manifest" },
  { href: "/dashboard", key: "dashboard" },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  const basePrefix = useMemo(() => {
    if (!pathname) return "";
    const m = /^\/(fa|en)(\/|$)/.exec(pathname);
    return m ? `/${m[1]}` : "";
  }, [pathname]);

  const normalizedPath = useMemo(() => {
    if (!pathname) return "/";
    if (!basePrefix) return pathname;
    const rest = pathname.slice(basePrefix.length);
    return rest.length ? rest : "/";
  }, [pathname, basePrefix]);

  const activeHref = useMemo(() => {
    if (!normalizedPath) return "/";
    if (normalizedPath === "/") return "/";
    const match = navItems
      .filter((i) => i.href !== "/")
      .find((i) => normalizedPath === i.href || normalizedPath.startsWith(`${i.href}/`));
    return match?.href ?? "/";
  }, [normalizedPath]);

  const homeHref = basePrefix || "/";
  const hrefFor = (href: string) => {
    if (href.startsWith("/dashboard")) return href;
    if (basePrefix) return `${basePrefix}${href === "/" ? "" : href}`;
    return href;
  };
  const currentLang = getUILangFromPathname(pathname);
  const otherLang = currentLang === "fa" ? "en" : "fa";
  const otherLangHref = normalizedPath.startsWith("/dashboard")
    ? `/${otherLang}`
    : `/${otherLang}${normalizedPath === "/" ? "" : normalizedPath}`;
  const labels = uiCopy[currentLang].nav;

  return (
    <header data-testid="site-header" className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-50">
      <div className="mx-auto w-full max-w-6xl px-4 h-16 flex items-center justify-between gap-4">
        <Link
          href={homeHref}
          prefetch={false}
          className="text-sm sm:text-base font-semibold tracking-tight text-foreground flex items-center gap-2"
          onClick={() => setOpen(false)}
          data-testid="nav-home"
        >
          <span className="w-8 h-8 rounded bg-card/60 border border-border shadow-[0_0_18px_var(--ring)] overflow-hidden">
            <Image
              src="/brand/sunlionet-color-64.png"
              alt="SunLionet"
              width={32}
              height={32}
              className="object-contain dark:hidden"
              priority={false}
            />
            <Image
              src="/brand/sunlionet-invert-64.png"
              alt="SunLionet"
              width={32}
              height={32}
              className="object-contain hidden dark:block"
              priority={false}
            />
          </span>
          SunLionet
        </Link>

        <nav data-testid="nav-desktop" className="hidden lg:flex items-center gap-6 text-sm font-semibold text-muted-foreground">
          {navItems.map((i) => (
            <Link
              key={i.href}
              href={hrefFor(i.href)}
              prefetch={false}
              data-testid={`nav-${i.key}`}
              className={cx(
                "transition-colors hover:text-foreground",
                activeHref === i.href ? "text-foreground" : null,
              )}
            >
              {labels[i.key]}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-2">
          <Link
            href={otherLangHref}
            prefetch={false}
            data-testid="nav-lang-toggle"
            className="hidden sm:inline-flex rounded-md border border-border bg-card/60 px-3 py-2 text-xs font-semibold text-muted-foreground hover:text-foreground hover:bg-card transition-colors"
          >
            {otherLang.toUpperCase()}
          </Link>
          <ThemeToggle />
          <Link
            href={hrefFor("/download")}
            prefetch={false}
            data-testid="nav-cta-download"
            className="hidden sm:inline-flex bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
          >
            {labels.download}
          </Link>
          <button
            type="button"
            className="inline-flex lg:hidden items-center justify-center rounded-md border border-border bg-card/60 p-2 text-foreground hover:bg-card transition-colors"
            aria-label="Open navigation menu"
            onClick={() => setOpen((v) => !v)}
            data-testid="nav-menu-button"
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
          <div data-testid="nav-mobile" className="mx-auto w-full max-w-6xl px-4 py-4 grid gap-2">
            {navItems.map((i) => (
              <Link
                key={i.href}
                href={hrefFor(i.href)}
                prefetch={false}
                onClick={() => setOpen(false)}
                data-testid={`nav-mobile-${i.key}`}
                className={cx(
                  "rounded-lg border border-border bg-card/60 px-4 py-3 text-sm font-semibold text-muted-foreground hover:text-foreground hover:bg-card transition-colors",
                  activeHref === i.href ? "text-foreground" : null,
                )}
              >
                {labels[i.key]}
              </Link>
            ))}
            <Link
              href={otherLangHref}
              prefetch={false}
              onClick={() => setOpen(false)}
              data-testid="nav-mobile-lang"
              className="rounded-lg border border-border bg-card/60 px-4 py-3 text-sm font-semibold text-muted-foreground hover:text-foreground hover:bg-card transition-colors"
            >
              {otherLang.toUpperCase()}
            </Link>
            <Link
              href={hrefFor("/download")}
              prefetch={false}
              onClick={() => setOpen(false)}
              data-testid="nav-mobile-download"
              className="mt-1 rounded-lg bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground hover:opacity-90 transition-opacity"
            >
              {labels.download}
            </Link>
          </div>
        </div>
      ) : null}
    </header>
  );
}
