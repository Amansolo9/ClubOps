import { expect, test } from "@playwright/test";

test("@smoke login and change-password pages render required forms", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Sign In" })).toBeVisible();
  await expect(page.locator("form[hx-post='/login'] input[name='username']")).toBeVisible();
  await expect(page.locator("form[hx-post='/login'] input[name='password']")).toBeVisible();

  await page.goto("/change-password");
  await expect(page.locator("form[hx-post='/api/auth/change-password'] input[name='new_password']")).toBeVisible();
});
