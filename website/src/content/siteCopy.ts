import en from "./site.en.json";
import fa from "./site.fa.json";

export type Lang = "en" | "fa";

export function normalizeLang(lang?: string): Lang {
  return lang === "fa" ? "fa" : "en";
}

export const siteCopy = {
  en,
  fa,
} as const;
