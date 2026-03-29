import { Page } from "@playwright/test";

export function requireAdminCreds(): { username: string; password: string } {
  const username = process.env.E2E_ADMIN_USER || "";
  const password = process.env.E2E_ADMIN_PASS || "";
  if (!username || !password) {
    throw new Error("E2E_ADMIN_USER and E2E_ADMIN_PASS are required for authenticated E2E runs");
  }
  return { username, password };
}

export async function login(page: Page, username: string, password: string): Promise<void> {
  await page.goto("/login");
  await page.fill("input[name='username']", username);
  await page.fill("input[name='password']", password);
  await page.getByRole("button", { name: "Login" }).click();
  await page.waitForLoadState("networkidle");
}
