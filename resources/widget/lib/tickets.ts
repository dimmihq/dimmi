/*
 * Requests are a chunk of their own, so the core does not grow with
 * them: Home's "Get help with" cards, the request form, the filter of
 * open requests and the progress strip above a request's thread. The
 * messenger asks for the chunk once it has a request type or a request
 * to show, and draws again once it arrived.
 */
export type TicketParts = typeof import("./ticket_parts");

let parts: TicketParts | undefined;
let loading: Promise<TicketParts> | undefined;

/*
 * tickets returns the loaded chunk, or nothing while it is on its way.
 */
export function tickets(): TicketParts | undefined {
  return parts;
}

/*
 * loadTickets fetches the chunk once and adopts its stylesheet into the
 * messenger's shadow root.
 */
export function loadTickets(root: ShadowRoot): Promise<TicketParts> {
  loading ??= import("./ticket_parts").then((loaded) => (parts = loaded));

  return loading.then((loaded) => {
    loaded.adopt(root);
    return loaded;
  });
}

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
    const fold = () => pinned.toggleAttribute("data-folded", thread.scrollTop > 12);
    thread.addEventListener("scroll", fold, { passive: true });
    fold();
  }
}
