/*
 * The public portal's only script. Every page is rendered on the server
 * and works as plain links and forms; Turbo Drive then swaps pages in
 * place, fetching a link as the pointer rests on it and fading between
 * pages, so moving around feels instant and anything marked permanent,
 * such as the messenger, survives the move. The rest are small touches a
 * page reads from its own markup: the greeting follows the visitor's
 * clock, and the copy buttons put an article's Markdown on the clipboard.
 * A reader coming back from signing in to the team's product brings an
 * identity token in the address's fragment, which no server sees; the
 * sign-in page's form takes it from there and sends it at once.
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
 * signIn sends the token the fragment carries, then forgets it.
 */
function signIn(root: ParentNode): void {
  const form = root.querySelector<HTMLFormElement>("form[data-help-sign-in]");
  const token = new URLSearchParams(window.location.hash.slice(1)).get("token");
  if (!form || !token) {
    return;
  }
  const field = form.querySelector<HTMLInputElement>('input[name="token"]');
  if (field) {
    field.value = token;
    history.replaceState(null, "", window.location.pathname + window.location.search);
    form.requestSubmit();
  }
}

document.addEventListener("turbo:load", () => {
  greet(document);
  signIn(document);
});

document.addEventListener("click", (event) => {
  const button = (event.target as Element).closest<HTMLButtonElement>("button[data-copy]");
  if (button) {
    void copy(button);
  }
});
