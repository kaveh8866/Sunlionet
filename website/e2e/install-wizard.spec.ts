import { test, expect, type Page } from "@playwright/test";

const steps = ["welcome", "platform", "download", "verify", "configure", "finish"] as const;

async function gotoWizardStep(page: Page, path: string) {
  const maxAttempts = 2;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    await page.goto(path);
    const errorCount = await page.getByRole("heading", { name: /Application error: a client-side exception has occurred/i }).count();
    if (errorCount === 0) return;
    if (attempt === maxAttempts) {
      throw new Error(`wizard route failed to load without client-side error: ${path}`);
    }
  }
}

test("outside install wizard: loads and navigates steps", async ({ page }) => {
  await page.goto("/installation/wizard");
  await expect(page).toHaveURL(/\/installation\/wizard\/welcome$/);
  await expect(page.locator("h1[tabindex='-1']")).toBeVisible();

  const next = page.getByRole("button", { name: "Next", exact: true });
  const back = page.getByRole("button", { name: "Back", exact: true });
  await expect(next).toBeEnabled();

  await next.click();
  await page.waitForURL(/\/installation\/wizard\/platform$/, { timeout: 15_000 });

  await back.click();
  await page.waitForURL(/\/installation\/wizard\/welcome$/, { timeout: 15_000 });

  await next.click();
  await page.waitForURL(/\/installation\/wizard\/platform$/, { timeout: 15_000 });
  await next.click();
  await page.waitForURL(/\/installation\/wizard\/download$/, { timeout: 15_000 });
});

test("inside install wizard: loads and deep-links", async ({ page }) => {
  await page.goto("/dashboard/installation/wizard/verify");
  await expect(page).toHaveURL(/\/dashboard\/installation\/wizard\/verify$/);
  await expect(page.locator("h1[tabindex='-1']")).toBeVisible();

  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page).toHaveURL(/\/dashboard\/installation\/wizard\/configure$/);
});

test("wizard is responsive on Android-sized viewport", async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    userAgent:
      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
  });
  const page = await context.newPage();
  await gotoWizardStep(page, "/installation/wizard/welcome");
  await expect(page.getByRole("button", { name: "Next", exact: true })).toBeVisible();

  for (const s of steps) {
    await gotoWizardStep(page, `/installation/wizard/${s}`);
    await expect(page.locator("h1[tabindex='-1']")).toBeVisible();
  }

  await context.close();
});
