import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { operator } from "./support/operator";

it.describe("single sign-on", () => {
  it("an owner starts a connection and reads what the identity provider needs", async ({ page, person, request }) => {
    const suffix = person.email.split("@")[0]?.split(".").at(-1) ?? "";
    await createAccount(request, person, `SSO ${suffix}`);
    await signIn(page, person);

    await page.goto("/d/settings/single-sign-on");
    const connections = page.getByRole("region", { name: "Connections" });
    await expect(connections.getByText("No connections yet.")).toBeVisible();
    await connections.getByRole("button", { name: "New connection" }).click();

    const dialog = page.getByRole("dialog", { name: "New connection" });
    await dialog.getByLabel("Name").fill("Okta");
    await dialog.getByLabel("Identity provider entity ID").fill("http://www.okta.com/exk1example");
    await dialog.getByLabel("Sign-in URL").fill("https://acme.okta.com/app/sso/saml");
    await dialog.getByRole("button", { name: "Create connection" }).click();
    await expect(dialog).toBeHidden();

    await expect(connections.getByText("Okta")).toBeVisible();
    await expect(connections.getByText("Draft")).toBeVisible();
    await connections.getByRole("button", { name: "Details" }).click();
    const details = page.getByRole("dialog", { name: "Okta" });
    await expect(details.getByText("Reply URL (ACS)")).toBeVisible();
    await expect(details.getByText(/\/auth\/saml\/.+\/acs$/)).toBeVisible();
    await details.getByRole("button", { name: "Done" }).click();
  });

  it("an owner verifies a domain with the operator command and sees it verified", async ({ page, person, request }) => {
    const suffix = person.email.split("@")[0]?.split(".").at(-1) ?? "";
    const domain = `acme-${suffix}-${Date.now()}.com`;
    await createAccount(request, person, `Domains ${suffix}`);
    await signIn(page, person);

    await page.goto("/d/settings/single-sign-on");
    const domains = page.getByRole("region", { name: "Domains" });
    await domains.getByRole("button", { name: "Add domain" }).click();
    const add = page.getByRole("alertdialog");
    await add.getByLabel("Domain").fill(domain);
    await add.getByRole("button", { name: "Add domain" }).click();

    await expect(domains.getByText(domain, { exact: true })).toBeVisible();
    await expect(domains.getByText("Not verified")).toBeVisible();
    await domains.getByRole("button", { name: "Verify" }).click();
    const record = page.getByRole("dialog", { name: `Verify ${domain}` });
    await expect(record.getByText(`_dimmi-verify.${domain}`)).toBeVisible();
    await expect(record.getByText(/^dimmi-domain-verification=/)).toBeVisible();
    await record.getByRole("button", { name: "Done" }).click();

    expect(operator("sso:verify-domain", domain)).toContain(`Verified ${domain}`);

    await page.reload();
    await expect(domains.getByText("Verified", { exact: true })).toBeVisible();
    await expect(domains.getByText("Verified by the operator")).toBeVisible();
    await expect(domains.getByRole("button", { name: `Connection for ${domain}` })).toContainText("No connection");
  });
});
