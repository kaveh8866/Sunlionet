import path from "node:path";
import type { Dirent } from "node:fs";
import { existsSync, statSync } from "node:fs";
import { readdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { cache } from "react";

export type DocEntry = {
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

async function walk(dir: string, root: string): Promise<DocEntry[]> {
  let entries: Dirent[];
  try {
    entries = (await readdir(dir, { withFileTypes: true })) as Dirent[];
  } catch {
    return [];
  }
  const out: DocEntry[] = [];

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

function isDocsRoot(candidate: string) {
  try {
    return (
      statSync(candidate).isDirectory() &&
      existsSync(path.join(candidate, "index.md"))
    );
  } catch {
    return false;
  }
}

function findDocsRoot() {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const candidates = [
    path.join(here, "../../../docs"),
    path.join(here, "../../../../docs"),
  ];
  for (const c of candidates) {
    if (isDocsRoot(c)) return c;
  }
  return candidates[0];
}

export const getDocsIndex = cache(async () => {
  const docsRoot = findDocsRoot();
  const entries = await walk(docsRoot, docsRoot);
  entries.sort((a, b) => a.slug.join("/").localeCompare(b.slug.join("/")));
  return entries;
});

export const getDocBySlug = cache(async (slug: string[]) => {
  const key = slug.join("/");
  const entries = await getDocsIndex();
  return entries.find((e) => e.slug.join("/") === key) ?? null;
});

export async function readDocMarkdownBySlug(slug: string[]) {
  const doc = await getDocBySlug(slug);
  if (!doc) return null;
  let raw: string;
  try {
    raw = await readFile(doc.filePath, "utf8");
  } catch {
    return null;
  }
  return { doc, raw };
}
