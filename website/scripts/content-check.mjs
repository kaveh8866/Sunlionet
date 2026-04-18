import path from "node:path";
import { readdir, readFile, stat } from "node:fs/promises";

const CONTENT_TYPES = ["blog", "docs", "pages", "updates"];

async function existsDir(p) {
  try {
    const s = await stat(p);
    return s.isDirectory();
  } catch {
    return false;
  }
}

async function findContentRoot() {
  let dir = process.cwd();
  for (let i = 0; i < 10; i++) {
    const candidate = path.join(dir, "content");
    if (await existsDir(path.join(candidate, "en")) && await existsDir(path.join(candidate, "fa"))) return candidate;
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return path.join(process.cwd(), "content");
}

async function listMarkdownFiles(dir, rootDir = dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const out = [];

  for (const e of entries) {
    if (e.name.startsWith(".")) continue;
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      out.push(...(await listMarkdownFiles(full, rootDir)));
      continue;
    }
    if (!e.isFile()) continue;
    if (!e.name.toLowerCase().endsWith(".md")) continue;
    out.push(path.relative(rootDir, full).replace(/\\/g, "/"));
  }

  out.sort((a, b) => a.localeCompare(b));
  return out;
}

async function readJson(filePath) {
  const raw = await readFile(filePath, "utf8");
  return JSON.parse(raw);
}

function assertTerminology(obj) {
  const bad = [];
  for (const [k, v] of Object.entries(obj)) {
    const en = v && typeof v === "object" ? v.en : undefined;
    const fa = v && typeof v === "object" ? v.fa : undefined;
    if (typeof en !== "string" || !en.trim() || typeof fa !== "string" || !fa.trim()) bad.push(k);
  }
  if (bad.length) {
    throw new Error(`terminology.json has invalid entries: ${bad.join(", ")}`);
  }
}

const root = await findContentRoot();
const terminologyPath = path.join(root, "terminology.json");
const terminology = await readJson(terminologyPath);
assertTerminology(terminology);

let ok = true;

for (const t of CONTENT_TYPES) {
  const enDir = path.join(root, "en", t);
  const faDir = path.join(root, "fa", t);
  const hasEn = await existsDir(enDir);
  const hasFa = await existsDir(faDir);

  if (!hasEn && !hasFa) continue;
  if (hasEn !== hasFa) {
    ok = false;
    console.error(`[content] ${t}: missing directory for ${hasEn ? "fa" : "en"}`);
    continue;
  }

  const enFiles = await listMarkdownFiles(enDir);
  const faFiles = await listMarkdownFiles(faDir);
  const enSet = new Set(enFiles);
  const faSet = new Set(faFiles);

  const missingFa = enFiles.filter((f) => !faSet.has(f));
  const missingEn = faFiles.filter((f) => !enSet.has(f));

  if (missingFa.length || missingEn.length) {
    ok = false;
    for (const f of missingFa) console.error(`[content] ${t}: missing fa/${t}/${f}`);
    for (const f of missingEn) console.error(`[content] ${t}: missing en/${t}/${f}`);
  }
}

if (!ok) process.exit(1);
console.log("content parity OK");

