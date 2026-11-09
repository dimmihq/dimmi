/*
 * The public portal's only script. Every page is rendered on the server
 * and works as plain links and forms; Turbo Drive then swaps pages in
 * place, fetching a link as the pointer rests on it and fading between
 * pages, so moving around feels instant and anything marked permanent,
 * such as the messenger, survives the move. The rest are small touches a
 * page reads from its own markup: the greeting follows the visitor's
 * clock, the copy buttons put an article's Markdown on the clipboard, and
 * the theme switch keeps the reader's pick of light, dark or their
 * system's theme in this browser, the browse drawer and dropdowns close
 * on a click outside them or Escape, collections open and close with a slide, and an
 * article's contents mark the heading being read.
 * A reader coming back from signing in to the team's product brings an
 * identity token in the address's fragment, which no server sees, and
 * one coming from the messenger brings a pass there; the sign-in page's
 * form takes either from there and sends it at once.
 */
import "@hotwired/turbo";
import "./portal.css";

const greetings: [hour: number, text: string][] = [
  [18, "Good evening."],
  [12, "Good afternoon."],
  [5, "Good morning."],
  [0, "Good evening."],
];

function greet(root: ParentNode): void {
  const hour = new Date().getHours();
  const text = greetings.find(([from]) => hour >= from)?.[1];
  for (const element of root.querySelectorAll<HTMLElement>("[data-greeting]")) {
    if (text) {
      element.textContent = text;
    }
  }
}

async function copy(button: HTMLButtonElement): Promise<void> {
  const source = button.dataset.copy;
  if (!source) {
    return;
  }

  const label = button.querySelector("[data-copy-label]");
  const before = label?.textContent;
  try {
    const response = await fetch(source, { headers: { Accept: "text/markdown" } });
    await navigator.clipboard.writeText(await response.text());
    if (label) {
      label.textContent = "Copied";
    }
  } catch {
    if (label) {
      label.textContent = "Copy failed";
    }
  }
  setTimeout(() => {
    if (label && before) {
      label.textContent = before;
    }
  }, 1600);
}

/*
 * signIn sends the token or pass the fragment carries, then forgets it.
 */
function signIn(root: ParentNode): void {
  const form = root.querySelector<HTMLFormElement>("form[data-help-sign-in]");
  const fragment = new URLSearchParams(window.location.hash.slice(1));
  const name = ["token", "pass"].find((key) => fragment.get(key));
  const field = name ? form?.querySelector<HTMLInputElement>(`input[name="${name}"]`) : null;
  if (!form || !name || !field) {
    return;
  }
  field.value = fragment.get(name) ?? "";
  history.replaceState(null, "", window.location.pathname + window.location.search);
  form.requestSubmit();
}

const themeKey = "dimmi-portal-theme";

/*
 * storedTheme is the reader's pick, system when they made none or the
 * browser keeps nothing.
 */
function storedTheme(): string {
  try {
    return localStorage.getItem(themeKey) ?? "system";
  } catch {
    return "system";
  }
}

/*
 * showThemes tells assistive technology which theme is picked; the page's
 * own styles already show it from the first paint.
 */
function showThemes(root: ParentNode): void {
  const pick = storedTheme();
  for (const button of root.querySelectorAll<HTMLButtonElement>("button[data-theme-choice]")) {
    button.setAttribute("aria-pressed", String(button.dataset.themeChoice === pick));
  }
}

function pickTheme(button: HTMLButtonElement): void {
  try {
    localStorage.setItem(themeKey, button.dataset.themeChoice ?? "system");
  } catch {
    return;
  }
  (window as { dimmiTheme?: () => void }).dimmiTheme?.();
  showThemes(document);
}

const motion = { duration: 260, easing: "cubic-bezier(0.22, 1, 0.36, 1)" };

/*
 * slide opens or closes a collection row with its height easing, where
 * the reader has not asked for less motion. It reports whether it took
 * over the toggle from the browser.
 */
function slide(summary: HTMLElement): boolean {
  const row = summary.parentElement;
  const body = row?.querySelector<HTMLElement>(":scope > [data-accordion-body]");
  if (!(row instanceof HTMLDetailsElement) || !body || matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return false;
  }
  if (body.getAnimations().length > 0) {
    return true;
  }
  const shut = { height: "0px", paddingBottom: "0px", opacity: 0 };
  if (row.open) {
    summary.querySelector("svg")?.animate([{ rotate: "90deg" }, { rotate: "0deg" }], motion);
    body.animate([{ height: `${body.offsetHeight}px` }, shut], motion).onfinish = () => {
      row.open = false;
    };
    return true;
  }
  row.open = true;
  body.animate([shut, { height: `${body.offsetHeight}px` }], motion);
  return true;
}

const reading = { links: [] as HTMLAnchorElement[], headings: [] as HTMLElement[], frame: 0 };

/*
 * watchContents finds the contents beside the article and the headings
 * they lead to.
 */
function watchContents(root: ParentNode): void {
  reading.links = [...root.querySelectorAll<HTMLAnchorElement>("[data-contents] a[href^='#']")];
  reading.headings = reading.links
    .map((link) => document.getElementById(decodeURIComponent(link.hash.slice(1))))
    .filter((heading): heading is HTMLElement => heading !== null);
  markReading();
}

/*
 * markReading marks the last heading that has scrolled past the top bar,
 * the first before any has, and the last once the page reaches its end.
 */
function markReading(): void {
  reading.frame = 0;
  if (reading.headings.length === 0) {
    return;
  }
  const atEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4;
  const passed = reading.headings.filter((heading) => heading.getBoundingClientRect().top <= 112);
  const current = atEnd ? reading.headings.at(-1) : (passed.at(-1) ?? reading.headings[0]);
  for (const link of reading.links) {
    const active = current !== undefined && link.hash === `#${encodeURIComponent(current.id)}`;
    link.toggleAttribute("aria-current", active);
    if (active) {
      link.setAttribute("aria-current", "location");
      keepInView(link);
    }
  }
}

/*
 * keepInView scrolls the rail beside the article, never the page, so
 * the marked heading stays visible in a long list of contents.
 */
function keepInView(link: HTMLElement): void {
  const rail = link.closest<HTMLElement>("[data-rail]");
  if (!rail) {
    return;
  }
  const top = link.getBoundingClientRect().top - rail.getBoundingClientRect().top + rail.scrollTop;
  if (top < rail.scrollTop + 24 || top > rail.scrollTop + rail.clientHeight - 48) {
    rail.scrollTo({ top: top - rail.clientHeight / 3, behavior: "smooth" });
  }
}

document.addEventListener(
  "scroll",
  () => {
    if (reading.frame === 0) {
      reading.frame = requestAnimationFrame(markReading);
    }
  },
  { passive: true },
);

document.addEventListener("turbo:load", () => {
  greet(document);
  signIn(document);
  showThemes(document);
  watchContents(document);
});

/*
 * closeDrawers shuts the browse drawer.
 */
function closeDrawers(): void {
  for (const drawer of document.querySelectorAll<HTMLDetailsElement>("details[data-drawer][open]")) {
    drawer.open = false;
  }
}

/*
 * closeMenus shuts every open dropdown except the one the reader is
 * using, so a click elsewhere or Escape dismisses it.
 */
function closeMenus(keep?: Element | null): void {
  for (const menu of document.querySelectorAll<HTMLDetailsElement>("details[data-menu][open]")) {
    if (menu !== keep) {
      menu.open = false;
    }
  }
}

/*
 * The drawer sits outside the page it pushes aside, so opening it moves
 * focus into it and closing it hands focus back to the menu button.
 */
document.addEventListener(
  "toggle",
  (event) => {
    const drawer = event.target;
    if (!(drawer instanceof HTMLDetailsElement && drawer.matches("[data-drawer]"))) {
      return;
    }
    if (drawer.open) {
      document.querySelector<HTMLElement>("[data-drawer-panel] input, [data-drawer-panel] button")?.focus();
      return;
    }
    if (document.activeElement?.closest("[data-drawer-panel]")) {
      drawer.querySelector("summary")?.focus();
    }
  },
  true,
);

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeDrawers();
    closeMenus();
  }
});

document.addEventListener("click", (event) => {
  const target = event.target as Element;
  closeMenus(target.closest("details[data-menu]"));
  if (target.closest("[data-drawer-backdrop], [data-drawer-close]")) {
    closeDrawers();
  }
  const summary = target.closest<HTMLElement>("details[data-accordion] > summary");
  if (summary && slide(summary)) {
    event.preventDefault();
  }
  const button = target.closest<HTMLButtonElement>("button[data-copy]");
  if (button) {
    void copy(button);
  }
  const theme = target.closest<HTMLButtonElement>("button[data-theme-choice]");
  if (theme) {
    pickTheme(theme);
  }
});
