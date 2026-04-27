import { test, expect, type Page, type Response } from "@playwright/test";

async function gotoWithRetry(page: Page, url: string, options?: Parameters<Page["goto"]>[1]): Promise<Response | null> {
  const maxAttempts = 2;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      return await page.goto(url, options);
    } catch (err: unknown) {
      if (attempt === maxAttempts) throw err as Error;
      await page.waitForTimeout(350 * attempt);
    }
  }
  return null;
}

test("homepage smoke: loads, nav visible, CTA visible", async ({ page }) => {
  await gotoWithRetry(page, "/", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page).toHaveURL(/\/(en|fa)\/?$/);
  await expect(page.getByTestId("site-header")).toBeVisible();
  await expect(page.getByTestId("nav-desktop")).toBeVisible();
  await expect(page.getByTestId("nav-cta-download")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
});

test("language toggle switches EN/FA deterministically on public routes", async ({ page }) => {
  await gotoWithRetry(page, "/en/download", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page).toHaveURL(/\/en\/download\/?$/);
  await expect(page.getByTestId("nav-lang-toggle")).toBeVisible();
  await expect(page.getByTestId("nav-lang-toggle")).toHaveText("FA");

  await Promise.all([
    page.waitForURL(/\/fa\/download\/?$/, { timeout: 30_000, waitUntil: "domcontentloaded" }),
    page.getByTestId("nav-lang-toggle").click(),
  ]);

  await expect(page).toHaveURL(/\/fa\/download\/?$/);
  await expect(page.locator('div[dir="rtl"]').first()).toBeVisible();
  await expect(page.getByTestId("nav-lang-toggle")).toHaveText("EN");
  await expect(page.getByTestId("nav-cta-download")).toBeVisible();
});

test("download -> verification guide route is reachable (EN/FA)", async ({ page }) => {
  const assertLang = async (lang: "en" | "fa") => {
    await gotoWithRetry(page, `/${lang}/download`, { waitUntil: "domcontentloaded", timeout: 60_000 });
    await expect(page.getByTestId("cta-verification")).toBeVisible();
    await Promise.all([
      page.waitForURL(new RegExp(`/${lang}/docs/outside/verification/?$`), { timeout: 30_000, waitUntil: "domcontentloaded" }),
      page.getByTestId("cta-verification").click(),
    ]);
    const h1 = page.getByRole("heading", { level: 1, name: /verification|تأیید/i }).first();
    await expect(h1).toBeVisible();
  };

  await assertLang("en");
  await assertLang("fa");
});

test("download section recommends Android when UA indicates Android", async ({ browser }) => {
  const context = await browser.newContext({
    userAgent:
      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
  });
  const page = await context.newPage();
  await gotoWithRetry(page, "/download", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page.getByText("Recommended download")).toBeVisible();
  await expect(page.getByText("Android").first()).toBeVisible();
  await expect(page.getByText(/android-arm64/i).first()).toBeVisible();
  await context.close();
});

test("download page shows platform options, verify/install sections, and local-fallback message", async ({ page }) => {
  await gotoWithRetry(page, "/download", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page).toHaveURL(/\/(?:(?:en|fa)\/)?download\/?$/);
  await expect(page.getByTestId("cta-verification")).toBeVisible();
  await expect(page.getByTestId("download-release-select")).toBeVisible();
  await expect(page.getByTestId("download-platform-select")).toBeVisible();
  const platformSelect = page.getByTestId("download-platform-select");
  await expect(platformSelect).toHaveAttribute("data-hydrated", "1", { timeout: 30_000 });

  const assertDownloadOk = async (platform: string, filePattern: RegExp) => {
    await platformSelect.selectOption(platform);
    const link = page.getByTestId("recommended-download");
    await expect(link).toBeVisible({ timeout: 30_000 });
    await expect(link).toHaveAttribute("href", filePattern, { timeout: 30_000 });
    const href = await link.getAttribute("href");
    expect(href).toBeTruthy();
    const absolute = new URL(href!, page.url()).toString();
    const res = await page.request.fetch(absolute, { method: "HEAD", timeout: 30_000 });
    expect(res.status()).toBeGreaterThanOrEqual(200);
    expect(res.status()).toBeLessThan(400);
  };

  await assertDownloadOk("linux-amd64", /linux-amd64/);
  await assertDownloadOk("windows-amd64", /windows-amd64\.zip$/);
  await assertDownloadOk("macos-arm64", /darwin-arm64\.tar\.gz$/);

  await platformSelect.selectOption("android");
  const androidHandoff = page.getByTestId("android-install-handoff");
  const androidFallback = page.getByText(/Android APK is not published for this release/i);
  await expect(androidHandoff.or(androidFallback)).toBeVisible();

  await platformSelect.selectOption("macos-amd64");
  await expect(page.getByText(/No matching artifact/i)).toBeVisible();

  await platformSelect.selectOption("source");
  await expect(page.getByText(/No matching artifact/i)).toBeVisible();
});

test("download hub exposes official sources and stays honest about Android install handoff (EN/FA)", async ({ page }) => {
  const assertLang = async (lang: "en" | "fa") => {
    await gotoWithRetry(page, `/${lang}/download`, { waitUntil: "domcontentloaded", timeout: 60_000 });
    await expect(page).toHaveURL(new RegExp(`/${lang}/download/?$`));

    await expect(page.getByTestId("source-github-releases")).toBeVisible();
    await expect(page.getByTestId("source-direct-apk")).toBeVisible();
    await expect(page.getByTestId("source-termux")).toBeVisible();

    const fdroidLink = page.getByTestId("source-fdroid");
    const fdroidDisabled = page.getByTestId("source-fdroid-disabled");
    await expect(fdroidLink.or(fdroidDisabled)).toBeVisible();

    await expect(page.getByText(/Trust Note|یادداشت اعتماد/)).toBeVisible();
    await expect(page.getByText(/cannot silently install|بی.?صدا/)).toBeVisible();

    if (lang === "fa") {
      await expect(page.locator('div[dir="rtl"]').first()).toBeVisible();
    }
  };

  await assertLang("en");
  await assertLang("fa");
});

test("fa download loads via localhost without connection refused", async ({ page }) => {
  await gotoWithRetry(page, "http://localhost:3001/fa/download", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page).toHaveURL(/\/fa\/download\/?$/);
  await expect(page.getByTestId("download-platform-select")).toBeVisible();
  await expect(page.locator('div[dir="rtl"]').first()).toBeVisible();
});

test("downloads API returns latest release metadata and platform map", async ({ page }) => {
  const res = await page.request.get("/api/downloads");
  expect(res.ok()).toBeTruthy();
  const json = (await res.json()) as {
    latest?: { tag?: string | null } | null;
    platforms?: Record<string, unknown> | null;
  };
  expect(json.latest?.tag).toMatch(/^v\d+\.\d+\.\d+$/);
  expect(json.platforms).toBeTruthy();
  expect(json.platforms?.["linux-amd64"]).toBeTruthy();
  expect(json.platforms?.["windows-amd64"]).toBeTruthy();
  expect(json.platforms?.["macos-arm64"]).toBeTruthy();
  expect(json.platforms?.["android"]).toBeTruthy();
  expect(json.platforms?.["ios"]).toBeTruthy();
});

test("downloads API returns an explicit issue when requesting Android outside role", async ({ page }) => {
  const res = await page.request.get("/api/downloads?platform=android&role=outside");
  expect(res.ok()).toBeTruthy();
  const json = (await res.json()) as {
    artifact?: unknown | null;
    issues?: string[] | null;
  };
  expect(json.artifact).toBeNull();
  expect(json.issues).toContain("android_outside_not_supported");
});

test("support page renders referral and donation sections", async ({ page }) => {
  await page.goto("/support");
  await expect(page.getByRole("heading", { level: 1 })).toContainText(/SunLionet/i);
  await expect(page.locator("#revolut-referral-title")).toBeVisible();
  await expect(page.getByRole("img", { name: /Revolut referral QR code/i })).toBeVisible();
  await expect(page.locator("#donation-title")).toBeVisible();
});

test("docs landing loads and at least one docs route is reachable", async ({ page }) => {
  await gotoWithRetry(page, "/docs", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page.getByRole("heading", { name: /Documentation/i })).toBeVisible();
  await expect(page.getByTestId("docs-nav")).toBeVisible();
  await expect(page.getByTestId("docs-browse-all")).toBeVisible();
  await page.getByTestId("docs-browse-all").click();
  await expect(page).toHaveURL(/\/docs\/all$/);
  await expect(page.getByRole("heading", { name: /Browse all docs/i })).toBeVisible();
});

test("installation page renders key headings", async ({ page }) => {
  await page.goto("/installation");
  await expect(page.getByRole("heading", { name: /Installation/i })).toBeVisible();
  await expect(page.getByRole("main").getByRole("link", { name: "Download" }).first()).toBeVisible();
});

test("navigation consistency across core pages", async ({ page }) => {
  await page.goto("/en");
  await page.getByTestId("nav-download").click();
  await expect(page).toHaveURL(/\/en\/download$/);
  await page.getByTestId("nav-docs").click();
  await expect(page).toHaveURL(/\/en\/docs$/);
  await page.getByTestId("nav-support").click();
  await expect(page).toHaveURL(/\/en\/support$/);
});
