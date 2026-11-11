import { createHash, randomBytes } from "node:crypto";

import { createAccount, expect, test as it, signIn } from "./support/fixtures";
import { mailedLink } from "./support/mail";

const callback = "http://127.0.0.1:43210/callback";

/*
 * pkce returns a fresh verifier and its S256 challenge.
 */
function pkce() {
  const verifier = randomBytes(32).toString("base64url");

  return { verifier, challenge: createHash("sha256").update(verifier).digest("base64url") };
}

it.describe("Connected apps", () => {
  it("connects an agent with OAuth, limits it to the access chosen and disconnects it", async ({
    page,
    person,
    request,
  }) => {
    await createAccount(request, person, `Apps ${person.email.split("@")[0]?.split(".").at(-1)}`);
    await signIn(page, person);
    await page.goto(await mailedLink(person.email, "Verify"));
    await expect(page.getByRole("heading", { name: "Email verified" })).toBeVisible();

    const registered = await request.post("/oauth/register", {
      data: {
        client_name: "Agent Tester",
        redirect_uris: [callback],
        token_endpoint_auth_method: "none",
        grant_types: ["authorization_code", "refresh_token"],
        response_types: ["code"],
      },
    });
    expect(registered.status()).toBe(201);
    const clientId: string = (await registered.json()).client_id;

    const answers: URL[] = [];
    await page.context().route(`${callback}**`, (route) => {
      answers.push(new URL(route.request().url()));
      return route.fulfill({ status: 200, contentType: "text/plain", body: "connected" });
    });

    const authorize = (challenge: string) =>
      `/oauth/authorize?${new URLSearchParams({
        response_type: "code",
        client_id: clientId,
        redirect_uri: callback,
        scope: "conversations:write contacts:read offline_access",
        state: "xyz",
        code_challenge: challenge,
        code_challenge_method: "S256",
      })}`;

    const denied = pkce();
    await page.goto(authorize(denied.challenge));
    await expect(page.getByRole("heading", { name: "Connect Agent Tester" })).toBeVisible();
    await page.getByRole("button", { name: "Deny" }).click();
    await page.waitForURL((url) => url.href.startsWith(callback) && url.searchParams.has("error"));
    expect(answers).toHaveLength(1);
    expect(answers[0]?.searchParams.get("error")).toBe("access_denied");

    const { verifier, challenge } = pkce();
    await page.goto(authorize(challenge));
    await expect(page.getByRole("heading", { name: "Connect Agent Tester" })).toBeVisible();
    await expect(page.getByText("dimmi can't verify who made this app.", { exact: false })).toBeVisible();
    await expect(page.getByText("This app runs on your computer.", { exact: false })).toBeVisible();
    await expect(page.getByRole("checkbox", { name: "Conversations: write" })).toBeChecked();
    await page.getByText("Contacts: read", { exact: true }).click();
    await expect(page.getByRole("checkbox", { name: "Contacts: read" })).not.toBeChecked();
    await page.getByRole("button", { name: "Allow" }).click();
    await page.waitForURL((url) => url.href.startsWith(callback) && url.searchParams.has("code"));
    expect(answers).toHaveLength(2);
    const answer = answers[1];
    expect(answer?.searchParams.get("state")).toBe("xyz");
    const code = answer?.searchParams.get("code") ?? "";
    expect(code).not.toBe("");

    const exchanged = await request.post("/oauth/token", {
      form: {
        grant_type: "authorization_code",
        code,
        client_id: clientId,
        redirect_uri: callback,
        code_verifier: verifier,
      },
    });
    expect(exchanged.status()).toBe(200);
    const tokens = await exchanged.json();
    expect(tokens.scope).toContain("conversations:write");
    expect(tokens.scope).not.toContain("contacts:read");

    const bearer = { Authorization: `Bearer ${tokens.access_token}` };
    expect((await request.get("/api/v1/conversations", { headers: bearer })).status()).toBe(200);
    expect((await request.get("/api/v1/contacts", { headers: bearer })).status()).toBe(403);

    await page.goto("/d/settings/connected-apps");
    await expect(page.getByRole("heading", { name: "Connected apps" })).toBeVisible();
    await expect(page.getByText("Agent Tester", { exact: true })).toBeVisible();
    await expect(page.getByText(/Conversations: write · Connected/)).toBeVisible();

    await page.getByRole("button", { name: "Disconnect" }).click();
    const confirm = page.getByRole("alertdialog", { name: "Disconnect Agent Tester?" });
    await confirm.getByRole("button", { name: "Disconnect" }).click();
    await expect(page.getByText(/No apps yet/)).toBeVisible();

    expect((await request.get("/api/v1/conversations", { headers: bearer })).status()).toBe(401);
  });
});
