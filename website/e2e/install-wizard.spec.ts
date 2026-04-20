import { test, expect } from "@playwright/test";

const steps = ["welcome", "platform", "download", "verify", "configure", "finish"] as const;

test("outside install wizard: loads and navigates steps", async ({ page }) => {
  await page.goto("/installation/wizard");
  await expect(page).toHaveURL(/\/installation\/wizard\/welcome$/);
  await expect(page.locator("h1[tabindex='-1']")).toBeVisible();

  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page).toHaveURL(/\/installation\/wizard\/platform$/);

  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/installation\/wizard\/welcome$/);

  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page).toHaveURL(/\/installation\/wizard\/platform$/);
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page).toHaveURL(/\/installation\/wizard\/download$/, { timeout: 15_000 });
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
  await page.goto("/installation/wizard/welcome");
  await expect(page.getByRole("button", { name: "Next", exact: true })).toBeVisible();

  for (const s of steps) {
    await page.goto(`/installation/wizard/${s}`);
    await expect(page.locator("h1[tabindex='-1']")).toBeVisible();
  }

  await context.close();
});
