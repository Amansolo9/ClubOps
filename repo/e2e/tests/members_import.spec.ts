import { expect, test } from "@playwright/test";
import { login, requireAdminCreds } from "./helpers";

test("members import shows downloadable error report link on invalid rows", async ({ page }) => {
  const creds = requireAdminCreds();
  await login(page, creds.username, creds.password);
  await page.goto("/members");
  const fileChooserPromise = page.waitForEvent("filechooser");
  await page.locator("input[type='file'][name='file']").first().click();
  const chooser = await fileChooserPromise;
  await chooser.setFiles({
    name: "members.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields\nA,a@x.com,1,2026-03-01,Role,true,G,{bad}\n")
  });
  await page.getByRole("button", { name: "Import" }).first().click();
  await expect(page.getByRole("link", { name: "Download error report CSV" })).toBeVisible();
});
