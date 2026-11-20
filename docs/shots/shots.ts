/*
 * The docs' product screenshots, captured by capture.ts. Each one opens a
 * path in the Lumen demo workspace, waits for a piece of text, optionally
 * prepares the screen (clicks, typing), and saves the whole window or a
 * crop. Pages show a shot with <Screenshot name="…" alt="…" />.
 *
 * Besides what `marketing:demo` seeds, these shots expect, in the Lumen
 * workspace: a webhook endpoint for https://hooks.acme.example/dimmi with a
 * test delivery, a sending domain lumen.example and an identity secret
 * named Production. Create them with the API (see the commands in
 * docs/shots/README.md).
 */
import type { Page } from "@playwright/test";

export const VIEWPORT = { width: 1440, height: 900 };

export type Clip = { x: number; y: number; width: number; height: number };

export type Crop =
  | Clip
  /** The settings section (or other <section>) whose heading has this exact text. */
  | { section: string; pad?: number }
  /** The open dialog. */
  | { dialog: true; pad?: number };

export type Shot = {
  name: string;
  path: string;
  /** Text that shows once the screen has loaded. */
  waitFor?: string;
  prepare?: (page: Page) => Promise<void>;
  /** Undoes what prepare left in the shared demo data, such as a saved draft. */
  cleanup?: (page: Page) => Promise<void>;
  crop?: Crop;
};

const click = (text: string) => async (page: Page) => {
  await page.getByText(text, { exact: true }).first().click();
  await page.waitForTimeout(800);
};

export const shots: Shot[] = [
  {
    name: "messenger-install",
    path: "/d/lumen/settings/messenger",
    waitFor: "Website",
    prepare: click("Website"),
    crop: { section: "Install" },
  },
  {
    name: "inbox-reply",
    path: "/d/lumen/inbox/unassigned",
    prepare: async (page) => {
      await page.locator('a[href*="/inbox/unassigned/conv_"]').first().click();
      await page.waitForTimeout(1200);
      const close = page.getByRole("button", { name: /close operator/i });
      if (await close.isVisible().catch(() => false)) {
        await close.click();
      }
      await page
        .getByText(/^Reply to /)
        .first()
        .click();
      await page.waitForTimeout(500);
      // Drafts are saved on the server, so clear what an earlier run left.
      await page.keyboard.press("ControlOrMeta+A");
      await page.keyboard.press("Backspace");
      await page.keyboard.type("Thanks for the details. I'm looking into it now and will reply here shortly.");
    },
    cleanup: async (page) => {
      await page.keyboard.press("ControlOrMeta+A");
      await page.keyboard.press("Backspace");
      await page.waitForTimeout(1500);
    },
  },
  {
    name: "webhook-deliveries",
    path: "/d/lumen/settings/webhooks",
    waitFor: "https://hooks.acme.example/dimmi",
    prepare: click("https://hooks.acme.example/dimmi"),
    crop: { section: "Recent deliveries" },
  },
  {
    name: "sending-domain-records",
    path: "/d/lumen/settings/email",
    waitFor: "lumen.example",
    prepare: click("lumen.example"),
    crop: { section: "DNS records" },
  },
  {
    name: "identity-secrets",
    path: "/d/lumen/settings/identity",
    waitFor: "Secrets",
    crop: { section: "Secrets" },
  },
];
