import { expect, test } from "@playwright/test";
import { login, requireAdminCreds } from "./helpers";

test("budget projection renders readable projection card", async ({ page }) => {
  const creds = requireAdminCreds();
  await login(page, creds.username, creds.password);
  await page.goto("/budgets");
  await page.fill("input[name='budget_id']", "1");
  await page.fill("input[name='expected_remaining_spend']", "100");
  await page.getByRole("button", { name: "Preview Projection" }).click();
  await expect(page.locator("#projection-result")).toContainText("Projected end balance");
});
