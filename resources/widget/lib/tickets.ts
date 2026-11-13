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
