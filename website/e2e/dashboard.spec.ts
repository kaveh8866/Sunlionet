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

async function waitForRuntimeSnapshot(page: Page) {
  await page.waitForResponse((r) => r.url().includes("/api/proxy/state") && r.status() === 200, { timeout: 30_000 });
}

test("/dashboard/runtime renders (offline)", async ({ page }) => {
  await gotoWithRetry(page, "/dashboard/runtime", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
  await expect(page.getByText("No active SunLionet runtime detected")).toBeVisible();
});

test("/dashboard/runtime renders (connected)", async ({ page }) => {
  await page.route("**/api/proxy/state**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        status: "connected",
        activeProfile: "reality-1",
        latencyMs: 120,
        lastUpdated: 1710000000,
        failures: [],
        mode: "real",
      }),
    });
  });
  await page.route("**/api/proxy/events/list**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        { timestamp: 1710000000, type: "PROFILE_SWITCH", message: "Selected profile reality-1", metadata: { selected: "reality-1" } },
      ]),
    });
  });
  await page.route("**/api/proxy/events**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify({
        timestamp: 1710000001,
        type: "CONNECTION_SUCCESS",
        message: "Connection validated by HTTP probe",
        metadata: { profile: "reality-1", latency_ms: 120, http_status: 204 },
      })}\n\n`,
    });
  });

  await gotoWithRetry(page, "/dashboard/runtime", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await waitForRuntimeSnapshot(page);
  await expect(page.getByTestId("runtime-status-badge")).toHaveText(/^Connected$/);
  await expect(page.getByTestId("runtime-active-profile")).toHaveText("reality-1");
  await page.locator("summary").filter({ hasText: /Show activity/i }).click();
  await expect(page.getByText("Event Timeline")).toBeVisible();
  await expect(page.getByText("Connection validated by HTTP probe", { exact: true })).toBeVisible();
});

test("/dashboard/runtime renders (error status)", async ({ page }) => {
  await page.route("**/api/proxy/state**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        status: "failed",
        activeProfile: "reality-1",
        latencyMs: 0,
        lastUpdated: 1710000000,
        failures: [{ timestamp: 1710000000, reason: "probe timeout" }],
        mode: "real",
      }),
    });
  });
  await page.route("**/api/proxy/events/list**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([{ timestamp: 1710000000, type: "CONNECTION_FAIL", message: "Probe timeout" }]),
    });
  });
  await page.route("**/api/proxy/events**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify({
        timestamp: 1710000001,
        type: "CONNECTION_FAIL",
        message: "Probe timeout",
      })}\n\n`,
    });
  });

  await gotoWithRetry(page, "/dashboard/runtime", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await waitForRuntimeSnapshot(page);
  await expect(page.getByTestId("runtime-status-badge")).toHaveText(/^Error$/i);
  await expect(page.getByText("Not secure")).toBeVisible();
});

test("/dashboard/global renders (no feed)", async ({ page }) => {
  await gotoWithRetry(page, "/dashboard/global", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
  await expect(page.getByText("No live dashboard feed detected")).toBeVisible();
  await expect(page.getByText("Privacy mode")).toBeVisible();
});

test("/dashboard/protocols renders", async ({ page }) => {
  await gotoWithRetry(page, "/dashboard/protocols", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
  await expect(page.getByText("No live dashboard feed detected")).toBeVisible();
});
