/*
 * The public portal's only script. Every page is rendered on the server
 * and works as plain links and forms; Turbo Drive then swaps pages in
 * place, fetching a link as the pointer rests on it and fading between
 * pages, so moving around feels instant and anything marked permanent,
 * such as the messenger, survives the move. The rest are small touches a
 * page reads from its own markup: the greeting follows the visitor's
 * clock, the copy buttons put an article's Markdown on the clipboard, and
 * the theme switch keeps the reader's pick of light, dark or their
 * system's theme in this browser.
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
 * showThemes reveals the switch and marks the reader's pick.
 */
function showThemes(root: ParentNode): void {
  const pick = storedTheme();
  for (const group of root.querySelectorAll<HTMLElement>("[data-theme-switch]")) {
    group.hidden = false;
  }
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

document.addEventListener("turbo:load", () => {
  greet(document);
  signIn(document);
  showThemes(document);
});

document.addEventListener("click", (event) => {
  const target = event.target as Element;
  const button = target.closest<HTMLButtonElement>("button[data-copy]");
  if (button) {
    void copy(button);
  }
  const theme = target.closest<HTMLButtonElement>("button[data-theme-choice]");
  if (theme) {
    pickTheme(theme);
  }
});
