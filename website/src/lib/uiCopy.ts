import en from "../locales/en/ui.json";
import fa from "../locales/fa/ui.json";

export type UILang = "en" | "fa";

export function resolveUILang(lang?: string): UILang {
  return lang === "fa" ? "fa" : "en";
}

export function getUILangFromPathname(pathname: string | null): UILang {
  if (!pathname) return "en";
  return pathname === "/fa" || pathname.startsWith("/fa/") ? "fa" : "en";
}

export const uiCopy = {
  en,
  fa,
} as const;

