import path from "node:path";
import { readdir, readFile, stat } from "node:fs/promises";
import { createReadStream } from "node:fs";
import { createHash } from "node:crypto";

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

async function existsFile(p) {
  try {
    const s = await stat(p);
    return s.isFile();
  } catch {
    return false;
  }
}

async function sha256File(filePath) {
  const hash = createHash("sha256");
  await new Promise((resolve, reject) => {
    const stream = createReadStream(filePath);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("error", reject);
    stream.on("end", resolve);
  });
  return hash.digest("hex").toLowerCase();
}

function parseSha256Line(line) {
  const m = /^([a-fA-F0-9]{64})\s+(\S+)$/.exec(line.trim());
  if (!m) return null;
  return { hash: m[1].toLowerCase(), fileName: m[2] };
}

async function checkDownloads() {
  const downloadsDir = path.join(process.cwd(), "public", "downloads");
  const hasDownloads = await existsDir(downloadsDir);
  if (!hasDownloads) return true;

  let downloadsOk = true;
  const entries = await readdir(downloadsDir, { withFileTypes: true });
  const versionDirs = entries.filter((e) => e.isDirectory() && e.name.startsWith("v")).map((e) => e.name);
  versionDirs.sort((a, b) => a.localeCompare(b));

  for (const tag of versionDirs) {
    const dir = path.join(downloadsDir, tag);
    const files = await readdir(dir, { withFileTypes: true });
    const fileSet = new Set(files.filter((f) => f.isFile()).map((f) => f.name));

    for (const name of fileSet) {
      if (name === "VERSION.txt") continue;
      if (name.endsWith(".sha256")) continue;
      if (name === "checksums.txt" || name === "checksums.sig" || name === "checksums.pub" || name === "checksums.pub.sha256") continue;

      const shaFile = `${name}.sha256`;
      if (!fileSet.has(shaFile)) {
        downloadsOk = false;
        console.error(`[downloads] ${tag}: missing ${shaFile}`);
        continue;
      }

      const shaPath = path.join(dir, shaFile);
      const raw = (await readFile(shaPath, "utf8")).split(/\r?\n/).find((l) => l.trim().length) ?? "";
      const parsed = parseSha256Line(raw);
      if (!parsed) {
        downloadsOk = false;
        console.error(`[downloads] ${tag}: invalid sha256 file ${shaFile}`);
        continue;
      }
      if (parsed.fileName !== name) {
        downloadsOk = false;
        console.error(`[downloads] ${tag}: sha256 file ${shaFile} references ${parsed.fileName} (expected ${name})`);
        continue;
      }

      const filePath = path.join(dir, name);
      if (!(await existsFile(filePath))) {
        downloadsOk = false;
        console.error(`[downloads] ${tag}: referenced file missing ${name}`);
        continue;
      }

      const actual = await sha256File(filePath);
      if (actual !== parsed.hash) {
        downloadsOk = false;
        console.error(`[downloads] ${tag}: checksum mismatch for ${name}: expected ${parsed.hash}, got ${actual}`);
      }
    }
  }

  return downloadsOk;
}

const downloadsOk = await checkDownloads();
if (!downloadsOk) process.exit(1);
console.log("content + downloads OK");
