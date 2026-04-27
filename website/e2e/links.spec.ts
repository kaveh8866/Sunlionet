import { expect, test, type APIResponse, type Page } from "@playwright/test";

function normalizeHref(href: string) {
  return href.trim();
}

function isSkippableLink(href: string) {
  const h = normalizeHref(href);
  if (!h) return true;
  if (h.startsWith("#")) return true;
  if (h.startsWith("mailto:")) return true;
  if (h.startsWith("tel:")) return true;
  if (h.startsWith("javascript:")) return true;
  return false;
}

function stripHash(url: URL) {
  const u = new URL(url.toString());
  u.hash = "";
  return u;
}

function isAssetPath(pathname: string) {
  if (pathname.startsWith("/_next/")) return true;
  if (pathname.startsWith("/downloads/")) return true;
  if (pathname.startsWith("/media/")) return true;
  if (pathname === "/icon.png") return true;
  if (pathname.endsWith(".tar.gz")) return true;
  if (pathname.endsWith(".zip")) return true;
  if (pathname.endsWith(".apk")) return true;
  if (pathname.endsWith(".sha256")) return true;
  if (pathname.endsWith(".sig")) return true;
  if (pathname.endsWith(".txt")) return true;
  if (pathname.endsWith(".png")) return true;
  if (pathname.endsWith(".jpg")) return true;
  if (pathname.endsWith(".jpeg")) return true;
  if (pathname.endsWith(".webp")) return true;
  if (pathname.endsWith(".gif")) return true;
  if (pathname.endsWith(".mp4")) return true;
  return false;
}

async function getWithRetry(page: Page, url: string): Promise<APIResponse> {
  const maxAttempts = 3;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      return await page.request.get(url, { maxRedirects: 0, timeout: 20_000 });
    } catch (err: unknown) {
      if (attempt === maxAttempts) throw err as Error;
      await page.waitForTimeout(350 * attempt);
    }
  }
  throw new Error(`request failed for ${url}`);
}

test.setTimeout(180_000);

test("all internal links resolve (crawl)", async ({ page, baseURL }) => {
  expect(baseURL).toBeTruthy();
  const origin = new URL(baseURL ?? "http://127.0.0.1:3001").origin;

  const seedPaths = ["/", "/docs", "/docs/all", "/download", "/installation", "/dashboard", "/support", "/community", "/video"];
  const queue: URL[] = seedPaths.map((p) => new URL(p, origin));
  const visited = new Set<string>();

  const maxPages = 250;

  while (queue.length && visited.size < maxPages) {
    const next = queue.shift();
    if (!next) break;

    const normalized = stripHash(next);
    const key = `${normalized.pathname}${normalized.search}`;
    if (visited.has(key)) continue;
    visited.add(key);

    if (isAssetPath(normalized.pathname)) continue;

    const resp = await getWithRetry(page, normalized.toString());
    const status = resp.status();
    expect(status, `bad status for ${normalized.toString()}`).toBeLessThan(400);

    if (status >= 300 && status < 400) {
      const location = resp.headers()["location"];
      if (location) {
        const u = new URL(location, origin);
        const candidate = stripHash(u);
        const candidateKey = `${candidate.pathname}${candidate.search}`;
        if (!visited.has(candidateKey)) queue.push(candidate);
      }
      continue;
    }

    const contentType = resp.headers()["content-type"] ?? "";
    if (!contentType.includes("text/html")) continue;

    const html = await resp.text();
    const hrefs: string[] = [];
    for (const m of html.matchAll(/href\s*=\s*(?:"([^"]*)"|'([^']*)')/g)) {
      const href = m[1] ?? m[2] ?? "";
      if (href) hrefs.push(href);
    }
    for (const rawHref of hrefs) {
      if (isSkippableLink(rawHref)) continue;

      let u: URL;
      try {
        u = new URL(rawHref, origin);
      } catch {
        throw new Error(`invalid URL on ${normalized.toString()}: ${rawHref}`);
      }

      if (u.origin !== origin) continue;

      const candidate = stripHash(u);
      const candidateKey = `${candidate.pathname}${candidate.search}`;
      if (!visited.has(candidateKey)) queue.push(candidate);
    }
  }

  expect(visited.size, "crawl found no pages").toBeGreaterThan(10);
});
