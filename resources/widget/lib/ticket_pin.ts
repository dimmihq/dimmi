/*
 * The pinned request bar above a request's thread and the rows that ease
 * in when the Messages list is narrowed. Both only matter once there are
 * requests, so they ship in the requests chunk, not the core.
 */
/*
 * pinStrip shows a request's progress in a bar of its own between the
 * conversation's header and its thread, so it stays in sight however far
 * the visitor scrolls and is never drawn away with the messages. Once the
 * thread scrolls, the bar folds to one line with a thin progress line.
 * An empty markup takes the bar away.
 */
export function pinStrip(thread: HTMLElement, markup: string) {
  let bar = thread.previousElementSibling as HTMLElement | null;
  if (!bar?.classList.contains("ticket-pin")) {
    if (!markup) {
      return;
    }
    bar = thread.ownerDocument.createElement("div");
    bar.className = "ticket-pin";
    thread.before(bar);
  }
  bar.hidden = !markup;
  if (bar.dataset.markup !== markup) {
    bar.innerHTML = markup;
    bar.dataset.markup = markup;
  }
  if (!thread.dataset.pinWatched) {
    thread.dataset.pinWatched = "true";
    const pinned = bar;
    thread.addEventListener("scroll", () => fold(pinned, thread), { passive: true });
  }
}

/*
 * fold folds the bar once the visitor scrolls well into a thread long
 * enough to stay scrollable after the bar shrinks, and unfolds it only
 * back at the very top. Folding changes the thread's height and with it
 * the scroll position; one threshold for both would let that change flip
 * the bar back and forth and fight the visitor's scrolling.
 */
function fold(bar: HTMLElement, thread: HTMLElement) {
  const folded = bar.hasAttribute("data-folded");
  const room = thread.scrollHeight - thread.clientHeight;
  if (!folded && thread.scrollTop > 56 && room > 140) {
    bar.setAttribute("data-folded", "");
  } else if (folded && thread.scrollTop < 4) {
    bar.removeAttribute("data-folded");
  }
}

/*
 * enterRows lets the rows a list now shows ease in one after another,
 * as when the Messages list is narrowed to open requests or opened up
 * again. Reduced motion shows them as they are.
 */
export function enterRows(rows: Iterable<Element>) {
  const view = (rows as { ownerDocument?: Document }).ownerDocument?.defaultView ?? globalThis.window;
  if (view?.matchMedia?.("(prefers-reduced-motion: reduce)").matches) {
    return;
  }
  let index = 0;
  for (const row of rows) {
    if (typeof row.animate !== "function") {
      return;
    }
    row.animate(
      [
        { opacity: 0, transform: "translateY(6px)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 260, delay: Math.min(index, 8) * 35, easing: "cubic-bezier(0.22, 1, 0.36, 1)", fill: "backwards" },
    );
    index += 1;
  }
}
