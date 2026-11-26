/*
 * The docs' product screenshots, captured by capture.ts. Each one opens a
 * path in the Lumen demo workspace, waits for a piece of text, optionally
 * prepares the screen (clicks, typing), optionally pins numbered callouts
 * to key UI elements, and saves the whole window or a crop. Pages show a
 * shot with <Screenshot name="…" alt="…" />.
 *
 * Besides what `db:seed` creates, these shots expect, in the Lumen
 * workspace: a webhook endpoint for https://hooks.acme.example/dimmi with a
 * test delivery, a sending domain lumen.example and an identity secret
 * named Production. Create them with the API (see the commands in
 * docs/shots/README.md).
 */
import type { Locator, Page } from "@playwright/test";

export const VIEWPORT = { width: 1440, height: 900 };

export type Clip = { x: number; y: number; width: number; height: number };

export type Crop =
  | Clip
  | ((page: Page) => Promise<Clip>)
  /** The settings section (or other <section>) whose heading has this exact text. */
  | { section: string; pad?: number }
  /** The open dialog. */
  | { dialog: true; pad?: number };

export type Annotation = {
  label: string;
  target: (page: Page) => Locator;
  anchor?: "top-left" | "top-right" | "bottom-left" | "bottom-right" | "middle-left" | "middle-right";
  dx?: number;
  dy?: number;
};

export type Shot = {
  name: string;
  path: string;
  /** Text that shows once the screen has loaded. */
  waitFor?: string;
  prepare?: (page: Page) => Promise<void>;
  /** Undoes what prepare left in the shared demo data, such as a saved draft. */
  cleanup?: (page: Page) => Promise<void>;
  annotations?: Annotation[];
  crop?: Crop;
};

const click = (text: string) => async (page: Page) => {
  await page.getByText(text, { exact: true }).first().click();
  await page.waitForTimeout(800);
};

async function openDuplicateCharge(page: Page, showTesting: boolean) {
  await page.getByText("Refund a duplicate charge", { exact: true }).first().click();
  await page.getByText("When dimmi uses this", { exact: true }).first().waitFor();
  const hide = page.getByRole("button", { name: "Hide testing", exact: true });
  const show = page.getByRole("button", { name: "Show testing", exact: true });
  if (showTesting && (await show.isVisible().catch(() => false))) {
    await show.click();
    await page.waitForTimeout(300);
  } else if (!showTesting && (await hide.isVisible().catch(() => false))) {
    await hide.click();
    await page.waitForTimeout(300);
  }
}

async function procedurePanels(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll("div, section, aside, main")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return (
          r.height > 750 &&
          r.width > 200 &&
          r.width < 1400 &&
          Number.parseFloat(getComputedStyle(el).borderTopLeftRadius) >= 12
        );
      })
      .map((el) => {
        const r = el.getBoundingClientRect();
        return { x: r.x, y: r.y, width: r.width, height: r.height };
      })
      .sort((a, b) => a.x - b.x),
  );
}

function column(text: string, above: number, height: number) {
  return async (page: Page): Promise<Clip> => {
    const label = page.getByText(text, { exact: true }).first();
    const box = await label.evaluate((el) => {
      let node: Element = el;
      while (node.parentElement && node.getBoundingClientRect().width < 560) {
        node = node.parentElement;
      }
      const col = node.getBoundingClientRect();
      return { x: col.x, width: col.width, top: el.getBoundingClientRect().top };
    });
    const y = Math.max(0, box.top - above);
    return {
      x: Math.max(0, box.x - 20),
      y,
      width: Math.min(VIEWPORT.width - Math.max(0, box.x - 20), box.width + 40),
      height: Math.min(height, VIEWPORT.height - y),
    };
  };
}

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
  {
    name: "procedure-editor",
    path: "/d/lumen/procedures",
    waitFor: "Refund a duplicate charge",
    prepare: async (page) => {
      await openDuplicateCharge(page, false);
      await page.getByRole("tab", { name: "History", exact: true }).click();
      await page.getByRole("tab", { name: "Steps", exact: true }).click();
      await page.getByText("is at least").first().waitFor();
    },
    annotations: [
      {
        label: "1",
        target: (page) => page.getByText("When dimmi uses this", { exact: true }).first(),
        anchor: "middle-left",
        dx: -16,
      },
      {
        label: "2",
        target: (page) => page.getByText("Look up charges", { exact: true }).first(),
        anchor: "middle-right",
        dx: 22,
        dy: 0,
      },
      {
        label: "3",
        target: (page) => page.getByText("Match all", { exact: true }).first(),
        anchor: "middle-left",
        dx: -18,
      },
      {
        label: "4",
        target: (page) => page.getByRole("button", { name: "Show testing", exact: true }).first(),
        anchor: "bottom-left",
        dx: -10,
        dy: 10,
      },
    ],
    crop: async (page) => {
      const all = await procedurePanels(page);
      const main = all[0];
      return { x: Math.max(0, main.x), y: 0, width: Math.min(VIEWPORT.width - main.x, main.width), height: 880 };
    },
  },
  {
    name: "procedure-rules",
    path: "/d/lumen/procedures",
    waitFor: "Refund a duplicate charge",
    prepare: async (page) => {
      await openDuplicateCharge(page, false);
      await page.getByRole("tab", { name: "History", exact: true }).click();
      await page.getByRole("tab", { name: "Steps", exact: true }).click();
      await page.getByText("is at least").first().waitFor();
      await page.getByText("Billing", { exact: true }).first().scrollIntoViewIfNeeded();
      await page.waitForTimeout(250);
    },
    annotations: [
      {
        label: "1",
        target: (page) => page.getByText("Match all", { exact: true }).first(),
        anchor: "middle-left",
        dx: -18,
      },
      {
        label: "2",
        target: (page) => page.getByText("is at least", { exact: true }).first(),
        anchor: "top-right",
        dx: 14,
        dy: -8,
      },
      { label: "3", target: (page) => page.getByText("Then", { exact: true }).first(), anchor: "middle-left", dx: -16 },
      { label: "4", target: (page) => page.getByText("Else", { exact: true }).first(), anchor: "middle-left", dx: -16 },
    ],
    crop: column("Match all", 24, 540),
  },
  {
    name: "procedure-simulations",
    path: "/d/lumen/procedures",
    waitFor: "Refund a duplicate charge",
    prepare: async (page) => {
      await openDuplicateCharge(page, true);
      await page.getByRole("tab", { name: /Simulations/ }).click();
      await page.getByText("Duplicate $49 charge in March", { exact: true }).first().click();
      await page.getByText("explains the 5 to 10 day card timeline").first().waitFor();
    },
    annotations: [
      { label: "1", target: (page) => page.getByText("Pass", { exact: true }).first(), anchor: "middle-left", dx: -14 },
      {
        label: "2",
        target: (page) => page.getByText("Called Look up charges", { exact: true }).first(),
        anchor: "middle-left",
        dx: -14,
      },
      {
        label: "3",
        target: (page) => page.getByText("Checks", { exact: true }).first(),
        anchor: "middle-left",
        dx: -14,
      },
      {
        label: "4",
        target: (page) => page.getByRole("textbox", { name: "Add a check" }).first(),
        anchor: "middle-left",
        dx: -14,
      },
    ],
    crop: async (page) => {
      const all = await procedurePanels(page);
      const side = all[all.length - 1];
      return {
        x: Math.max(0, side.x - 16),
        y: 0,
        width: Math.min(VIEWPORT.width - Math.max(0, side.x - 16), side.width + 20),
        height: 760,
      };
    },
  },
  {
    name: "procedure-history",
    path: "/d/lumen/procedures",
    waitFor: "Refund a duplicate charge",
    prepare: async (page) => {
      await openDuplicateCharge(page, false);
      await page.getByRole("tab", { name: "History", exact: true }).click();
      await page.getByRole("button", { name: "Version 1 options", exact: true }).waitFor();
      await page.getByRole("button", { name: "Version 1 options", exact: true }).click();
      await page.getByText("Compare with working copy", { exact: true }).click();
      await page.getByText("Added").first().waitFor();
    },
    annotations: [
      {
        label: "1",
        target: (page) => page.getByRole("button", { name: /Compare from/i }).first(),
        anchor: "middle-left",
        dx: -16,
      },
      {
        label: "2",
        target: (page) => page.getByText("Added", { exact: true }).first(),
        anchor: "middle-right",
        dx: 52,
        dy: 0,
      },
      {
        label: "3",
        target: (page) => page.getByRole("button", { name: "Version 2 options", exact: true }).first(),
        anchor: "middle-left",
        dx: -16,
      },
    ],
    crop: column("Compare", 24, 680),
  },
];
