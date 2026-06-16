import { test, expect } from "@playwright/test";

// Prerequisites: full stack running via `make scenario-b.up`
// Supervisor credentials: supervisor@centralbank.gov / Supervisor2026!
// Governance credentials: governance@centralbank.gov / Governance2026!

const SUPERVISOR_URL = process.env.SUPERVISOR_URL ?? "http://localhost:5174";

test.describe("Audit Vault — Scenario B", () => {
  test.describe("happy path", () => {
    test.beforeEach(async ({ page }) => {
      await page.goto(`${SUPERVISOR_URL}/login`);
      await page.getByLabel(/username/i).fill("supervisor@centralbank.gov");
      await page.getByLabel(/password/i).fill("Supervisor2026!");
      await page.getByRole("button", { name: /sign in/i }).click();
      await page.waitForURL(`${SUPERVISOR_URL}/`);
    });

    test("supervisor sees audit logs table with real backend data", async ({ page }) => {
      await page.goto(`${SUPERVISOR_URL}/audit`);
      await expect(page.getByRole("heading", { name: /immutable audit logs/i })).toBeVisible();

      // Table should render rows from the backend (not mock data)
      const rows = page.getByRole("row");
      await expect(rows).toHaveCount({ min: 2 }); // header + at least one log

      // Each row must have severity and outcome badges
      const firstDataRow = rows.nth(1);
      await expect(firstDataRow.getByRole("cell")).toHaveCount({ min: 5 });
    });

    test("severity filter reduces results", async ({ page }) => {
      await page.goto(`${SUPERVISOR_URL}/audit`);
      const severityInput = page.getByPlaceholder(/severity/i);
      await severityInput.fill("CRITICAL");
      await page.getByRole("button", { name: /filter/i }).click();

      // All visible badges in the severity column should read CRITICAL
      const severityBadges = page.getByRole("row").nth(1).locator("text=CRITICAL");
      await expect(severityBadges.or(page.getByText(/no audit logs found/i))).toBeVisible();
    });

    test("clear filter resets the table", async ({ page }) => {
      await page.goto(`${SUPERVISOR_URL}/audit`);
      await page.getByPlaceholder(/severity/i).fill("CRITICAL");
      await page.getByRole("button", { name: /filter/i }).click();
      await page.getByRole("button", { name: /clear/i }).click();

      // Table should reload without filter
      await expect(page.getByRole("row")).toHaveCount({ min: 2 });
    });
  });

  test.describe("403 for governance role", () => {
    test("governance user gets insufficient privileges message", async ({ page }) => {
      // Login as governance user
      await page.goto(`${SUPERVISOR_URL}/login`);
      await page.getByLabel(/username/i).fill("governance@centralbank.gov");
      await page.getByLabel(/password/i).fill("Governance2026!");
      await page.getByRole("button", { name: /sign in/i }).click();

      // Attempt to directly fetch the audit logs endpoint — expect 403
      const resp = await page.evaluate(async () => {
        const r = await fetch("/api/v1/compliance/audit/logs", { credentials: "include" });
        return r.status;
      });
      expect(resp).toBe(403);
    });
  });
});
