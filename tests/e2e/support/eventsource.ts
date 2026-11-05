/*
 * A plain event-stream reader over fetch, standing in for the browser's
 * EventSource, which Node does not ship without a flag. It reads the
 * same frames a messenger's EventSource would: one `data:` line per
 * centrifuge push, separated by blank lines.
 */

export interface Push {
  connect?: {
    client: string;
    subs?: Record<string, { recovered?: boolean; epoch?: string; offset?: number; publications?: { data: Event }[] }>;
  };
  pub?: { data: Event; offset?: number };
  disconnect?: { code: number; reason: string };
}

export interface Event {
  event: string;
  data: Record<string, unknown>;
}

/*
 * Stream is one open event stream and the pushes it received so far.
 */
export class Stream {
  readonly pushes: Push[] = [];
  private readonly abort = new AbortController();
  private ended = false;

  /*
   * open connects with the connect command in the URL, as the messenger
   * does, and keeps reading in the background until closed.
   */
  static async open(base: string, connect: object, headers: Record<string, string> = {}): Promise<Stream> {
    const stream = new Stream();
    const url = `${base}/connection/uni_sse?cf_connect=${encodeURIComponent(JSON.stringify(connect))}`;
    const response = await fetch(url, { headers, signal: stream.abort.signal });
    if (!response.ok || !response.body) {
      throw new Error(`stream refused: ${response.status}`);
    }

    void stream.read(response.body);

    return stream;
  }

  /*
   * events lists the names of the publications received so far.
   */
  events(): string[] {
    return this.pushes.flatMap((push) => (push.pub ? [push.pub.data.event] : []));
  }

  /*
   * lastOffset is the position of the latest publication, for recovery.
   */
  lastOffset(): number {
    return this.pushes.reduce((offset, push) => push.pub?.offset ?? offset, 0);
  }

  get closed(): boolean {
    return this.ended;
  }

  close() {
    this.abort.abort();
  }

  private async read(body: ReadableStream<Uint8Array>) {
    const decoder = new TextDecoder();
    let buffer = "";

    try {
      for await (const chunk of body) {
        buffer += decoder.decode(chunk, { stream: true });
        const frames = buffer.split("\n\n");
        buffer = frames.pop() ?? "";
        for (const frame of frames) {
          this.take(frame);
        }
      }
    } catch {
      return;
    } finally {
      this.ended = true;
    }
  }

  private take(frame: string) {
    const data = frame
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trim())
      .join("\n");
    if (data === "" || data === "{}") {
      return;
    }

    this.pushes.push(JSON.parse(data) as Push);
  }
}
