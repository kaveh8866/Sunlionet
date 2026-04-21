"use client";

import Link, { type LinkProps } from "next/link";
import { usePathname } from "next/navigation";
import { useMemo } from "react";
import type { AnchorHTMLAttributes } from "react";

type Props = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
  LinkProps & {
    href: string;
  };

function getLangPrefix(pathname: string | null) {
  if (!pathname) return "";
  if (pathname === "/fa" || pathname.startsWith("/fa/")) return "/fa";
  if (pathname === "/en" || pathname.startsWith("/en/")) return "/en";
  return "";
}

export function LocalizedLink({ href, ...props }: Props) {
  const pathname = usePathname();
  const prefix = useMemo(() => getLangPrefix(pathname), [pathname]);

  const resolvedHref = useMemo(() => {
    if (!href.startsWith("/")) return href;
    if (
      href === "/icon.png" ||
      href.startsWith("/api/") ||
      href.startsWith("/downloads/") ||
      href.startsWith("/media/") ||
      href.startsWith("/dashboard")
    ) {
      return href;
    }
    if (!prefix) return href;
    if (href === "/") return prefix;
    if (href === prefix || href.startsWith(`${prefix}/`)) return href;
    return `${prefix}${href}`;
  }, [href, prefix]);

  return <Link href={resolvedHref} {...props} />;
}
