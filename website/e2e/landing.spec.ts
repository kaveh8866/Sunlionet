import { test, expect } from "@playwright/test";

test("home page renders", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: /SunLionet/i })).toBeVisible();
  await expect(page.locator('a[href="#downloads"]').first()).toBeVisible();
});

test("download section recommends Android when UA indicates Android", async ({ browser }) => {
  const context = await browser.newContext({
    userAgent:
      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
  });
  const page = await context.newPage();
  await page.goto("/download");
  await expect(page.getByText("Recommended download")).toBeVisible();
  await expect(page.getByText("Android (Termux)").first()).toBeVisible();
  await expect(page.getByText(/android-arm64/i).first()).toBeVisible();
  await context.close();
});

test("support page renders referral and donation sections", async ({ page }) => {
  await page.goto("/support");
  await expect(page.getByRole("heading", { name: /Support SunLionet/i })).toBeVisible();
  await expect(page.getByRole("link", { name: /Create Revolut Account/i })).toBeVisible();
  await expect(page.getByRole("img", { name: /Revolut referral QR code/i })).toBeVisible();
  await expect(page.getByRole("heading", { name: /Direct Anonymous Donation/i })).toBeVisible();
});
