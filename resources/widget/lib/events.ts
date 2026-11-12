/*
 * Sends the events the page tracked, Dimmi("track", name, metadata), in
 * batches of up to 20 once the visitor has a session. The queue is the
 * connection's own: a batch that never reached the server, or came back
 * over the visitor's limit, goes back to its front for the next try; one
 * the server refused is dropped, saying why in the console, and so is a
 * name the workspace has not opened to the messenger. Each event carries
 * an id, so a batch sent twice counts once.
 */
import { ApiError, type Http } from "../client/http";
import { report } from "./report";

const batchSize = 20;
let sending = false;

interface Sent {
  data: { events: { status: string }[] };
}

export async function send(http: Http, queue: unknown[][]): Promise<void> {
  if (sending) {
    return;
  }
  sending = true;
  try {
    while (queue.length > 0) {
      const batch = queue.splice(0, batchSize);
      try {
        const res = await http.post<Sent>("/events", {
          events: batch.map(([name, metadata, id]) => ({ name: String(name), metadata: plain(metadata), id })),
        });
        res.data.events.forEach((event, i) => {
          if (event.status === "not_allowed") {
            report(
              "warn",
              "event_not_allowed",
              `"${String(batch[i]?.[0])}" is not open to the messenger. Switch it on in Settings > Attributes.`,
            );
          }
        });
      } catch (error) {
        if (!(error instanceof ApiError) || error.status === 0 || error.status === 429) {
          queue.unshift(...batch);
          return;
        }
        report("warn", error.code, `events not recorded: ${error.message}`);
      }
    }
  } finally {
    sending = false;
  }
}

/*
 * plain is the metadata as an object, or none.
 */
function plain(metadata: unknown): Record<string, unknown> {
  return metadata && typeof metadata === "object" && !Array.isArray(metadata)
    ? (metadata as Record<string, unknown>)
    : {};
}
