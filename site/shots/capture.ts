/*
 * Captures the marketing site's product shots from a running dimmi: it
 * signs in to the Lumen demo workspace that `db:seed` creates, opens
 * each screen, and saves it at twice the pixel density in light and
 * dark mode as WebP into site/public/shots.
 *
 *   ./tmp/dimmi-shots db:seed --only lumen --reset
 *   node site/shots/capture.ts [--only inbox]
 *
 * SHOTS_URL points at the app (default http://localhost:8080). The demo
 * owner signs in with the password db:seed prints. When it finishes it
 * runs downscale.ts, which writes each shot's half-width .1x.webp.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { chromium, type Page } from "@playwright/test";

import { downscaleShots } from "./downscale.ts";

const base = process.env.SHOTS_URL ?? "http://localhost:8080";
const email = process.env.SHOTS_EMAIL ?? "maya@lumen.test";
const password = process.env.SHOTS_PASSWORD ?? "stowline-demo-2026";
const out = fileURLToPath(new URL("../public/shots/", import.meta.url));
const only = process.argv.includes("--only") ? process.argv[process.argv.indexOf("--only") + 1] : undefined;

type Clip = { x: number; y: number; width: number; height: number };

/*
 * Each step puts the app in a state, then saves one or more pictures of
 * it: the whole window, or a crop given in CSS pixels of the 1680×1000
 * window. Steps run in order on one page, so later ones build on earlier
 * ones.
 */
type Step = {
  prepare: (page: Page) => Promise<void>;
  save: { name: string; clip?: (page: Page) => Promise<Clip> }[];
};

/*
 * The app's panels (list, thread, details, Operator) from left to right,
 * measured live so crops follow the layout whatever is open.
 */
async function panels(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll("div, section, aside, main")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return (
          r.height > 850 &&
          r.width > 200 &&
          r.width < 1200 &&
          Number.parseFloat(getComputedStyle(el).borderTopLeftRadius) >= 12
        );
      })
      .map((el) => {
        const r = el.getBoundingClientRect();
        return { x: r.x, width: r.width };
      })
      .sort((a, b) => a.x - b.x),
  );
}

/* A crop around panels first..last, padded, from the top of the window. */
function around(first: number, last: number, height: number) {
  return async (page: Page): Promise<Clip> => {
    const all = await panels(page);
    const from = all[first];
    const to = all[Math.min(last, all.length - 1)];
    const x = Math.max(0, from.x - 10);
    return { x, y: 0, width: Math.min(1680, to.x + to.width + 10) - x, height };
  };
}

/* A crop around every node on the workflow canvas, padded. */
function nodes(pad: number) {
  return async (page: Page): Promise<Clip> => {
    const box = await page.evaluate(() => {
      const rects = [...document.querySelectorAll(".react-flow__node")].map((el) => el.getBoundingClientRect());
      return {
        left: Math.min(...rects.map((r) => r.left)),
        top: Math.min(...rects.map((r) => r.top)),
        right: Math.max(...rects.map((r) => r.right)),
        bottom: Math.max(...rects.map((r) => r.bottom)),
      };
    });
    return {
      x: box.left - pad,
      y: box.top - pad,
      width: box.right - box.left + pad * 2,
      height: box.bottom - box.top + pad * 2,
    };
  };
}

/*
 * A crop of the column of settings a piece of text sits in: as wide as
 * the column, starting `above` pixels over the text and `height` tall.
 */
function column(text: string, above: number, height: number) {
  return async (page: Page): Promise<Clip> => {
    const label = page.getByText(text, { exact: true }).first();
    const box = await label.evaluate((el) => {
      let node: Element = el;
      while (node.parentElement && node.getBoundingClientRect().width < 560) {
        node = node.parentElement;
      }
      const column = node.getBoundingClientRect();
      return { x: column.x, width: column.width, top: el.getBoundingClientRect().top };
    });
    const y = Math.max(0, box.top - above);
    return { x: box.x - 16, y, width: box.width + 32, height: Math.min(height, 1000 - y) };
  };
}

const operator = (page: Page) => page.getByRole("button", { name: "Operator", exact: true });

async function closeMenus(page: Page) {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);
}

const steps: Step[] = [
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/inbox/all`);
      await page.getByRole("radio", { name: "Open", exact: true }).click();
      await page.getByText("Omar Haddad").first().click();
      await page.getByText("Thanks Omar, I can see it.").first().waitFor();
      // The Operator panel stays shut so the list, thread and details fit.
      const close = page.getByRole("button", { name: /close operator/i });
      if (await close.isVisible().catch(() => false)) {
        await close.click();
      }
    },
    save: [
      { name: "inbox" },
      { name: "inbox-list", clip: around(0, 0, 660) },
      { name: "inbox-details", clip: around(2, 2, 640) },
    ],
  },
  {
    prepare: async (page) => {
      await operator(page).click();
      await page.getByRole("button", { name: "Brief me" }).waitFor();
    },
    save: [{ name: "inbox-operator", clip: around(2, 3, 660) }],
  },
  {
    prepare: async (page) => {
      await page.getByRole("button", { name: "Assign", exact: true }).click();
      await page.getByText("Teammates").first().waitFor();
    },
    save: [{ name: "inbox-assign", clip: around(1, 1, 540) }],
  },
  {
    prepare: async (page) => {
      await closeMenus(page);
      await page.getByRole("button", { name: "Snooze", exact: true }).click();
      await page.getByText("Pick date and time").first().waitFor();
    },
    save: [{ name: "inbox-snooze", clip: around(1, 1, 540) }],
  },
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/workflows`);
      await page.getByText("Sort new conversations by topic").first().click();
      await page.getByText("Sort into Billing, Bug").first().waitFor();
    },
    save: [{ name: "workflow" }, { name: "workflow-canvas", clip: nodes(28) }],
  },
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/procedures`);
      await page.getByText("Refund a duplicate charge").first().click();
      await page.getByText("When dimmi uses this").first().waitFor();
      const hideTesting = page.getByRole("button", { name: "Hide testing", exact: true });
      if (await hideTesting.isVisible().catch(() => false)) {
        await hideTesting.click();
        await page.waitForTimeout(300);
      }
      await page.getByRole("tab", { name: "History", exact: true }).click();
      await page.getByRole("tab", { name: "Steps", exact: true }).click();
      await page.getByText("is at least").first().waitFor();
    },
    save: [{ name: "procedure", clip: column("When dimmi uses this", 16, 800) }],
  },
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/procedures`);
      await page.getByText("Refund a duplicate charge").first().click();
      await page.getByText("When dimmi uses this").first().waitFor();
      const showTesting = page.getByRole("button", { name: "Show testing", exact: true });
      if (await showTesting.isVisible().catch(() => false)) {
        await showTesting.click();
        await page.waitForTimeout(300);
      }
      await page.getByRole("tab", { name: /Simulations/ }).click();
      await page.getByText("Duplicate $49 charge in March").first().click();
      await page.getByText("explains the 5 to 10 day card timeline").first().waitFor();
    },
    save: [
      {
        name: "procedure-simulations",
        clip: async (page) => {
          const all = await panels(page);
          const side = all[all.length - 1];
          return {
            x: Math.max(0, side.x - 4),
            y: 0,
            width: Math.min(1680 - Math.max(0, side.x - 4), side.width + 8),
            height: 740,
          };
        },
      },
    ],
  },
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/procedures`);
      await page.getByText("Refund a duplicate charge").first().click();
      await page.getByText("When dimmi uses this").first().waitFor();
      const hideTesting = page.getByRole("button", { name: "Hide testing", exact: true });
      if (await hideTesting.isVisible().catch(() => false)) {
        await hideTesting.click();
        await page.waitForTimeout(300);
      }
      await page.getByRole("tab", { name: "History", exact: true }).click();
      await page.getByRole("button", { name: "Version 1 options", exact: true }).waitFor();
      await page.getByRole("button", { name: "Version 1 options", exact: true }).click();
      await page.getByText("Compare with working copy", { exact: true }).click();
      await page.getByText("Added").first().waitFor();
    },
    save: [{ name: "procedure-history", clip: column("Compare", 24, 680) }],
  },
  {
    prepare: async (page) => {
      await page.goto(`${base}/d/lumen/settings/agent`);
      await page.getByText("Guidance", { exact: true }).first().waitFor();
    },
    save: [{ name: "agent-voice", clip: column("Guidance", 250, 486) }],
  },
  {
    // Leave the Operator panel shut for the next run.
    prepare: async (page) => {
      await closeMenus(page);
      await operator(page).click();
    },
    save: [],
  },
];

/*
 * Flows are short click-throughs of the real app, saved as frames. Each
 * action moves the pointer to an element and clicks it, hovers it or
 * types into it; a frame is saved after each one, with where the pointer
 * went, so the site can replay it with an animated cursor. Flows only
 * open menus, switch tabs and hover, so they never change the demo data.
 */
type Action = { kind: "click" | "hover" | "type"; target: (page: Page) => Locator; text?: string };
type Flow = {
  name: string;
  setup: (page: Page) => Promise<void>;
  clip: (page: Page) => Promise<Clip>;
  actions: Action[];
};

async function openConversation(page: Page, withOperator = false) {
  await page.goto(`${base}/d/lumen/inbox/all`);
  // The list remembers its last tab; start every flow from Open.
  await page.getByRole("radio", { name: "Open", exact: true }).click();
  await page.getByText("Omar Haddad").first().click();
  await page.getByText("Thanks Omar, I can see it.").first().waitFor();
  const close = page.getByRole("button", { name: /close operator/i });
  const open = await close.isVisible().catch(() => false);
  if (open !== withOperator) {
    await operator(page).click();
  }
  await page.waitForTimeout(600);
}

/* The visible element with this exact text; menus render last, so the last one wins. */
const text = (value: string) => (page: Page) => page.getByText(value, { exact: true }).filter({ visible: true }).last();

const flows: Flow[] = [
  {
    name: "queue",
    setup: (page) => openConversation(page),
    clip: around(0, 0, 560),
    actions: [
      { kind: "click", target: (page) => page.getByRole("radio", { name: "Snoozed", exact: true }) },
      { kind: "click", target: (page) => page.getByRole("radio", { name: "Closed", exact: true }) },
      { kind: "click", target: (page) => page.getByRole("radio", { name: "Open", exact: true }) },
    ],
  },
  {
    name: "details",
    setup: (page) => openConversation(page),
    clip: around(2, 2, 560),
    actions: [
      {
        kind: "click",
        target: (page) => page.locator('button[aria-label="Priority"]').filter({ visible: true }).last(),
      },
      { kind: "hover", target: (page) => page.getByRole("option").filter({ visible: true }).nth(1) },
      { kind: "hover", target: (page) => page.getByRole("option").filter({ visible: true }).nth(3) },
    ],
  },
  {
    name: "assign",
    setup: (page) => openConversation(page),
    clip: around(1, 1, 540),
    actions: [
      {
        kind: "click",
        target: (page) => page.getByRole("button", { name: "Assign", exact: true }).filter({ visible: true }),
      },
      { kind: "hover", target: text("Kofi Mensah") },
      { kind: "hover", target: text("Lena Fischer") },
    ],
  },
  {
    name: "snooze",
    setup: (page) => openConversation(page),
    clip: around(1, 1, 540),
    actions: [
      {
        kind: "click",
        target: (page) => page.getByRole("button", { name: "Snooze", exact: true }).filter({ visible: true }),
      },
      { kind: "hover", target: text("Tomorrow") },
      { kind: "hover", target: text("Next week") },
    ],
  },
  {
    name: "operator",
    setup: (page) => openConversation(page, true),
    clip: async (page) => {
      const box = await around(3, 3, 1000)(page);
      return { ...box, y: 300, height: 700 };
    },
    actions: [
      { kind: "click", target: (page) => page.getByPlaceholder(/Ask Operator/) },
      { kind: "type", target: (page) => page.getByPlaceholder(/Ask Operator/), text: "Any past " },
      { kind: "type", target: (page) => page.getByPlaceholder(/Ask Operator/), text: "webhook issues?" },
    ],
  },
];

type Frame = { next?: { x: number; y: number; kind: Action["kind"] } };

async function shoot(page: Page, file: string, clip: Clip) {
  await page.waitForTimeout(500);
  const cdp = await page.context().newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 92,
    clip: { ...clip, scale: 2 },
  });
  writeFileSync(file, Buffer.from(data, "base64"));
}

async function runFlow(page: Page, flow: Flow, scheme: string) {
  await flow.setup(page);
  await page.mouse.move(2, 998);
  const clip = await flow.clip(page);
  const frames: Frame[] = [{}];
  await shoot(page, `${out}flow-${flow.name}-0-${scheme}.webp`, clip);

  for (const [i, action] of flow.actions.entries()) {
    const target = action.target(page);
    const box = await target.boundingBox();
    if (!box) {
      throw new Error(`${flow.name}: action ${i + 1} found nothing to ${action.kind}`);
    }
    const x = box.x + Math.min(box.width / 2, 40);
    const y = box.y + box.height / 2;
    frames[i].next = { x: Math.round(x - clip.x), y: Math.round(y - clip.y), kind: action.kind };
    if (action.kind === "type") {
      await target.pressSequentially(action.text ?? "", { delay: 10 });
    } else {
      await page.mouse.move(x, y, { steps: 4 });
      if (action.kind === "click") {
        await page.mouse.down();
        await page.mouse.up();
      }
    }
    frames.push({});
    await shoot(page, `${out}flow-${flow.name}-${i + 1}-${scheme}.webp`, clip);
  }

  await closeMenus(page);
  return { width: Math.round(clip.width), height: Math.round(clip.height), frames };
}

async function capture(page: Page, file: string, crop?: (page: Page) => Promise<Clip>) {
  await page.mouse.move(2, 998);
  // Let fonts, images and entry motion settle before the capture.
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(900);
  const clip = crop ? await crop(page) : undefined;
  const cdp = await page.context().newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 92,
    ...(clip ? { clip: { ...clip, scale: 2 } } : {}),
  });
  writeFileSync(file, Buffer.from(data, "base64"));
}

mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const manifest: Record<string, Awaited<ReturnType<typeof runFlow>>> = {};

for (const scheme of ["light", "dark"] as const) {
  const context = await browser.newContext({
    viewport: { width: 1680, height: 1000 },
    deviceScaleFactor: 2,
    colorScheme: scheme,
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await page.goto(`${base}/login`);
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL(/\/d\//);
  await page
    .getByRole("button", { name: scheme === "dark" ? "Use dark mode" : "Use light mode" })
    .click()
    .catch(() => undefined);

  for (const step of steps) {
    const wanted = step.save.filter((shot) => !only || shot.name.startsWith(only));
    if (only && wanted.length === 0) {
      continue;
    }
    await step.prepare(page);
    for (const shot of wanted) {
      await capture(page, `${out}${shot.name}-${scheme}.webp`, shot.clip);
      process.stdout.write(`saved ${shot.name}-${scheme}.webp\n`);
    }
  }
  for (const flow of flows) {
    if (only && !`flow-${flow.name}`.startsWith(only)) {
      continue;
    }
    manifest[flow.name] = await runFlow(page, flow, scheme);
    process.stdout.write(`saved flow ${flow.name} (${manifest[flow.name].frames.length} frames, ${scheme})\n`);
  }

  await context.close();
}

await browser.close();

// The light run's pointer positions serve both themes: the layout is the same.
if (Object.keys(manifest).length > 0) {
  const file = fileURLToPath(new URL("../src/flows.json", import.meta.url));
  // Keep flows this run skipped (with --only).
  const kept = existsSync(file) ? JSON.parse(readFileSync(file, "utf8")) : {};
  writeFileSync(file, `${JSON.stringify({ ...kept, ...manifest }, null, 2)}\n`);
}

// Write the half-width variants the site's srcsets serve.
await downscaleShots();
