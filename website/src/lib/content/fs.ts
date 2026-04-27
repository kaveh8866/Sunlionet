import path from "node:path";
import type { Dirent } from "node:fs";
import { existsSync, statSync } from "node:fs";
import { readdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { cache } from "react";

export type ContentType = "blog" | "docs" | "pages" | "updates";

export type ContentEntry = {
  type: ContentType;
  lang: "en" | "fa";
  slug: string[];
  filePath: string;
  title: string;
};

function titleFromFilename(name: string) {
  return name
    .replace(/\.md$/i, "")
    .replace(/[-_]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

async function readTitle(filePath: string) {
  let raw: string;
  try {
    raw = await readFile(filePath, "utf8");
  } catch {
    return titleFromFilename(path.basename(filePath));
  }
  const lines = raw.split(/\r?\n/);
  for (const line of lines) {
    const m = /^#\s+(.+)\s*$/.exec(line);
    if (m) return m[1].trim();
  }
  return titleFromFilename(path.basename(filePath));
}

function isContentRoot(candidate: string) {
  try {
    return statSync(candidate).isDirectory() && existsSync(path.join(candidate, "en")) && existsSync(path.join(candidate, "fa"));
  } catch {
    return false;
  }
}

function findContentRoot() {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const candidates = [
    path.join(here, "../../../content"),
    path.join(here, "../../../../content"),
  ];
  for (const c of candidates) {
    if (isContentRoot(c)) return c;
  }
  return candidates[0];
}

async function walk(dir: string, root: string): Promise<Array<{ slug: string[]; filePath: string; title: string }>> {
  let entries: Dirent[];
  try {
    entries = (await readdir(dir, { withFileTypes: true })) as Dirent[];
  } catch {
    return [];
  }
  const out: Array<{ slug: string[]; filePath: string; title: string }> = [];

  for (const e of entries) {
    if (e.name.startsWith(".")) continue;
    if (e.isDirectory()) {
      out.push(...(await walk(path.join(dir, e.name), root)));
      continue;
    }
    if (!e.isFile()) continue;
    if (!e.name.toLowerCase().endsWith(".md")) continue;

    const filePath = path.join(dir, e.name);
    const rel = path.relative(root, filePath);
    const slug = rel
      .replace(/\\/g, "/")
      .replace(/\.md$/i, "")
      .split("/")
      .filter(Boolean);

    const title = await readTitle(filePath);
    out.push({ slug, filePath, title });
  }

  return out;
}

export const getContentIndex = cache(async (type: ContentType, lang: "en" | "fa") => {
  const root = findContentRoot();
  const base = path.join(root, lang, type);
  const entries = await walk(base, base);
  entries.sort((a, b) => a.slug.join("/").localeCompare(b.slug.join("/")));
  const out: ContentEntry[] = entries.map((e) => ({ ...e, type, lang }));
  return out;
});

export const getContentBySlug = cache(async (type: ContentType, lang: "en" | "fa", slug: string[]) => {
  const key = slug.join("/");
  const entries = await getContentIndex(type, lang);
  return entries.find((e) => e.slug.join("/") === key) ?? null;
});

export async function readContentMarkdownBySlug(type: ContentType, lang: "en" | "fa", slug: string[]) {
  const entry = await getContentBySlug(type, lang, slug);
  if (!entry) return null;
  let raw: string;
  try {
    raw = await readFile(entry.filePath, "utf8");
  } catch {
    return null;
  }
  return { entry, raw };
}
