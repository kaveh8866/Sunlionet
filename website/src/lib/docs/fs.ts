import path from "node:path";
import { readdir, readFile } from "node:fs/promises";
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
  const raw = await readFile(filePath, "utf8");
  const lines = raw.split(/\r?\n/);
  for (const line of lines) {
    const m = /^#\s+(.+)\s*$/.exec(line);
    if (m) return m[1].trim();
  }
  return titleFromFilename(path.basename(filePath));
}

async function walk(dir: string, root: string): Promise<DocEntry[]> {
  const entries = await readdir(dir, { withFileTypes: true });
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

export const getDocsIndex = cache(async () => {
  const docsRoot = path.join(process.cwd(), "..", "docs");
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
  const raw = await readFile(doc.filePath, "utf8");
  return { doc, raw };
}
