import path from "node:path";
import { test, expect } from "@playwright/test";
import { getLocalReleasesUncached } from "../src/lib/releases/local";

function computeDirs(cwd: string) {
  if (path.basename(cwd) === "website") {
    return { repoRoot: path.dirname(cwd), websiteDir: cwd };
  }
  return { repoRoot: cwd, websiteDir: path.join(cwd, "website") };
}

test("local releases resolve from both repo root and website directory", async () => {
  const originalCwd = process.cwd();
  const { repoRoot, websiteDir } = computeDirs(originalCwd);

  try {
    process.chdir(websiteDir);
    const fromWebsite = await getLocalReleasesUncached();
    expect(fromWebsite.length).toBeGreaterThan(0);

    process.chdir(repoRoot);
    const fromRoot = await getLocalReleasesUncached();
    expect(fromRoot.length).toBeGreaterThan(0);
    expect(fromRoot[0]?.tag).toBe(fromWebsite[0]?.tag);

    const latest = fromRoot[0]!;
    expect(latest.artifacts.some((a) => a.role === "inside" && a.target === "linux-amd64")).toBeTruthy();
    expect(latest.artifacts.some((a) => a.role === "outside" && a.target === "linux-amd64")).toBeTruthy();
  } finally {
    process.chdir(originalCwd);
  }
});

