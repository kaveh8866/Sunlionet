import { test, expect } from "@playwright/test";

test("homepage smoke: loads, nav visible, CTA visible", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/(en|fa)$/);
  await expect(page.getByTestId("site-header")).toBeVisible();
  await expect(page.getByTestId("nav-desktop")).toBeVisible();
  await expect(page.getByTestId("nav-cta-download")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.locator("#downloads")).toHaveCount(1);
});

test("download section recommends Android when UA indicates Android", async ({ browser }) => {
  const context = await browser.newContext({
    userAgent:
      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36",
  });
  const page = await context.newPage();
  await page.goto("/download");
  await expect(page.getByText("Recommended download")).toBeVisible();
  await expect(page.getByText("Android (APK)").first()).toBeVisible();
  await expect(page.getByText(/android-arm64/i).first()).toBeVisible();
  await context.close();
});

test("download page shows platform options, verify/install sections, and local-fallback message", async ({ page }) => {
  await page.goto("/download");
  await expect(page.getByText("Recommended artifact", { exact: true })).toBeVisible();
  await expect(page.getByText("Quick install (stepwise)")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Verification" })).toBeVisible();
  await expect(page.getByTestId("download-platform-select")).toBeVisible();
  await page.getByTestId("download-platform-select").selectOption("linux-amd64");
  await expect(page.getByTestId("recommended-download")).toBeVisible();

  await page.getByTestId("download-platform-select").selectOption("source");
  await expect(page.getByText("No matching artifact for this platform/role.")).toBeVisible();
});

test("support page renders referral and donation sections", async ({ page }) => {
  await page.goto("/support");
  await expect(page.getByRole("heading", { name: /Support SunLionet/i })).toBeVisible();
  await expect(page.getByRole("link", { name: /Create Revolut Account/i })).toBeVisible();
  await expect(page.getByRole("img", { name: /Revolut referral QR code/i })).toBeVisible();
  await expect(page.getByRole("heading", { name: /Direct Anonymous Donation/i })).toBeVisible();
});

test("docs landing loads and at least one docs route is reachable", async ({ page }) => {
  await page.goto("/docs");
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
