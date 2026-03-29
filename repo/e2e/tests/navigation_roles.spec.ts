import { expect, test } from "@playwright/test";
import { login, requireAdminCreds } from "./helpers";

test("admin navigation shows key governance links", async ({ page }) => {
  const creds = requireAdminCreds();
  await login(page, creds.username, creds.password);
  await expect(page.getByRole("link", { name: "Budgets" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Members" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Users" })).toBeVisible();
});
