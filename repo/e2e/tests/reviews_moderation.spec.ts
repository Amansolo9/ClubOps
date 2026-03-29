import { expect, test } from "@playwright/test";
import { login, requireAdminCreds } from "./helpers";

test("reviews list shows moderation reason inputs for operators", async ({ page }) => {
  const creds = requireAdminCreds();
  await login(page, creds.username, creds.password);
  await page.goto("/reviews");
  await expect(page.locator("input[placeholder='Hide reason']").first()).toBeVisible();
  await expect(page.locator("input[placeholder='Reinstate reason']").first()).toBeVisible();
});
