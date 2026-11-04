/*
 * The public portal's only script. Every page is rendered on the server
 * and works as plain links and forms; Turbo Drive then swaps pages in
 * place, fetching a link as the pointer rests on it and fading between
 * pages, so moving around feels instant and anything marked permanent,
 * such as the messenger, survives the move. The rest are small touches a
 * page reads from its own markup: the greeting follows the visitor's
 * clock, and the copy buttons put an article's Markdown on the clipboard.
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

document.addEventListener("turbo:load", () => greet(document));

document.addEventListener("click", (event) => {
  const button = (event.target as Element).closest<HTMLButtonElement>("button[data-copy]");
  if (button) {
    void copy(button);
  }
});
